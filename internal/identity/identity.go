package identity

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/edgenet-project/edgenet-agent/pkg/models"
)

// LoadOrCreate loads the identity from disk or generates a new one if it doesn't exist
func LoadOrCreate(path string) (*models.Node, error) {
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

func load(path string) (*models.Node, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var id models.Node
	if err := json.NewDecoder(f).Decode(&id); err != nil {
		return nil, err
	}

	return &id, nil
}

func save(path string, id *models.Node) error {
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

func generate() (*models.Node, error) {
	// Generate random node code (6 characters, uppercase hex-like or simple random)
	// Using 3 bytes -> 6 hex chars
	codeBytes := make([]byte, 3)
	if _, err := io.ReadFull(rand.Reader, codeBytes); err != nil {
		return nil, err
	}
	nodeCode := hex.EncodeToString(codeBytes)

	return &models.Node{
		Code: nodeCode,
	}, nil
}
