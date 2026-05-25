package kubernetes

import (
	"context"
	"fmt"
	"sort"

	"github.com/EdgeNet-project/nodemanager/internal/system/modules"
	"github.com/EdgeNet-project/nodemanager/internal/system/swap"
	"github.com/EdgeNet-project/nodemanager/internal/system/sysctl"
	"go.uber.org/zap"
)

func (p *KubernetesProvisioner) prepareOS(ctx context.Context) error {
	p.logger.Info("Preparing OS for Kubernetes")

	// Load and persist kernel modules
	kernelModules := []string{"overlay", "br_netfilter"}
	for _, mod := range kernelModules {
		p.logger.Info("Ensuring kernel module", zap.String("module", mod))
		if err := modules.Load(ctx, mod); err != nil {
			return fmt.Errorf("failed to load kernel module %s: %w", mod, err)
		}
		if err := modules.Persist(mod); err != nil {
			p.logger.Warn("Failed to persist kernel module", zap.String("module", mod), zap.Error(err))
		}
	}

	// Sysctl
	sysctls := map[string]string{
		"net.bridge.bridge-nf-call-iptables":  "1",
		"net.ipv4.ip_forward":                 "1",
		"net.bridge.bridge-nf-call-ip6tables": "1",

		"vm.overcommit_memory": "1",
		"kernel.panic":         "10",
		"kernel.panic_on_oops": "1",
	}

	keys := make([]string, 0, len(sysctls))
	for k := range sysctls {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		v := sysctls[k]
		p.logger.Info("Ensuring sysctl", zap.String("key", k), zap.String("value", v))
		if err := sysctl.Set(ctx, k, v); err != nil {
			return fmt.Errorf("failed to set sysctl %s: %w", k, err)
		}
	}

	// Persist sysctl
	sysctlPath := "/etc/sysctl.d/99-kubernetes-cri.conf"
	if err := sysctl.Persist(sysctlPath, sysctls); err != nil {
		return fmt.Errorf("failed to write sysctl config: %w", err)
	}

	// Disable swap
	if swap.IsEnabled() {
		p.logger.Info("Disabling swap")
		if err := swap.Disable(ctx); err != nil {
			p.logger.Warn("Failed to disable swap", zap.Error(err))
		}
	}

	// Remove from fstab
	if err := swap.RemoveFromFstab(); err != nil {
		p.logger.Warn("Failed to remove swap from /etc/fstab", zap.Error(err))
	} else {
		p.logger.Info("Swap entries removed from /etc/fstab (if any)")
	}

	return nil
}
