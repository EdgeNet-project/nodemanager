package kubernetes

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"time"
)

func (p *KubernetesProvisioner) writeKubeletConfig(bootstrap *BootstrapResponse) error {
	p.logger.Info("Writing kubelet config")
	_ = os.MkdirAll("/var/lib/kubelet", 0755)

	// In a real scenario, we would use the YAML from bootstrap.KubeletConfig
	// and merge/override values.
	// For now, let's write a basic one as per requirements.

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

	return os.WriteFile("/var/lib/kubelet/config.yaml", []byte(config), 0644)
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

	return os.WriteFile("/etc/kubernetes/bootstrap-kubelet.conf", []byte(kubeconfig), 0600)
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

	if err := os.WriteFile("/etc/kubernetes/pki/ca.crt", caCert, 0644); err != nil {
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
	if err := os.WriteFile("/etc/systemd/system/kubelet.service.d/10-nodemanager.conf", []byte(override), 0644); err != nil {
		return err
	}

	_ = exec.CommandContext(ctx, "systemctl", "daemon-reload").Run()
	_ = exec.CommandContext(ctx, "systemctl", "enable", "kubelet").Run()
	return exec.CommandContext(ctx, "systemctl", "restart", "kubelet").Run()
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

func (p *KubernetesProvisioner) waitForNodeReadiness(ctx context.Context) error {
	p.logger.Info("Waiting for node to become ready")
	// In a real scenario, we'd use kubectl or k8s client to check node status.
	// For now, let's just check if kubelet is running.
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
			cmd := exec.CommandContext(ctx, "systemctl", "is-active", "kubelet")
			if err := cmd.Run(); err == nil {
				return nil
			}
		}
	}
}
