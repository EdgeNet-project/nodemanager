package network

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"edge-net.org/nodemanager/internal/config"
	"edge-net.org/nodemanager/internal/system"
	"edge-net.org/nodemanager/pkg/models"
	"go.uber.org/zap"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

var WireguardConfigPath = "/etc/edgenet/wiregard.json"
var DefaultInterface = "wg0"

// SetupWireguard handles Phase 4: WireGuard configuration and activation
func SetupWireguard(ctx context.Context, logger *zap.Logger, cfg *config.Config, id *models.Node) error {
	logger.Info("Setting up WireGuard...")

	// 1. Load or generate keys
	wgConfig, err := LoadOrGenerateKeys(logger)
	if err != nil {
		return fmt.Errorf("failed to load or generate WireGuard keys: %w", err)
	}

	// 2. Activation loop
	systemUUID, _ := system.GetSystemUUID()
	for {
		err := activate(logger, cfg.Server, systemUUID, id.Code, wgConfig)
		if err == nil {
			logger.Info("WireGuard activation successful")
			break
		}

		logger.Warn("WireGuard activation failed, retrying in 5 minutes", zap.Error(err))
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Minute):
		}
	}

	// 3. Save config
	if err := SaveWireguardConfig(wgConfig); err != nil {
		logger.Warn("Failed to save WireGuard configuration", zap.Error(err))
	}

	// 4. Interface configuration loop
	for {
		err := ConfigureInterface(logger, wgConfig)
		if err == nil {
			logger.Info("WireGuard interface configured successfully")

			// 5. Verify connectivity (ping peer)
			if verifyConnectivity(logger, wgConfig) {
				logger.Info("WireGuard connectivity verified")
				return nil
			}
			err = fmt.Errorf("connectivity check failed")
		}

		logger.Warn("WireGuard setup or connectivity check failed, retrying in 5 minutes", zap.Error(err))
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Minute):
		}
	}
}

// LoadOrGenerateKeys loads keys from the config file or generates new ones
func LoadOrGenerateKeys(logger *zap.Logger) (*models.Wiregard, error) {
	if _, err := os.Stat(WireguardConfigPath); err == nil {
		data, err := os.ReadFile(WireguardConfigPath)
		if err == nil {
			var wg models.Wiregard
			if err := json.Unmarshal(data, &wg); err == nil && wg.PrivateKey != "" && wg.PublicKey != "" {
				logger.Info("Loaded existing WireGuard keys")
				return &wg, nil
			}
		}
	}

	logger.Info("Generating new WireGuard keys...")
	priv, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return nil, err
	}

	return &models.Wiregard{
		PrivateKey: priv.String(),
		PublicKey:  priv.PublicKey().String(),
	}, nil
}

func activate(logger *zap.Logger, server, uuid, code string, wg *models.Wiregard) error {
	reqBody := models.ActivateRequest{
		SystemUUID: uuid,
		Code:       code,
		PublicKey:  wg.PublicKey,
	}
	data, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/api/node/wiregard", server)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(body))
	}

	var activateResp models.Wiregard
	if err := json.NewDecoder(resp.Body).Decode(&activateResp); err != nil {
		return err
	}

	logger.Info("Wiregard request successful",
		zap.Any("response", activateResp),
	)

	// Update our config with received values
	wg.Endpoint = activateResp.Endpoint
	wg.EndpointKey = activateResp.EndpointKey
	wg.Address = activateResp.Address
	wg.AllowedIPs = activateResp.AllowedIPs
	wg.MTU = activateResp.MTU

	return nil
}

// SaveWireguardConfig persists the configuration to disk
func SaveWireguardConfig(wg *models.Wiregard) error {
	dir := filepath.Dir(WireguardConfigPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(wg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(WireguardConfigPath, data, 0600)
}

// ConfigureInterface sets up the WireGuard interface
func ConfigureInterface(logger *zap.Logger, wg *models.Wiregard) error {
	// 1. Create interface if it doesn't exist
	if err := exec.Command("ip", "link", "add", "dev", DefaultInterface, "type", "wireguard").Run(); err != nil {
		// If it already exists, it's fine
		logger.Debug("Interface might already exist", zap.Error(err))
	}

	// 2. Set IP address
	// Remove existing addresses first to avoid conflicts? Or just add.
	// We'll use 'ip addr replace' if available, but 'add' is safer if we know it's ours.
	// Actually, let's flush it or just try to add.
	exec.Command("ip", "addr", "flush", "dev", DefaultInterface).Run()
	if err := exec.Command("ip", "addr", "add", wg.Address, "dev", DefaultInterface).Run(); err != nil {
		return fmt.Errorf("failed to set IP address: %w", err)
	}

	// 3. Set MTU and bring up
	mtu := fmt.Sprintf("%d", wg.MTU)
	if wg.MTU == 0 {
		mtu = "1420"
	}
	if err := exec.Command("ip", "link", "set", "mtu", mtu, "up", "dev", DefaultInterface).Run(); err != nil {
		return fmt.Errorf("failed to bring up interface: %w", err)
	}

	// 4. Configure WireGuard device using wgctrl
	client, err := wgctrl.New()
	if err != nil {
		return fmt.Errorf("failed to create wgctrl client: %w", err)
	}
	defer client.Close()

	privKey, err := wgtypes.ParseKey(wg.PrivateKey)
	if err != nil {
		return fmt.Errorf("invalid private key: %w", err)
	}

	peerKey, err := wgtypes.ParseKey(wg.EndpointKey)
	if err != nil {
		return fmt.Errorf("invalid endpoint key: %w", err)
	}

	_, allowedIPs, err := net.ParseCIDR(wg.AllowedIPs)
	if err != nil {
		// Fallback if it's just an IP
		ip := net.ParseIP(wg.AllowedIPs)
		if ip != nil {
			mask := net.CIDRMask(32, 32)
			if ip.To4() == nil {
				mask = net.CIDRMask(128, 128)
			}
			allowedIPs = &net.IPNet{IP: ip, Mask: mask}
		} else {
			return fmt.Errorf("invalid allowed IPs: %w", err)
		}
	}

	peer := wgtypes.PeerConfig{
		PublicKey:         peerKey,
		ReplaceAllowedIPs: true,
		AllowedIPs:        []net.IPNet{*allowedIPs},
		Endpoint:          &net.UDPAddr{}, // Will be set below
	}

	// Parse endpoint
	host, port, err := net.SplitHostPort(wg.Endpoint)
	if err == nil {
		ips, err := net.LookupIP(host)
		if err == nil && len(ips) > 0 {
			p, _ := strconv.Atoi(port)
			if p == 0 {
				p = 51820
			}
			peer.Endpoint = &net.UDPAddr{
				IP:   ips[0],
				Port: p,
			}
		}
	}

	config := wgtypes.Config{
		PrivateKey:   &privKey,
		ReplacePeers: true,
		Peers:        []wgtypes.PeerConfig{peer},
	}

	if err := client.ConfigureDevice(DefaultInterface, config); err != nil {
		return fmt.Errorf("failed to configure WireGuard device: %w", err)
	}

	return nil
}

func verifyConnectivity(logger *zap.Logger, wg *models.Wiregard) bool {
	// Try to ping the peer.
	// We need an IP to ping. AllowedIPs usually contains the peer's tunnel IP.
	// If multiple allowed IPs, we try the first one that looks like an IP.

	peerIP := ""
	if strings.Contains(wg.AllowedIPs, "/") {
		ip, _, _ := net.ParseCIDR(wg.AllowedIPs)
		if ip != nil {
			peerIP = ip.String()
		}
	} else {
		peerIP = wg.AllowedIPs
	}

	if peerIP == "" {
		logger.Warn("Could not determine peer IP for connectivity check")
		return false
	}

	logger.Info("Pinging peer...", zap.String("ip", peerIP))
	return Ping(peerIP)
}
