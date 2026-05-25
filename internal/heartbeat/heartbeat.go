package heartbeat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/EdgeNet-project/nodemanager/internal/config"
	"github.com/EdgeNet-project/nodemanager/internal/system"
	"github.com/EdgeNet-project/nodemanager/pkg/models"
	"go.uber.org/zap"
)

// Run starts the heartbeat process, pinging the orchestrator every 5 minutes.
func Run(ctx context.Context, logger *zap.Logger, cfg *config.Config) {
	systemUUID, _ := system.GetSystemUUID()
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	logger.Info("Heartbeat loop started", zap.String("uuid", systemUUID))

	// Run first ping immediately
	ping(ctx, logger, cfg.Server, systemUUID)

	for {
		select {
		case <-ctx.Done():
			logger.Info("Heartbeat loop stopped")
			return
		case <-ticker.C:
			ping(ctx, logger, cfg.Server, systemUUID)
		}
	}
}

func ping(ctx context.Context, logger *zap.Logger, server, uuid string) {
	reqBody := models.PingRequest{
		SystemUUID: uuid,
	}
	data, err := json.Marshal(reqBody)
	if err != nil {
		logger.Error("Failed to marshal ping request", zap.Error(err))
		return
	}

	url := fmt.Sprintf("%s/api/node/ping", server)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(data))
	if err != nil {
		logger.Error("Failed to create ping request", zap.Error(err))
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logger.Warn("Ping request failed", zap.Error(err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		logger.Warn("Server returned error for ping", zap.Int("status", resp.StatusCode), zap.String("body", string(body)))
		return
	}

	var pingResp models.PingResponse
	if err := json.NewDecoder(resp.Body).Decode(&pingResp); err != nil {
		logger.Error("Failed to decode ping response", zap.Error(err))
		return
	}

	logger.Info("Heartbeat ping successful",
		zap.Bool("enabled", pingResp.Enabled),
		zap.String("status", pingResp.Status),
	)
}
