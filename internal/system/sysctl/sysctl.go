package sysctl

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
)

// Get returns the value of a sysctl parameter.
func Get(key string) (string, error) {
	out, err := exec.Command("sysctl", "-n", key).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// Set sets a sysctl parameter.
func Set(ctx context.Context, key, value string) error {
	current, err := Get(key)
	if err == nil && current == value {
		return nil
	}
	return exec.CommandContext(ctx, "sysctl", "-w", fmt.Sprintf("%s=%s", key, value)).Run()
}

// Persist writes sysctl settings to a file.
func Persist(path string, settings map[string]string) error {
	keys := make([]string, 0, len(settings))
	for k := range settings {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	for _, k := range keys {
		sb.WriteString(fmt.Sprintf("%s=%s\n", k, settings[k]))
	}
	newContent := sb.String()

	data, err := os.ReadFile(path)
	if err == nil && string(data) == newContent {
		return nil
	}

	return os.WriteFile(path, []byte(newContent), 0644)
}
