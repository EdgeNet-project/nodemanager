package kubernetes

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/EdgeNet-project/nodemanager/pkg/models"
	"go.uber.org/zap"
)

func (p *KubernetesProvisioner) writeKubeletConfig(bootstrap *BootstrapResponse) error {
	p.logger.Info("Writing kubelet config")
	_ = os.MkdirAll("/var/lib/kubelet", 0755)

	config := fmt.Sprintf(`kind: KubeletConfiguration
apiVersion: kubelet.config.k8s.io/v1beta1
authentication:
  anonymous:
    enabled: false
  webhook:
    enabled: true
  x509:
    clientCAFile: /etc/kubernetes/pki/ca.crt
authorization:
  mode: Webhook
cgroupDriver: systemd
clusterDNS:
  - %s
clusterDomain: cluster.local
rotateCertificates: true
serverTLSBootstrap: true
eventRecordQPS: 0
maxPods: 110
podPidsLimit: 4096
protectKernelDefaults: true
readOnlyPort: 0
`, bootstrap.ClusterDNS)

	configPath := "/var/lib/kubelet/config.yaml"
	if current, err := os.ReadFile(configPath); err == nil && string(current) == config {
		p.logger.Info("Kubelet config is already up to date")
		return nil
	}

	return os.WriteFile(configPath, []byte(config), 0644)
}

func (p *KubernetesProvisioner) writeBootstrapKubeconfig(bootstrap *BootstrapResponse) error {
	p.logger.Info("Writing bootstrap kubeconfig")
	_ = os.MkdirAll("/etc/kubernetes", 0755)

	kubeconfig := fmt.Sprintf(`apiVersion: v1
kind: Config
clusters:
- cluster:
    certificate-authority-data: %s
    server: %s
  name: cluster
users:
- name: kubelet-bootstrap
  user:
    token: %s
contexts:
- context:
    cluster: cluster
    user: kubelet-bootstrap
  name: bootstrap
current-context: bootstrap
`, bootstrap.CACert, bootstrap.APIServer, bootstrap.BootstrapToken)

	configPath := "/etc/kubernetes/bootstrap-kubelet.conf"
	if current, err := os.ReadFile(configPath); err == nil && string(current) == kubeconfig {
		p.logger.Info("Bootstrap kubeconfig is already up to date")
		return nil
	}

	return os.WriteFile(configPath, []byte(kubeconfig), 0600)
}

func (p *KubernetesProvisioner) writeKubernetesPKI(bootstrap *BootstrapResponse) error {
	// create the directory
	if err := os.MkdirAll("/etc/kubernetes/pki", 0755); err != nil {
		return fmt.Errorf("failed to create pki dir: %w", err)
	}

	// decode the CA cert (assuming it comes as base64 from your orchestrator)
	caCert, err := base64.StdEncoding.DecodeString(bootstrap.CACert)
	if err != nil {
		return fmt.Errorf("failed to decode CA cert: %w", err)
	}

	certPath := "/etc/kubernetes/pki/ca.crt"
	if current, err := os.ReadFile(certPath); err == nil && bytes.Equal(current, caCert) {
		p.logger.Info("CA certificate is already up to date")
		return nil
	}

	if err := os.WriteFile(certPath, caCert, 0644); err != nil {
		return fmt.Errorf("failed to write CA cert: %w", err)
	}

	return nil
}

func (p *KubernetesProvisioner) configureKubeletService(ctx context.Context, bootstrap *BootstrapResponse) error {
	p.logger.Info("Configuring kubelet service")
	_ = os.MkdirAll("/etc/systemd/system/kubelet.service.d", 0755)

	override := fmt.Sprintf(`[Service]
ExecStart=
ExecStart=/usr/bin/kubelet \
  --bootstrap-kubeconfig=/etc/kubernetes/bootstrap-kubelet.conf \
  --kubeconfig=/etc/kubernetes/kubelet.conf \
  --config=/var/lib/kubelet/config.yaml \
  --node-ip=%s
`, bootstrap.NodeIP)

	overridePath := "/etc/systemd/system/kubelet.service.d/10-nodemanager.conf"
	changed := false
	if current, err := os.ReadFile(overridePath); err != nil || string(current) != override {
		if err := os.WriteFile(overridePath, []byte(override), 0644); err != nil {
			return err
		}
		changed = true
	}

	if changed {
		_ = exec.CommandContext(ctx, "systemctl", "daemon-reload").Run()
	}

	_ = exec.CommandContext(ctx, "systemctl", "enable", "kubelet").Run()

	isActive := exec.CommandContext(ctx, "systemctl", "is-active", "kubelet").Run() == nil

	if changed || !isActive {
		p.logger.Info("Restarting kubelet service")
		return exec.CommandContext(ctx, "systemctl", "restart", "kubelet").Run()
	}

	p.logger.Info("Kubelet service is already correctly configured and running")
	return nil
}

func (p *KubernetesProvisioner) waitForTLSBootstrap(ctx context.Context) error {
	p.logger.Info("Waiting for TLS bootstrap to complete")
	timeout := time.After(15 * time.Minute)
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("timed out waiting for kubelet.conf")
		case <-ticker.C:
			if _, err := os.Stat("/etc/kubernetes/kubelet.conf"); err == nil {
				p.logger.Info("kubelet.conf found, TLS bootstrap complete")
				return nil
			}
		}
	}
}

func (p *KubernetesProvisioner) waitForNodeReadiness(ctx context.Context, node models.Node) error {
	p.logger.Info("Waiting for node to become ready")

	nodeName := node.Name
	if nodeName == "" {
		nodeName, _ = os.Hostname()
	}

	timeout := time.After(5 * time.Minute)
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("timed out waiting for node readiness")
		case <-ticker.C:
			// 1. Check if kubelet service is running
			if err := exec.CommandContext(ctx, "systemctl", "is-active", "kubelet").Run(); err != nil {
				p.logger.Debug("Kubelet service is not active yet")
				continue
			}

			// 2. Check container runtime health
			if err := exec.CommandContext(ctx, "crictl", "info").Run(); err != nil {
				p.logger.Debug("Container runtime is not healthy yet")
				continue
			}

			// 3. Check node registration and identity
			kubeconfig := "/etc/kubernetes/kubelet.conf"
			if _, err := os.Stat(kubeconfig); err != nil {
				p.logger.Debug("kubelet.conf not found yet")
				continue
			}

			// Check node registration and Ready condition
			readyCmd := exec.CommandContext(ctx, "kubectl", "--kubeconfig="+kubeconfig, "get", "node", nodeName, "-o", "jsonpath={.status.conditions[?(@.type==\"Ready\")].status}")
			readyOut, err := readyCmd.Output()
			if err != nil {
				p.logger.Debug("Failed to get node status (might not be registered yet or permission error)", zap.Error(err))
				continue
			}
			status := string(bytes.TrimSpace(readyOut))
			if status != "True" {
				p.logger.Debug("Node is registered but not Ready yet", zap.String("status", status))
				continue
			}

			// 4. Check Lease object heartbeat
			leaseCmd := exec.CommandContext(ctx, "kubectl", "--kubeconfig="+kubeconfig, "get", "lease", "-n", "kube-node-lease", nodeName)
			if err := leaseCmd.Run(); err != nil {
				p.logger.Debug("Node lease not found yet")
				continue
			}

			p.logger.Info("Node is registered, healthy, and ready")
			return nil
		}
	}
}
