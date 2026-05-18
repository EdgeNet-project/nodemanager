package system

import (
	"os"
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
