package metrics

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type collector struct {
	mu sync.Mutex

	lastTimestamp time.Time

	lastCPUStats *cpuStats

	lastDiskStats *diskStats

	lastNetStats map[string]netStats
}

type cpuStats struct {
	user    uint64
	nice    uint64
	system  uint64
	idle    uint64
	iowait  uint64
	irq     uint64
	softirq uint64
	steal   uint64
	guest   uint64
	guestNice uint64
}

func (s *cpuStats) Total() uint64 {
	return s.user + s.nice + s.system + s.idle + s.iowait + s.irq + s.softirq + s.steal
}

func (s *cpuStats) Idle() uint64 {
	return s.idle + s.iowait
}

type diskStats struct {
	readSectors  uint64
	writeSectors uint64
}

type netStats struct {
	rxBytes uint64
	txBytes uint64
}

func NewCollector() Collector {
	return &collector{
		lastNetStats: make(map[string]netStats),
	}
}

func (c *collector) Collect() (*Metrics, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(c.lastTimestamp).Seconds()

	metrics := &Metrics{
		Timestamp: now,
	}

	// CPU and Load
	c.collectCPU(metrics, elapsed)
	c.collectLoad(metrics)

	// Disk IO
	c.collectDisk(metrics, elapsed)

	// Network
	c.collectNetwork(metrics, elapsed)

	c.lastTimestamp = now
	return metrics, nil
}

func (c *collector) collectCPU(m *Metrics, elapsed float64) {
	stats, err := readCPUStats()
	if err != nil {
		return
	}

	if c.lastCPUStats != nil && elapsed > 0 {
		totalDelta := stats.Total() - c.lastCPUStats.Total()
		idleDelta := stats.Idle() - c.lastCPUStats.Idle()

		if totalDelta > 0 {
			m.CPU.UsagePercent = 100.0 * float64(totalDelta-idleDelta) / float64(totalDelta)
		}
	}
	c.lastCPUStats = stats
}

func (c *collector) collectLoad(m *Metrics) {
	load1, load5, load15, err := readLoadAvg()
	if err == nil {
		m.CPU.Load1 = load1
		m.CPU.Load5 = load5
		m.CPU.Load15 = load15
	}
}

func (c *collector) collectDisk(m *Metrics, elapsed float64) {
	stats, err := readDiskStats()
	if err != nil {
		return
	}

	if c.lastDiskStats != nil && elapsed > 0 {
		readDelta := stats.readSectors - c.lastDiskStats.readSectors
		writeDelta := stats.writeSectors - c.lastDiskStats.writeSectors

		// Standard sector size is 512 bytes
		m.IO.ReadBytesPerSec = float64(readDelta) * 512.0 / elapsed
		m.IO.WriteBytesPerSec = float64(writeDelta) * 512.0 / elapsed
	}
	c.lastDiskStats = stats
}

func (c *collector) collectNetwork(m *Metrics, elapsed float64) {
	interfaces, err := os.ReadDir("/sys/class/net")
	if err != nil {
		return
	}

	currentNetStats := make(map[string]netStats)
	for _, iface := range interfaces {
		name := iface.Name()
		if name == "lo" {
			continue
		}

		// Check if interface is up
		operstate, err := os.ReadFile(filepath.Join("/sys/class/net", name, "operstate"))
		if err != nil || strings.TrimSpace(string(operstate)) != "up" {
			continue
		}

		rx, tx, err := readInterfaceStats(name)
		if err != nil {
			continue
		}

		stats := netStats{rxBytes: rx, txBytes: tx}
		currentNetStats[name] = stats

		ifaceMetrics := InterfaceMetrics{
			Name:    name,
			RxBytes: rx,
			TxBytes: tx,
		}

		if last, ok := c.lastNetStats[name]; ok && elapsed > 0 {
			if rx >= last.rxBytes {
				ifaceMetrics.RxBytesPerSec = float64(rx-last.rxBytes) / elapsed
			}
			if tx >= last.txBytes {
				ifaceMetrics.TxBytesPerSec = float64(tx-last.txBytes) / elapsed
			}
		}

		m.Network.Interfaces = append(m.Network.Interfaces, ifaceMetrics)
	}
	c.lastNetStats = currentNetStats
}

func readCPUStats() (*cpuStats, error) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	if scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "cpu ") {
			fields := strings.Fields(line)
			if len(fields) < 8 {
				return nil, fmt.Errorf("unexpected cpu line format")
			}
			s := &cpuStats{}
			s.user, _ = strconv.ParseUint(fields[1], 10, 64)
			s.nice, _ = strconv.ParseUint(fields[2], 10, 64)
			s.system, _ = strconv.ParseUint(fields[3], 10, 64)
			s.idle, _ = strconv.ParseUint(fields[4], 10, 64)
			s.iowait, _ = strconv.ParseUint(fields[5], 10, 64)
			s.irq, _ = strconv.ParseUint(fields[6], 10, 64)
			s.softirq, _ = strconv.ParseUint(fields[7], 10, 64)
			s.steal, _ = strconv.ParseUint(fields[8], 10, 64)
			return s, nil
		}
	}
	return nil, fmt.Errorf("cpu line not found")
}

func readLoadAvg() (float64, float64, float64, error) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, 0, 0, err
	}
	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return 0, 0, 0, fmt.Errorf("unexpected loadavg format")
	}
	l1, _ := strconv.ParseFloat(fields[0], 64)
	l5, _ := strconv.ParseFloat(fields[1], 64)
	l15, _ := strconv.ParseFloat(fields[2], 64)
	return l1, l5, l15, nil
}

func readDiskStats() (*diskStats, error) {
	f, err := os.Open("/proc/diskstats")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	stats := &diskStats{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 14 {
			continue
		}
		name := fields[2]
		// Ignore common virtual devices
		if strings.HasPrefix(name, "loop") || strings.HasPrefix(name, "ram") || strings.HasPrefix(name, "sr") {
			continue
		}

		// We want to sum up stats for all physical disks.
		// Usually these are sda, sdb, nvme0n1, etc.
		// A simple heuristic is to check if it's a whole device, but it's hard without more info.
		// However, the requirement says "sum sectors read" for "physical disks".
		// For simplicity, and to avoid double counting partitions and whole devices, 
		// we could try to identify whole devices. On Linux, whole devices usually don't have a trailing digit if they start with sd, 
		// but nvme is nvme0n1.
		
		// Alternative: check /sys/block/<name>/partition. If it doesn't exist, it's a whole device.
		if isWholeDisk(name) {
			rs, _ := strconv.ParseUint(fields[5], 10, 64)
			ws, _ := strconv.ParseUint(fields[9], 10, 64)
			stats.readSectors += rs
			stats.writeSectors += ws
		}
	}
	return stats, nil
}

func isWholeDisk(name string) bool {
	// Check if /sys/block/<name>/partition exists. 
	// If it does, it's a partition. If not, it's a whole disk.
	_, err := os.Stat(fmt.Sprintf("/sys/block/%s/partition", name))
	if os.IsNotExist(err) {
		// Also check if it's actually in /sys/block
		_, err := os.Stat(fmt.Sprintf("/sys/block/%s", name))
		return err == nil
	}
	return false
}

func readInterfaceStats(name string) (uint64, uint64, error) {
	rxData, err := os.ReadFile(filepath.Join("/sys/class/net", name, "statistics/rx_bytes"))
	if err != nil {
		return 0, 0, err
	}
	txData, err := os.ReadFile(filepath.Join("/sys/class/net", name, "statistics/tx_bytes"))
	if err != nil {
		return 0, 0, err
	}

	rx, _ := strconv.ParseUint(strings.TrimSpace(string(rxData)), 10, 64)
	tx, _ := strconv.ParseUint(strings.TrimSpace(string(txData)), 10, 64)

	return rx, tx, nil
}
