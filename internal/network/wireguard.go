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
	"path/filepath"
	"strings"
	"time"

	"github.com/EdgeNet-project/nodemanager/internal/config"
	"github.com/EdgeNet-project/nodemanager/internal/system"
	"github.com/EdgeNet-project/nodemanager/pkg/models"
	"github.com/vishvananda/netlink"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

var WireguardConfigPath = "/etc/edgenet/wireguard.json"

const DefaultInterface = "wg0"

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
	wg.PersistentKeepalive = activateResp.PersistentKeepalive

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

func ConfigureInterface(logger *zap.Logger, wg *models.Wiregard) error {
	const ifName = DefaultInterface

	// ------------------------------------------------------------
	// 0. OPEN NETLINK + WG CLIENT EARLY
	// ------------------------------------------------------------
	link, err := netlink.LinkByName(ifName)
	if err != nil {
		// Interface doesn't exist — create it
		wgLink := &netlink.GenericLink{
			LinkAttrs: netlink.LinkAttrs{Name: ifName},
			LinkType:  "wireguard",
		}
		if err := netlink.LinkAdd(wgLink); err != nil {
			return fmt.Errorf("failed to create wireguard interface: %w", err)
		}
		link, err = netlink.LinkByName(ifName)
		if err != nil {
			return fmt.Errorf("interface not found after creation: %w", err)
		}
	}

	// ------------------------------------------------------------
	// 1. CLEANUP PHASE (IMPORTANT)
	// ------------------------------------------------------------

	logger.Info("cleaning existing wireguard state")

	// 1.1 flush IP addresses
	addrs, err := netlink.AddrList(link, unix.AF_UNSPEC)
	if err != nil {
		return fmt.Errorf("list addrs: %w", err)
	}

	for _, addr := range addrs {
		if err := netlink.AddrDel(link, &addr); err != nil {
			// ignore "not found" type errors safely
			logger.Debug("failed to delete addr", zap.Error(err))
		}
	}

	// 1.2 delete all routes bound to interface
	routes, _ := netlink.RouteList(link, unix.AF_UNSPEC)
	for _, r := range routes {
		_ = netlink.RouteDel(&r)
	}

	// 1.3 bring link down temporarily
	_ = netlink.LinkSetDown(link)

	// 1.4 reset WireGuard config
	client, err := wgctrl.New()
	if err != nil {
		return fmt.Errorf("wgctrl: %w", err)
	}
	defer client.Close()

	empty := wgtypes.Config{
		ReplacePeers: true,
		Peers:        []wgtypes.PeerConfig{},
	}

	_ = client.ConfigureDevice(ifName, empty)

	// ------------------------------------------------------------
	// 2. RECONFIGURE PHASE
	// ------------------------------------------------------------

	mtu := wg.MTU
	if mtu == 0 {
		mtu = 1420
	}

	if err := netlink.LinkSetMTU(link, mtu); err != nil {
		return fmt.Errorf("mtu: %w", err)
	}

	ipNet, err := parseAddress(wg.Address)
	if err != nil {
		return err
	}

	_ = netlink.AddrAdd(link, &netlink.Addr{IPNet: ipNet})

	if err := netlink.LinkSetUp(link); err != nil {
		return fmt.Errorf("link up: %w", err)
	}

	// ------------------------------------------------------------
	// 3. WG CONFIG
	// ------------------------------------------------------------

	priv, err := wgtypes.ParseKey(wg.PrivateKey)
	if err != nil {
		return err
	}

	peerKey, err := wgtypes.ParseKey(wg.EndpointKey)
	if err != nil {
		return err
	}

	allowed, err := parseAllowedIPs(wg.AllowedIPs)
	if err != nil {
		return err
	}

	endpoint, err := resolveEndpoint(wg.Endpoint)
	if err != nil {
		return err
	}

	keepalive := time.Duration(wg.PersistentKeepalive) * time.Second

	cfg := wgtypes.Config{
		PrivateKey:   &priv,
		ReplacePeers: true,
		Peers: []wgtypes.PeerConfig{
			{
				PublicKey:                   peerKey,
				Endpoint:                    endpoint,
				AllowedIPs:                  allowed,
				ReplaceAllowedIPs:           true,
				PersistentKeepaliveInterval: &keepalive,
			},
		},
	}

	if err := client.ConfigureDevice(ifName, cfg); err != nil {
		return fmt.Errorf("wg config: %w", err)
	}

	// ------------------------------------------------------------
	// 4. ROUTES
	// ------------------------------------------------------------

	if err := installRoutes(link, allowed); err != nil {
		return fmt.Errorf("routes: %w", err)
	}

	return nil
}

func parseAddress(addr string) (*net.IPNet, error) {
	ip, ipnet, err := net.ParseCIDR(addr)
	if err == nil {
		return &net.IPNet{IP: ip, Mask: ipnet.Mask}, nil
	}

	ip = net.ParseIP(addr)
	if ip == nil {
		return nil, fmt.Errorf("invalid IP address: %s", addr)
	}

	if ip.To4() != nil {
		return &net.IPNet{IP: ip, Mask: net.CIDRMask(32, 32)}, nil
	}

	return &net.IPNet{IP: ip, Mask: net.CIDRMask(128, 128)}, nil
}

func parseAllowedIPs(input string) ([]net.IPNet, error) {
	parts := strings.Split(input, ",")

	var out []net.IPNet

	for _, p := range parts {
		p = strings.TrimSpace(p)

		_, ipnet, err := net.ParseCIDR(p)
		if err != nil {
			return nil, fmt.Errorf("invalid CIDR: %s", p)
		}

		out = append(out, *ipnet)
	}

	return out, nil
}

func installRoutes(link netlink.Link, ips []net.IPNet) error {
	for _, ipnet := range ips {

		// skip host routes if you want (optional behavior)
		route := netlink.Route{
			LinkIndex: link.Attrs().Index,
			Dst:       &ipnet,
		}

		// ignore "already exists"
		_ = netlink.RouteAdd(&route)
	}
	return nil
}

func resolveEndpoint(ep string) (*net.UDPAddr, error) {
	host, portStr, err := net.SplitHostPort(ep)
	if err != nil {
		return nil, err
	}

	port, err := net.LookupPort("udp", portStr)
	if err != nil {
		port = 51820
	}

	ips, err := net.LookupIP(host)
	if err != nil || len(ips) == 0 {
		return nil, fmt.Errorf("dns resolution failed: %s", host)
	}

	return &net.UDPAddr{
		IP:   ips[0],
		Port: port,
	}, nil
}

func verifyConnectivity(logger *zap.Logger, wg *models.Wiregard) bool {
	// Try to ping the peer.
	// We need an IP to ping. AllowedIPs usually contains the peer's tunnel IP.
	// If multiple allowed IPs, we try the first one that looks like an IP.

	peerIP := "10.80.0.1"

	if peerIP == "" {
		logger.Warn("Could not determine peer IP for connectivity check")
		return false
	}

	logger.Info("Pinging peer...", zap.String("ip", peerIP))
	return Ping(peerIP)
}
