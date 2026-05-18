package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "agent.conf")

	// 1. Test missing server field
	configContent := `[agent]
identity = /etc/edgenet/identity.json
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Error("expected error when server is missing, got nil")
	}

	// 2. Test successful load
	configContent = `[agent]
server = https://n.planetlab.io
identity = /etc/edgenet/identity.json
state = /var/lib/edgenet/state.json
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Server != "https://n.planetlab.io" {
		t.Errorf("expected server https://n.planetlab.io, got %s", cfg.Server)
	}

	if cfg.Identity != "/etc/edgenet/identity.json" {
		t.Errorf("expected identity /etc/edgenet/identity.json, got %s", cfg.Identity)
	}

	if cfg.State != "/var/lib/edgenet/state.json" {
		t.Errorf("expected state /var/lib/edgenet/state.json, got %s", cfg.State)
	}
}
