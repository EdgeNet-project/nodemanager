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
	configPath := "/etc/containerd/config.toml"
	needsRestart := false

	existingConfig, err := os.ReadFile(configPath)
	if err != nil || !strings.Contains(string(existingConfig), "SystemdCgroup = true") {
		p.logger.Info("Configuring containerd (SystemdCgroup = true)")
		cmd := exec.CommandContext(ctx, "containerd", "config", "default")
		out, err := cmd.Output()
		if err == nil {
			// Enable SystemdCgroup
			config := strings.ReplaceAll(string(out), "SystemdCgroup = false", "SystemdCgroup = true")
			if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
				p.logger.Warn("Failed to write containerd config", zap.Error(err))
			} else {
				needsRestart = true
			}
		}
	}

	_ = exec.CommandContext(ctx, "systemctl", "enable", "containerd").Run()

	// Check if service is active
	if !needsRestart {
		if err := exec.CommandContext(ctx, "systemctl", "is-active", "containerd").Run(); err != nil {
			needsRestart = true
		}
	}

	if needsRestart {
		p.logger.Info("Restarting containerd")
		if err := exec.CommandContext(ctx, "systemctl", "restart", "containerd").Run(); err != nil {
			return fmt.Errorf("failed to restart containerd: %w", err)
		}
	} else {
		p.logger.Info("containerd service is already running with correct configuration")
	}

	// Configure crictl
	crictlConfig := `runtime-endpoint: unix:///run/containerd/containerd.sock
image-endpoint: unix:///run/containerd/containerd.sock
timeout: 10
debug: false
`
	existingCrictl, err := os.ReadFile("/etc/crictl.yaml")
	if err != nil || string(existingCrictl) != crictlConfig {
		p.logger.Info("Writing crictl configuration")
		_ = os.WriteFile("/etc/crictl.yaml", []byte(crictlConfig), 0644)
	}

	// Validate
	if err := exec.CommandContext(ctx, "crictl", "info").Run(); err != nil {
		p.logger.Warn("crictl info failed, but continuing", zap.Error(err))
	}

	return nil
}

func (p *KubernetesProvisioner) installContainerdDnf(ctx context.Context) error {
	p.logger.Info("Installing containerd using dnf")

	repoPath := "/etc/yum.repos.d/docker-ce.repo"
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		p.logger.Info("Adding Docker repository")
		_ = exec.CommandContext(ctx, "dnf", "install", "-y", "dnf-plugins-core").Run()
		if err := exec.CommandContext(ctx, "dnf", "config-manager", "--add-repo", "https://download.docker.com/linux/centos/docker-ce.repo").Run(); err != nil {
			return fmt.Errorf("failed to add docker repository: %w", err)
		}
	}

	if err := exec.CommandContext(ctx, "dnf", "install", "-y", "containerd.io").Run(); err != nil {
		return fmt.Errorf("failed to install containerd.io: %w", err)
	}
	return nil
}

func (p *KubernetesProvisioner) installContainerdApt(ctx context.Context) error {
	p.logger.Info("Installing containerd using apt")

	gpgKeyPath := "/etc/apt/keyrings/docker.gpg"
	repoPath := "/etc/apt/sources.list.d/docker.list"

	archOut, _ := exec.CommandContext(ctx, "dpkg", "--print-architecture").Output()
	arch := strings.TrimSpace(string(archOut))
	codename := system.GetOSReleaseValue("VERSION_CODENAME")
	repoLine := fmt.Sprintf("deb [arch=%s signed-by=%s] https://download.docker.com/linux/ubuntu %s stable\n", arch, gpgKeyPath, codename)

	repoCorrect := false
	if data, err := os.ReadFile(repoPath); err == nil && string(data) == repoLine {
		if _, err := os.Stat(gpgKeyPath); err == nil {
			repoCorrect = true
		}
	}

	if !repoCorrect {
		p.logger.Info("Configuring Docker repository")
		_ = exec.CommandContext(ctx, "apt-get", "update").Run()
		_ = exec.CommandContext(ctx, "apt-get", "install", "-y", "ca-certificates", "curl", "gnupg").Run()

		_ = os.MkdirAll("/etc/apt/keyrings", 0755)
		_ = os.Remove(gpgKeyPath) // Remove if exists to avoid gpg prompt

		gpgCmd := "curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o " + gpgKeyPath
		if err := exec.CommandContext(ctx, "bash", "-c", gpgCmd).Run(); err != nil {
			return fmt.Errorf("failed to download docker gpg key: %w", err)
		}
		_ = os.Chmod(gpgKeyPath, 0644)

		if err := os.WriteFile(repoPath, []byte(repoLine), 0644); err != nil {
			return fmt.Errorf("failed to write docker.list: %w", err)
		}
		_ = exec.CommandContext(ctx, "apt-get", "update").Run()
	}

	if err := exec.CommandContext(ctx, "apt-get", "install", "-y", "containerd.io").Run(); err != nil {
		return fmt.Errorf("failed to install containerd.io: %w", err)
	}
	return nil
}
