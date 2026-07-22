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
	"github.com/EdgeNet-project/nodemanager/internal/metrics"
	"github.com/EdgeNet-project/nodemanager/internal/system"
	"github.com/EdgeNet-project/nodemanager/pkg/models"
	"go.uber.org/zap"
)

// Run starts the heartbeat process, pinging the orchestrator every 5 minutes.
func Run(ctx context.Context, logger *zap.Logger, cfg *config.Config) {
	systemUUID, _ := system.GetSystemUUID()
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	collector := metrics.NewCollector()

	logger.Info("Heartbeat loop started", zap.String("uuid", systemUUID))

	// Run first ping immediately
	ping(ctx, logger, cfg.Server, systemUUID, collector)

	for {
		select {
		case <-ctx.Done():
			logger.Info("Heartbeat loop stopped")
			return
		case <-ticker.C:
			ping(ctx, logger, cfg.Server, systemUUID, collector)
		}
	}
}

func ping(ctx context.Context, logger *zap.Logger, server, uuid string, collector metrics.Collector) {
	reqBody := models.PingRequest{
		SystemUUID: uuid,
	}

	m, err := collector.Collect()
	if err != nil {
		logger.Warn("Failed to collect metrics", zap.Error(err))
	} else if m != nil {
		reqBody.Metrics = &models.MetricsPayload{
			CPU: models.CPUMetrics{
				UsagePercent: m.CPU.UsagePercent,
				Load1:        m.CPU.Load1,
				Load5:        m.CPU.Load5,
				Load15:       m.CPU.Load15,
			},
			IO: models.IOMetrics{
				ReadBytesPerSec:  m.IO.ReadBytesPerSec,
				WriteBytesPerSec: m.IO.WriteBytesPerSec,
			},
			Network: models.NetworkMetrics{
				Interfaces: make([]models.InterfaceMetrics, len(m.Network.Interfaces)),
			},
		}
		for i, iface := range m.Network.Interfaces {
			reqBody.Metrics.Network.Interfaces[i] = models.InterfaceMetrics{
				Name:          iface.Name,
				RxBytes:       iface.RxBytes,
				TxBytes:       iface.TxBytes,
				RxBytesPerSec: iface.RxBytesPerSec,
				TxBytesPerSec: iface.TxBytesPerSec,
			}
		}
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
