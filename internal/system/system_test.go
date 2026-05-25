package system

import (
	"testing"
)

func TestGetOSReleaseValue(t *testing.T) {
	// Create a mock /etc/os-release if we wanted to be thorough,
	// but for now just test current system.
	val := GetOSReleaseValue("ID")
	if val == "" {
		t.Error("Expected some value for ID")
	}
}
