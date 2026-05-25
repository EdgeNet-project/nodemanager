package kubernetes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/EdgeNet-project/nodemanager/internal/system"
	"github.com/EdgeNet-project/nodemanager/pkg/models"
	"go.uber.org/zap"
)

const kubernetesStateFile = "/etc/edgenet/kubernetes.json"

type BootstrapPayload struct {
	Uuid           string `json:"uuid"`
	Hostname       string `json:"hostname"`
	BootstrapToken string `json:"bootstrap_token"`
}

type BootstrapResponse struct {
	ClusterName    string `json:"cluster_name"`
	APIServer      string `json:"api_server"`
	CACert         string `json:"ca_cert"`
	BootstrapToken string `json:"bootstrap_token"`
	NodeIP         string `json:"node_ip"`
	ClusterDNS     string `json:"cluster_dns"`
}

func (p *KubernetesProvisioner) retrieveBootstrapPayload(ctx context.Context, node models.Node) (*BootstrapResponse, error) {
	p.logger.Info("Retrieving bootstrap payload from orchestrator")

	systemUUID, _ := system.GetSystemUUID()
	hostname, _ := os.Hostname()

	payload := BootstrapPayload{
		Uuid:     systemUUID,
		Hostname: hostname,
	}

	// Read existing bootstrap token if present
	if data, err := os.ReadFile(kubernetesStateFile); err == nil {
		var existingResp BootstrapResponse
		if err := json.Unmarshal(data, &existingResp); err == nil {
			payload.BootstrapToken = existingResp.BootstrapToken
		}
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/api/node/kubernetes", p.cfg.Server)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		_ = os.Remove(kubernetesStateFile)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		_ = os.Remove(kubernetesStateFile)
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("orchestrator returned status %d: %s", resp.StatusCode, string(body))
	}

	var bootstrapResp BootstrapResponse
	if err := json.NewDecoder(resp.Body).Decode(&bootstrapResp); err != nil {
		_ = os.Remove(kubernetesStateFile)
		return nil, err
	}

	// Save bootstrap response to file
	_ = os.MkdirAll(filepath.Dir(kubernetesStateFile), 0755)
	respData, err := json.MarshalIndent(bootstrapResp, "", "  ")
	if err == nil {
		if err := os.WriteFile(kubernetesStateFile, respData, 0644); err != nil {
			p.logger.Warn("Failed to save bootstrap response", zap.Error(err))
		}
	}

	return &bootstrapResp, nil
}
