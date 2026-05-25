package kubernetes

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"go.uber.org/zap"
)

func (p *KubernetesProvisioner) installKubernetesComponents(ctx context.Context) error {
	version := p.cfg.KubernetesVersion
	p.logger.Info("Installing Kubernetes components", zap.String("version", version))

	if _, err := exec.LookPath("dnf"); err == nil {
		return p.installKubernetesComponentsDnf(ctx)
	} else if _, err := exec.LookPath("apt-get"); err == nil {
		return p.installKubernetesComponentsApt(ctx)
	}

	return fmt.Errorf("no supported package manager found (dnf or apt-get)")
}

func (p *KubernetesProvisioner) installKubernetesComponentsDnf(ctx context.Context) error {
	version := p.cfg.KubernetesVersion
	p.logger.Info("Installing Kubernetes components using dnf", zap.String("version", version))

	repo := fmt.Sprintf(`[kubernetes]
name=Kubernetes
baseurl=https://pkgs.k8s.io/core:/stable:/v%s/rpm/
enabled=1
gpgcheck=1
gpgkey=https://pkgs.k8s.io/core:/stable:/v%s/rpm/repodata/repomd.xml.key
`, version, version)
	if err := os.WriteFile("/etc/yum.repos.d/kubernetes.repo", []byte(repo), 0644); err != nil {
		return fmt.Errorf("failed to write kubernetes.repo: %w", err)
	}

	if err := exec.CommandContext(ctx, "dnf", "install", "-y", "kubelet", "kubectl", "cri-tools").Run(); err != nil {
		return fmt.Errorf("failed to install kubernetes components via dnf: %w", err)
	}
	return nil
}

func (p *KubernetesProvisioner) installKubernetesComponentsApt(ctx context.Context) error {
	version := p.cfg.KubernetesVersion
	p.logger.Info("Installing Kubernetes components using apt", zap.String("version", version))

	_ = exec.CommandContext(ctx, "apt-get", "update").Run()
	_ = exec.CommandContext(ctx, "apt-get", "install", "-y", "apt-transport-https", "ca-certificates", "curl", "gnupg").Run()

	_ = os.MkdirAll("/etc/apt/keyrings", 0755)
	gpgKeyPath := "/etc/apt/keyrings/kubernetes-apt-keyring.gpg"
	_ = os.Remove(gpgKeyPath)

	gpgCmd := fmt.Sprintf("curl -fsSL https://pkgs.k8s.io/core:/stable:/v%s/deb/Release.key | gpg --dearmor -o %s", version, gpgKeyPath)
	if err := exec.CommandContext(ctx, "bash", "-c", gpgCmd).Run(); err != nil {
		return fmt.Errorf("failed to download kubernetes gpg key: %w", err)
	}
	_ = os.Chmod(gpgKeyPath, 0644)

	repoLine := fmt.Sprintf("deb [signed-by=%s] https://pkgs.k8s.io/core:/stable:/v%s/deb/ /\n", gpgKeyPath, version)
	if err := os.WriteFile("/etc/apt/sources.list.d/kubernetes.list", []byte(repoLine), 0644); err != nil {
		return fmt.Errorf("failed to write kubernetes.list: %w", err)
	}

	_ = exec.CommandContext(ctx, "apt-get", "update").Run()
	if err := exec.CommandContext(ctx, "apt-get", "install", "-y", "kubelet", "kubectl", "cri-tools").Run(); err != nil {
		return fmt.Errorf("failed to install kubernetes components via apt: %w", err)
	}
	return nil
}

func (p *KubernetesProvisioner) configureAPIServerConnectivity() error {
	p.logger.Info("Configuring API server connectivity in /etc/hosts")
	ip := p.cfg.APIServerIP
	hostLine := fmt.Sprintf("%s api.cluster", ip)

	content, err := os.ReadFile("/etc/hosts")
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	found := false
	for i, line := range lines {
		if strings.Contains(line, "api.cluster") {
			lines[i] = hostLine
			found = true
			break
		}
	}

	if !found {
		lines = append(lines, hostLine)
	}

	return os.WriteFile("/etc/hosts", []byte(strings.Join(lines, "\n")), 0644)
}
