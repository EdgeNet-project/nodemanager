package kubernetes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/EdgeNet-project/nodemanager/internal/system"
	"github.com/EdgeNet-project/nodemanager/pkg/models"
	"go.uber.org/zap"
)

func (p *KubernetesProvisioner) notifyNodeReady(ctx context.Context, node models.Node) error {
	p.logger.Info("Notifying orchestrator that node is ready")

	systemUUID, _ := system.GetSystemUUID()
	nodeName := node.Name
	if nodeName == "" {
		nodeName, _ = os.Hostname()
	}

	payload := models.ReadyRequest{
		SystemUUID: systemUUID,
		Name:       nodeName,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal ready request: %w", err)
	}

	url := fmt.Sprintf("%s/api/node/kubernetes/ready", p.cfg.Server)

	for {
		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(data))
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			p.logger.Error("Failed to send ready notification, retrying in 5 minutes", zap.Error(err))
		} else {
			if resp.StatusCode == http.StatusOK {
				p.logger.Info("Successfully notified orchestrator that node is ready")
				resp.Body.Close()
				return nil
			}

			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			p.logger.Error("Orchestrator returned error for ready notification, retrying in 5 minutes",
				zap.Int("status_code", resp.StatusCode),
				zap.String("response", string(body)))
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Minute):
			// Retry
		}
	}
}
