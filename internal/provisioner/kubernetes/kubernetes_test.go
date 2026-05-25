package kubernetes

import (
	"testing"

	"github.com/EdgeNet-project/nodemanager/internal/config"
	"go.uber.org/zap"
)

func TestKubernetesProvisioner_Name(t *testing.T) {
	logger := zap.NewNop()
	cfg := &config.Config{}
	p := New(logger, cfg)

	if p.Name() != "kubernetes" {
		t.Errorf("Expected kubernetes, got %s", p.Name())
	}
}
