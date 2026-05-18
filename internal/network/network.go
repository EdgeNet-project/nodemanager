package network

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

var publicIPServices = []string{
	"https://api.ipify.org",
	"https://ifconfig.me/ip",
	"https://ident.me",
}

// GetPublicIP returns the public IP address of the node
func GetPublicIP() (string, error) {
	client := http.Client{
		Timeout: 5 * time.Second,
	}

	var lastErr error
	for _, service := range publicIPServices {
		resp, err := client.Get(service)
		if err != nil {
			lastErr = err
			continue
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			lastErr = err
			continue
		}

		ip := strings.TrimSpace(string(body))
		if net.ParseIP(ip) != nil {
			return ip, nil
		}
	}

	return "", fmt.Errorf("failed to get public IP: %v", lastErr)
}

// GetLocalIPs returns a list of local IP addresses
func GetLocalIPs() ([]string, error) {
	var ips []string
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}

	for _, address := range addrs {
		// check the address type and if it is not a loopback the display it
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				ips = append(ips, ipnet.IP.String())
			}
		}
	}
	return ips, nil
}

// IsAnyInterfaceUp checks if any non-loopback network interface is up
func IsAnyInterfaceUp() bool {
	ifaces, err := net.Interfaces()
	if err != nil {
		return false
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp != 0 && iface.Flags&net.FlagLoopback == 0 {
			return true
		}
	}
	return false
}

// GetDefaultGateway returns the default gateway IP address on Linux
func GetDefaultGateway() (string, error) {
	file, err := os.Open("/proc/net/route")
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		// Destination 00000000 is the default route
		if fields[1] == "00000000" {
			gwHex := fields[2]
			// Gateway is in little-endian hex
			d, err := hex.DecodeString(gwHex)
			if err != nil || len(d) != 4 {
				continue
			}
			return net.IPv4(d[3], d[2], d[1], d[0]).String(), nil
		}
	}
	return "", fmt.Errorf("default gateway not found")
}

// Ping checks if a host is reachable via ping
func Ping(host string) bool {
	// -c 1: send 1 packet
	// -W 1: wait 1 second for response
	err := exec.Command("ping", "-c", "1", "-W", "1", host).Run()
	return err == nil
}
