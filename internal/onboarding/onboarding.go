package onboarding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/edgenet-project/edgenet-agent/internal/config"
	"github.com/edgenet-project/edgenet-agent/internal/network"
	"github.com/edgenet-project/edgenet-agent/internal/system"
	"github.com/edgenet-project/edgenet-agent/pkg/models"
	"go.uber.org/zap"
)

type onboardingState struct {
	mu     sync.RWMutex
	node   models.Node
	status string
	srv    *http.Server
	cancel context.CancelFunc
}

var state = &onboardingState{}

// Run handles the onboarding process
func Run(ctx context.Context, logger *zap.Logger, cfg *config.Config, id *models.Node) error {
	state.mu.Lock()
	state.node.Code = id.Code
	state.mu.Unlock()

	systemUUID, _ := system.GetSystemUUID()
	localIPs, _ := network.GetLocalIPs()
	localIP := ""
	if len(localIPs) > 0 {
		localIP = localIPs[0]
	}

	for {
		resp, err := checkin(cfg.Server, localIP, systemUUID, id.Code)
		if err != nil {
			logger.Warn("Checkin failed, retrying in 5 minutes", zap.Error(err))
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(5 * time.Minute):
				continue
			}
		}

		logger.Info("Checkin successful",
			zap.String("status", resp.Status),
			zap.String("node_name", resp.Name),
		)

		// Change hostname if a name is provided
		if resp.Name != "" {
			if err := system.SetHostname(resp.Name); err != nil {
				logger.Warn("Failed to set hostname", zap.String("hostname", resp.Name), zap.Error(err))
			} else {
				logger.Info("Hostname updated", zap.String("hostname", resp.Name))
			}
		}

		state.mu.Lock()
		state.status = resp.Status
		state.node.Name = resp.Name
		state.node.PublicIP = resp.PublicIP
		state.node.LocalIP = localIP
		nodeToSave := state.node
		state.mu.Unlock()

		// Store node state locally
		if err := saveNode(cfg.State, &nodeToSave); err != nil {
			logger.Warn("Failed to save node state locally", zap.Error(err))
		}

		// Manage UI and Server based on status
		updateUI(logger)
		manageServer(ctx, logger)

		state.mu.RLock()
		currentStatus := state.status
		state.mu.RUnlock()

		if currentStatus == "ENABLED" {
			logger.Info("Node is ENABLED. Onboarding completed.")
			return nil
		}

		// If REGISTERED or CHECKIN, wait and retry
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Minute):
		}
	}
}

func checkin(server, ip, uuid, code string) (*models.CheckinResponse, error) {
	reqBody := models.CheckinRequest{
		IP:         ip,
		SystemUUID: uuid,
		Code:       code,
	}
	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/api/node/checkin", server)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(body))
	}

	var checkinResp models.CheckinResponse
	if err := json.NewDecoder(resp.Body).Decode(&checkinResp); err != nil {
		return nil, err
	}

	return &checkinResp, nil
}

func saveNode(path string, node *models.Node) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(node, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func updateUI(logger *zap.Logger) {
	var message string
	state.mu.RLock()
	status := state.status
	code := state.node.Code
	name := state.node.Name
	state.mu.RUnlock()

	switch status {
	case "checkin":
		message = fmt.Sprintf(`This node is part of the PlanetLab testbed.

To activate this node you have to add it to your account:

1. Visit https://console.planetlab.io
2. Login or register
3. Go to "Add Node"
4. Use this code: %s

Node Name: %s

`, code, name)
	case "registered":
		message = `This node is part of the PlanetLab testbed.

This node is not enabled yet. Please wait for administrator approval.
`
	default:
		message = fmt.Sprintf(`This node is part of the PlanetLab testbed.

For more information, visit https://planetlab.io

Node Name: %s

`, name)
	}

	if err := os.WriteFile("/etc/issue", []byte(message), 0644); err != nil {
		logger.Warn("Failed to update /etc/issue", zap.Error(err))
	}
}

func manageServer(ctx context.Context, logger *zap.Logger) {
	state.mu.Lock()
	defer state.mu.Unlock()

	shouldRun := state.status == "CHECKIN" || state.status == "REGISTERED"

	if shouldRun && state.srv == nil {
		// Start server
		mux := http.NewServeMux()
		mux.HandleFunc("/", handleOnboarding)
		state.srv = &http.Server{
			Addr:    ":80",
			Handler: mux,
		}

		serverCtx, cancel := context.WithCancel(ctx)
		state.cancel = cancel

		go func(srv *http.Server) {
			logger.Info("Starting temporary onboarding web server on port 80")
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logger.Error("Onboarding server failed", zap.Error(err))
			}
		}(state.srv)

		// 10 minutes timeout logic as per previous requirement, but only starts once.
		go func() {
			select {
			case <-time.After(10 * time.Minute):
				logger.Info("10 minutes elapsed, shutting down onboarding server")
				StopServer(logger)
			case <-serverCtx.Done():
			}
		}()
	} else if !shouldRun && state.srv != nil {
		// Stop server
		state.stopServerLocked(logger)
	}
}

func StopServer(logger *zap.Logger) {
	state.mu.Lock()
	defer state.mu.Unlock()
	state.stopServerLocked(logger)
}

func (s *onboardingState) stopServerLocked(logger *zap.Logger) {
	if s.srv != nil {
		logger.Info("Shutting down onboarding server")
		if s.cancel != nil {
			s.cancel()
		}
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.srv.Shutdown(shutdownCtx); err != nil {
			logger.Error("Onboarding server shutdown failed", zap.Error(err))
		}
		s.srv = nil
	}
}

func handleOnboarding(w http.ResponseWriter, r *http.Request) {
	state.mu.RLock()
	status := state.status
	code := state.node.Code
	name := state.node.Name
	state.mu.RUnlock()

	var content string
	if status == "CHECKIN" {
		content = fmt.Sprintf(`<h1>This node is part of the PlanetLab testbed.</h1>
<p>To activate this node you have to add it to your account:</p>
<ol>
    <li>Visit <a href="https://planetlab.io">https://planetlab.io</a></li>
    <li>Login or register</li>
    <li>Go to "Add Node"</li>
    <li>Use this code: <strong>%s</strong></li>
</ol>
<p>Node Name: %s</p>`, code, name)
	} else if status == "REGISTERED" {
		content = `<h1>This node is part of the PlanetLab testbed.</h1>
<p>This node is not enabled yet. Please wait for administrator approval.</p>`
	} else {
		content = fmt.Sprintf(`<h1>This node is part of the PlanetLab testbed.</h1>
<p>Status: %s</p>`, status)
	}

	fmt.Fprintf(w, `<html>
<head><title>EdgeNet Node Onboarding</title></head>
<body>
%s
</body>
</html>`, content)
}

// UpdateIssue is kept for backward compatibility if needed, but UI is now managed by Run
func UpdateIssue(nodeCode string) error {
	state.mu.Lock()
	state.node.Code = nodeCode
	state.status = "CHECKIN" // Default if called directly
	state.mu.Unlock()
	updateUI(zap.NewNop())
	return nil
}

// StartTemporaryServer is kept for backward compatibility if needed
func StartTemporaryServer(ctx context.Context, logger *zap.Logger, nodeCode string) {
	state.mu.Lock()
	state.node.Code = nodeCode
	state.status = "CHECKIN"
	state.mu.Unlock()
	manageServer(ctx, logger)
}
