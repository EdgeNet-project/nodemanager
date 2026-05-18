package identity

import (
	"os"
	"testing"
)

func TestLoadOrCreate(t *testing.T) {
	tmpFile := "test_identity.json"
	defer os.Remove(tmpFile)

	// Test Create
	id1, err := LoadOrCreate(tmpFile)
	if err != nil {
		t.Fatalf("Failed to create identity: %v", err)
	}

	if id1.Code == "" {
		t.Error("NodeCode should not be empty")
	}

	// Verify permissions (on Unix-like systems)
	info, err := os.Stat(tmpFile)
	if err != nil {
		t.Fatalf("Failed to stat identity file: %v", err)
	}
	// 0600 = -rw-------
	if info.Mode().Perm() != 0600 {
		t.Errorf("Expected permissions 0600, got %o", info.Mode().Perm())
	}

	// Test Load
	id2, err := LoadOrCreate(tmpFile)
	if err != nil {
		t.Fatalf("Failed to load identity: %v", err)
	}

	if id1.Code != id2.Code {
		t.Errorf("NodeCode mismatch: %s != %s", id1.Code, id2.Code)
	}
}
