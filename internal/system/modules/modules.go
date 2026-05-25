package modules

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// IsLoaded checks if a kernel module is loaded.
func IsLoaded(name string) bool {
	data, err := os.ReadFile("/proc/modules")
	if err != nil {
		return false
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, name+" ") {
			return true
		}
	}
	return false
}

// Load loads a kernel module using modprobe.
func Load(ctx context.Context, name string) error {
	if IsLoaded(name) {
		return nil
	}
	return exec.CommandContext(ctx, "modprobe", name).Run()
}

// Persist ensures a kernel module is loaded at boot time by adding it to /etc/modules-load.d/.
func Persist(name string) error {
	modPath := fmt.Sprintf("/etc/modules-load.d/%s.conf", name)
	if data, err := os.ReadFile(modPath); err != nil || string(data) != name {
		return os.WriteFile(modPath, []byte(name), 0644)
	}
	return nil
}
