package identity

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Identity represents the node's stable identity
type Identity struct {
	Code string `json:"code"`
}

// LoadOrCreate loads the identity from disk or generates a new one if it doesn't exist
func LoadOrCreate(path string) (*Identity, error) {
	if _, err := os.Stat(path); err == nil {
		return load(path)
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to check identity file: %w", err)
	}

	// Not exists, create new
	id, err := generate()
	if err != nil {
		return nil, fmt.Errorf("failed to generate identity: %w", err)
	}

	if err := save(path, id); err != nil {
		return nil, fmt.Errorf("failed to save identity: %w", err)
	}

	return id, nil
}

func load(path string) (*Identity, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var id Identity
	if err := json.NewDecoder(f).Decode(&id); err != nil {
		return nil, err
	}

	return &id, nil
}

func save(path string, id *Identity) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(id, "", "  ")
	if err != nil {
		return err
	}

	// Write with 0600 permissions
	return os.WriteFile(path, data, 0600)
}

func generate() (*Identity, error) {
	// Generate random node code (6 characters, uppercase hex-like or simple random)
	// Using 3 bytes -> 6 hex chars
	codeBytes := make([]byte, 3)
	if _, err := io.ReadFull(rand.Reader, codeBytes); err != nil {
		return nil, err
	}
	nodeCode := hex.EncodeToString(codeBytes)

	return &Identity{
		Code: nodeCode,
	}, nil
}
