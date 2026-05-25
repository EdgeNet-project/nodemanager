package system

import (
	"os"
	"os/exec"
	"runtime"
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

// GetArch returns the system architecture.
func GetArch() string {
	return runtime.GOARCH
}

// GetKernelVersion returns the kernel version.
func GetKernelVersion() string {
	data, err := os.ReadFile("/proc/sys/kernel/osrelease")
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(data))
}

// GetDistroInfo returns the distribution ID and version.
func GetDistroInfo() (string, string) {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return "unknown", "unknown"
	}

	lines := strings.Split(string(data), "\n")
	var id, version string
	for _, line := range lines {
		if strings.HasPrefix(line, "ID=") {
			id = strings.Trim(strings.TrimPrefix(line, "ID="), "\"")
		} else if strings.HasPrefix(line, "VERSION_ID=") {
			version = strings.Trim(strings.TrimPrefix(line, "VERSION_ID="), "\"")
		}
	}

	if id == "" {
		id = "unknown"
	}
	if version == "" {
		version = "unknown"
	}

	return id, version
}
