package kubernetes

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/EdgeNet-project/nodemanager/internal/config"
	"github.com/EdgeNet-project/nodemanager/pkg/models"
	"go.uber.org/zap"
)

type KubernetesProvisioner struct {
	logger *zap.Logger
	cfg    *config.Config
}

func New(logger *zap.Logger, cfg *config.Config) *KubernetesProvisioner {
	return &KubernetesProvisioner{
		logger: logger,
		cfg:    cfg,
	}
}

func (p *KubernetesProvisioner) Name() string {
	return "kubernetes"
}

func (p *KubernetesProvisioner) IsProvisioned(ctx context.Context) (bool, error) {
	// Check if kubelet is running and config exists
	if _, err := os.Stat("/etc/kubernetes/kubelet.conf"); err != nil {
		return false, nil
	}
	// Also check if kubelet service is active
	cmd := exec.CommandContext(ctx, "systemctl", "is-active", "kubelet")
	if err := cmd.Run(); err != nil {
		return false, nil
	}
	return true, nil
}

func (p *KubernetesProvisioner) Provision(ctx context.Context, node models.Node) error {
	p.logger.Info("Starting Kubernetes provisioning phase")

	// 1. Retrieve Bootstrap Payload
	bootstrap, err := p.retrieveBootstrapPayload(ctx, node)
	if err != nil {
		return fmt.Errorf("failed to retrieve bootstrap payload: %w", err)
	}

	// 2. OS Preparation
	if err := p.prepareOS(ctx); err != nil {
		return fmt.Errorf("failed to prepare OS: %w", err)
	}

	// 3. Install containerd
	if err := p.installContainerd(ctx); err != nil {
		return fmt.Errorf("failed to install containerd: %w", err)
	}

	// 4. Install Kubernetes Components
	if err := p.installKubernetesComponents(ctx); err != nil {
		return fmt.Errorf("failed to install kubernetes components: %w", err)
	}

	// 5. Write kubelet Config
	if err := p.writeKubeletConfig(bootstrap); err != nil {
		return fmt.Errorf("failed to write kubelet config: %w", err)
	}

	// 6. Bootstrap kubeconfig
	if err := p.writeBootstrapKubeconfig(bootstrap); err != nil {
		return fmt.Errorf("failed to write bootstrap kubeconfig: %w", err)
	}

	// 6a. Write kubernetes API CA pki
	if err := p.writeKubernetesPKI(bootstrap); err != nil {
		return fmt.Errorf("failed to write API CA pki: %w", err)
	}

	// 7. Configure API Server Connectivity
	if err := p.configureAPIServerConnectivity(); err != nil {
		return fmt.Errorf("failed to configure api server connectivity: %w", err)
	}

	// 8. Configure kubelet Service
	if err := p.configureKubeletService(ctx, bootstrap); err != nil {
		return fmt.Errorf("failed to configure kubelet service: %w", err)
	}

	// 9. Wait for TLS Bootstrap
	if err := p.waitForTLSBootstrap(ctx); err != nil {
		return fmt.Errorf("failed to wait for TLS bootstrap: %w", err)
	}

	// 10. Node Readiness
	if err := p.waitForNodeReadiness(ctx, node); err != nil {
		return fmt.Errorf("failed to wait for node readiness: %w", err)
	}

	// 11. Notify Orchestrator
	if err := p.notifyNodeReady(ctx, node); err != nil {
		return fmt.Errorf("failed to notify node readiness: %w", err)
	}

	p.logger.Info("Kubernetes provisioning completed successfully")
	return nil
}

func (p *KubernetesProvisioner) Deprovision(ctx context.Context) error {
	p.logger.Info("Deprovisioning Kubernetes")
	// Stop and disable kubelet
	_ = exec.CommandContext(ctx, "systemctl", "stop", "kubelet").Run()
	_ = exec.CommandContext(ctx, "systemctl", "disable", "kubelet").Run()

	// Remove files
	_ = os.RemoveAll("/etc/kubernetes")
	_ = os.RemoveAll("/var/lib/kubelet")
	_ = os.Remove("/etc/systemd/system/kubelet.service.d/10-nodemanager.conf")
	_ = exec.CommandContext(ctx, "systemctl", "daemon-reload").Run()

	return nil
}
