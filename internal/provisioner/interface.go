package provisioner

import (
	"context"

	"github.com/edgenet-project/edgenet-agent/pkg/models"
)

// Provisioner defines the interface for cluster/service setup backends
type Provisioner interface {
	Name() string
	Provision(ctx context.Context, node models.Node) error
	Deprovision(ctx context.Context) error
	IsProvisioned(ctx context.Context) (bool, error)
}
