package kubernetes

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/EdgeNet-project/nodemanager/internal/system/packages"
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

	repoPath := "/etc/yum.repos.d/kubernetes.repo"
	existingRepo, err := os.ReadFile(repoPath)
	if err == nil && string(existingRepo) == repo {
		p.logger.Info("Kubernetes repo already configured correctly")
	} else {
		if err := os.WriteFile(repoPath, []byte(repo), 0644); err != nil {
			return fmt.Errorf("failed to write kubernetes.repo: %w", err)
		}
	}

	// Check if packages are already installed and match version
	pkgs := []string{"kubelet", "kubectl", "cri-tools"}
	allCorrect := true
	for _, pkg := range pkgs {
		v, err := packages.GetVersion(ctx, pkg)
		if err != nil || !strings.HasPrefix(v, version) {
			allCorrect = false
			break
		}
	}

	if allCorrect {
		p.logger.Info("Kubernetes packages already installed and match version")
		return nil
	}

	if err := exec.CommandContext(ctx, "dnf", "install", "-y", "kubelet", "kubectl", "cri-tools").Run(); err != nil {
		return fmt.Errorf("failed to install kubernetes components via dnf: %w", err)
	}
	return nil
}

func (p *KubernetesProvisioner) installKubernetesComponentsApt(ctx context.Context) error {
	version := p.cfg.KubernetesVersion
	p.logger.Info("Installing Kubernetes components using apt", zap.String("version", version))

	gpgKeyPath := "/etc/apt/keyrings/kubernetes-apt-keyring.gpg"
	repoPath := "/etc/apt/sources.list.d/kubernetes.list"
	repoLine := fmt.Sprintf("deb [signed-by=%s] https://pkgs.k8s.io/core:/stable:/v%s/deb/ /\n", gpgKeyPath, version)

	repoCorrect := false
	if data, err := os.ReadFile(repoPath); err == nil && string(data) == repoLine {
		if _, err := os.Stat(gpgKeyPath); err == nil {
			repoCorrect = true
		}
	}

	if !repoCorrect {
		_ = exec.CommandContext(ctx, "apt-get", "update").Run()
		_ = exec.CommandContext(ctx, "apt-get", "install", "-y", "apt-transport-https", "ca-certificates", "curl", "gnupg").Run()

		_ = os.MkdirAll("/etc/apt/keyrings", 0755)
		_ = os.Remove(gpgKeyPath)

		gpgCmd := fmt.Sprintf("curl -fsSL https://pkgs.k8s.io/core:/stable:/v%s/deb/Release.key | gpg --dearmor -o %s", version, gpgKeyPath)
		if err := exec.CommandContext(ctx, "bash", "-c", gpgCmd).Run(); err != nil {
			return fmt.Errorf("failed to download kubernetes gpg key: %w", err)
		}
		_ = os.Chmod(gpgKeyPath, 0644)

		if err := os.WriteFile(repoPath, []byte(repoLine), 0644); err != nil {
			return fmt.Errorf("failed to write kubernetes.list: %w", err)
		}
		_ = exec.CommandContext(ctx, "apt-get", "update").Run()
	}

	// Check if packages are already installed and match version
	pkgs := []string{"kubelet", "kubectl", "cri-tools"}
	allCorrect := true
	for _, pkg := range pkgs {
		v, err := packages.GetVersion(ctx, pkg)
		if err != nil || !strings.HasPrefix(v, version) {
			allCorrect = false
			break
		}
	}

	if allCorrect {
		p.logger.Info("Kubernetes packages already installed and match version")
		return nil
	}

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

	newContent := strings.Join(lines, "\n")
	if string(content) == newContent {
		p.logger.Info("API server connectivity already configured in /etc/hosts")
		return nil
	}

	return os.WriteFile("/etc/hosts", []byte(newContent), 0644)
}
