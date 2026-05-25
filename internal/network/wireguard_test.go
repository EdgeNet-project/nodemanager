package network

import (
	"path/filepath"
	"testing"

	"go.uber.org/zap"
)

func TestLoadOrGenerateKeys(t *testing.T) {
	tempDir := t.TempDir()
	oldPath := WireguardConfigPath
	WireguardConfigPath = filepath.Join(tempDir, "wireguard.json")
	defer func() { WireguardConfigPath = oldPath }()

	logger := zap.NewNop()

	// 1. Test generate new keys
	wg1, err := LoadOrGenerateKeys(logger)
	if err != nil {
		t.Fatalf("Failed to generate keys: %v", err)
	}
	if wg1.PrivateKey == "" || wg1.PublicKey == "" {
		t.Error("Generated keys should not be empty")
	}

	// 2. Save and load
	if err := SaveWireguardConfig(wg1); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	wg2, err := LoadOrGenerateKeys(logger)
	if err != nil {
		t.Fatalf("Failed to load keys: %v", err)
	}

	if wg1.PrivateKey != wg2.PrivateKey || wg1.PublicKey != wg2.PublicKey {
		t.Errorf("Keys mismatch: %s != %s", wg1.PrivateKey, wg2.PrivateKey)
	}
}
