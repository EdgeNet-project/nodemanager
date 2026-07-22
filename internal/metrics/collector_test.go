package metrics

import (
	"os"
	"testing"
)

func TestReadCPUStats(t *testing.T) {
	// This test might fail on non-Linux systems if /proc/stat is missing.
	// In a real scenario, we would mock the file system.
	stats, err := readCPUStats()
	if err != nil {
		if os.IsNotExist(err) {
			t.Skip("Skipping test: /proc/stat not found (non-Linux?)")
		}
		t.Fatalf("Failed to read CPU stats: %v", err)
	}
	if stats.Total() == 0 {
		t.Errorf("Total CPU time is 0")
	}
}

func TestReadLoadAvg(t *testing.T) {
	l1, l5, l15, err := readLoadAvg()
	if err != nil {
		if os.IsNotExist(err) {
			t.Skip("Skipping test: /proc/loadavg not found")
		}
		t.Fatalf("Failed to read loadavg: %v", err)
	}
	if l1 < 0 || l5 < 0 || l15 < 0 {
		t.Errorf("Negative load average: %f %f %f", l1, l5, l15)
	}
}

func TestReadDiskStats(t *testing.T) {
	stats, err := readDiskStats()
	if err != nil {
		if os.IsNotExist(err) {
			t.Skip("Skipping test: /proc/diskstats not found")
		}
		t.Fatalf("Failed to read disk stats: %v", err)
	}
	// It's possible for stats to be 0 if no physical disks are found or no activity
	t.Logf("Read sectors: %d, Write sectors: %d", stats.readSectors, stats.writeSectors)
}

func TestCollector(t *testing.T) {
	c := NewCollector()
	m, err := c.Collect()
	if err != nil {
		t.Fatalf("First collect failed: %v", err)
	}
	if m == nil {
		t.Fatal("Metrics are nil")
	}

	// Rates should be 0 on first collection
	if m.CPU.UsagePercent != 0 {
		t.Errorf("Expected 0 CPU usage on first collection, got %f", m.CPU.UsagePercent)
	}
}
