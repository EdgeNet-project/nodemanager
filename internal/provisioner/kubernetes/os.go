package kubernetes

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"go.uber.org/zap"
)

func (p *KubernetesProvisioner) prepareOS(ctx context.Context) error {
	p.logger.Info("Preparing OS for Kubernetes")

	// Load kernel modules
	modules := []string{"overlay", "br_netfilter"}
	for _, mod := range modules {
		if err := exec.CommandContext(ctx, "modprobe", mod).Run(); err != nil {
			return fmt.Errorf("failed to load kernel module %s: %w", mod, err)
		}
		// Persist modules
		_ = os.WriteFile(fmt.Sprintf("/etc/modules-load.d/%s.conf", mod), []byte(mod), 0644)
	}

	// Sysctl
	sysctls := map[string]string{
		"net.bridge.bridge-nf-call-iptables":  "1",
		"net.ipv4.ip_forward":                 "1",
		"net.bridge.bridge-nf-call-ip6tables": "1",
	}
	for k, v := range sysctls {
		if err := exec.CommandContext(ctx, "sysctl", "-w", fmt.Sprintf("%s=%s", k, v)).Run(); err != nil {
			return fmt.Errorf("failed to set sysctl %s: %w", k, err)
		}
	}
	// Persist sysctl
	var sysctlConfig strings.Builder
	for k, v := range sysctls {
		sysctlConfig.WriteString(fmt.Sprintf("%s=%s\n", k, v))
	}
	if err := os.WriteFile("/etc/sysctl.d/99-kubernetes-cri.conf", []byte(sysctlConfig.String()), 0644); err != nil {
		return fmt.Errorf("failed to write sysctl config: %w", err)
	}

	// Disable swap
	if err := exec.CommandContext(ctx, "swapoff", "-a").Run(); err != nil {
		p.logger.Warn("Failed to disable swap", zap.Error(err))
	}
	// Remove from fstab
	fstab, err := os.ReadFile("/etc/fstab")
	if err == nil {
		lines := strings.Split(string(fstab), "\n")
		var newLines []string
		for _, line := range lines {
			if !strings.Contains(line, "swap") {
				newLines = append(newLines, line)
			}
		}
		_ = os.WriteFile("/etc/fstab", []byte(strings.Join(newLines, "\n")), 0644)
	}

	return nil
}
