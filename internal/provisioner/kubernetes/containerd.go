package kubernetes

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/EdgeNet-project/nodemanager/internal/system"
	"go.uber.org/zap"
)

func (p *KubernetesProvisioner) installContainerd(ctx context.Context) error {
	p.logger.Info("Installing and configuring containerd")

	if _, err := exec.LookPath("containerd"); err != nil {
		p.logger.Info("containerd not found, attempting to install")
		if _, err := exec.LookPath("dnf"); err == nil {
			if err := p.installContainerdDnf(ctx); err != nil {
				return err
			}
		} else if _, err := exec.LookPath("apt-get"); err == nil {
			if err := p.installContainerdApt(ctx); err != nil {
				return err
			}
		} else {
			return fmt.Errorf("no supported package manager found (dnf or apt-get)")
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

func (p *KubernetesProvisioner) installContainerdDnf(ctx context.Context) error {
	p.logger.Info("Installing containerd using dnf")
	_ = exec.CommandContext(ctx, "dnf", "install", "-y", "dnf-plugins-core").Run()
	_ = exec.CommandContext(ctx, "dnf", "config-manager", "--add-repo", "https://download.docker.com/linux/centos/docker-ce.repo").Run()
	if err := exec.CommandContext(ctx, "dnf", "install", "-y", "containerd.io").Run(); err != nil {
		return fmt.Errorf("failed to install containerd.io: %w", err)
	}
	return nil
}

func (p *KubernetesProvisioner) installContainerdApt(ctx context.Context) error {
	p.logger.Info("Installing containerd using apt")
	_ = exec.CommandContext(ctx, "apt-get", "update").Run()
	_ = exec.CommandContext(ctx, "apt-get", "install", "-y", "ca-certificates", "curl", "gnupg").Run()

	_ = os.MkdirAll("/etc/apt/keyrings", 0755)

	gpgKeyPath := "/etc/apt/keyrings/docker.gpg"
	_ = os.Remove(gpgKeyPath) // Remove if exists to avoid gpg prompt

	gpgCmd := "curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o " + gpgKeyPath
	_ = exec.CommandContext(ctx, "bash", "-c", gpgCmd).Run()
	_ = os.Chmod(gpgKeyPath, 0644)

	archOut, _ := exec.CommandContext(ctx, "dpkg", "--print-architecture").Output()
	arch := strings.TrimSpace(string(archOut))
	codename := system.GetOSReleaseValue("VERSION_CODENAME")

	repoLine := fmt.Sprintf("deb [arch=%s signed-by=%s] https://download.docker.com/linux/ubuntu %s stable\n", arch, gpgKeyPath, codename)
	_ = os.WriteFile("/etc/apt/sources.list.d/docker.list", []byte(repoLine), 0644)

	_ = exec.CommandContext(ctx, "apt-get", "update").Run()
	if err := exec.CommandContext(ctx, "apt-get", "install", "-y", "containerd.io").Run(); err != nil {
		return fmt.Errorf("failed to install containerd.io: %w", err)
	}
	return nil
}
