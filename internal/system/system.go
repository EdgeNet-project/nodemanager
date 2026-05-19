package system

import (
	"os"
	"os/exec"
	"strings"
)

// GetSystemUUID returns the system UUID from /sys/class/dmi/id/product_uuid on Linux.
func GetSystemUUID() (string, error) {
	data, err := os.ReadFile("/sys/class/dmi/id/product_uuid")
	if err != nil {
		// Fallback for development/other systems if needed, or just return error
		return "unknown-uuid", nil
	}
	return strings.TrimSpace(string(data)), nil
}

// SetHostname changes the machine's hostname.
// It uses hostnamectl on systemd systems, falling back to the hostname command.
func SetHostname(name string) error {
	if name == "" {
		return nil
	}

	current, err := os.Hostname()
	if err == nil && current == name {
		return nil
	}

	// Try hostnamectl first (systemd)
	if _, err := exec.LookPath("hostnamectl"); err == nil {
		if err := exec.Command("hostnamectl", "set-hostname", name).Run(); err == nil {
			return nil
		}
	}

	// Fallback to hostname command
	return exec.Command("hostname", name).Run()
}
