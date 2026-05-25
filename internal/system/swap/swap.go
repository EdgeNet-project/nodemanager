package swap

import (
	"context"
	"os"
	"os/exec"
	"strings"
)

// IsEnabled checks if swap is enabled on the system.
func IsEnabled() bool {
	data, err := os.ReadFile("/proc/swaps")
	if err != nil {
		return false
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	return len(lines) > 1
}

// Disable disables all swap devices.
func Disable(ctx context.Context) error {
	if !IsEnabled() {
		return nil
	}
	return exec.CommandContext(ctx, "swapoff", "-a").Run()
}

// RemoveFromFstab removes swap entries from /etc/fstab.
func RemoveFromFstab() error {
	fstabPath := "/etc/fstab"
	fstab, err := os.ReadFile(fstabPath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(fstab), "\n")
	var newLines []string
	changed := false
	for _, line := range lines {
		if strings.Contains(line, "swap") {
			changed = true
			continue
		}
		newLines = append(newLines, line)
	}

	if !changed {
		return nil
	}

	return os.WriteFile(fstabPath, []byte(strings.Join(newLines, "\n")), 0644)
}
