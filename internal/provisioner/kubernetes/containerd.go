package kubernetes

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"go.uber.org/zap"
)

func (p *KubernetesProvisioner) installContainerd(ctx context.Context) error {
	p.logger.Info("Installing and configuring containerd")

	if _, err := exec.LookPath("containerd"); err != nil {
		// Assuming we are on a system where we can use dnf/apt
		// For now let's just try to install it.
		// In a real scenario, we'd check the OS and use the appropriate package manager.
		p.logger.Info("containerd not found, attempting to install")
		if _, err := exec.LookPath("dnf"); err == nil {
			_ = exec.CommandContext(ctx, "dnf", "install", "-y", "containerd").Run()
		} else if _, err := exec.LookPath("apt-get"); err == nil {
			_ = exec.CommandContext(ctx, "apt-get", "update").Run()
			_ = exec.CommandContext(ctx, "apt-get", "install", "-y", "containerd.io").Run()
		}
	}

	// Configure containerd
	_ = os.MkdirAll("/etc/containerd", 0755)
	cmd := exec.CommandContext(ctx, "containerd", "config", "default")
	out, err := cmd.Output()
	if err == nil {
		// Enable SystemdCgroup
		config := strings.ReplaceAll(string(out), "SystemdCgroup = false", "SystemdCgroup = true")
		_ = os.WriteFile("/etc/containerd/config.toml", []byte(config), 0644)
	}

	_ = exec.CommandContext(ctx, "systemctl", "enable", "containerd").Run()
	if err := exec.CommandContext(ctx, "systemctl", "restart", "containerd").Run(); err != nil {
		return fmt.Errorf("failed to restart containerd: %w", err)
	}

	// Configure crictl
	crictlConfig := `runtime-endpoint: unix:///run/containerd/containerd.sock
image-endpoint: unix:///run/containerd/containerd.sock
timeout: 10
debug: false
`
	_ = os.WriteFile("/etc/crictl.yaml", []byte(crictlConfig), 0644)

	// Validate
	if err := exec.CommandContext(ctx, "crictl", "info").Run(); err != nil {
		p.logger.Warn("crictl info failed, but continuing", zap.Error(err))
	}

	return nil
}
