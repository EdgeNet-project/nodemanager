package preflight

import (
	"context"
	"fmt"
	"net"
	"runtime"
	"time"

	"edge-net.org/nodemanager/internal/network"
	"go.uber.org/zap"
	"golang.zx2c4.com/wireguard/wgctrl"
)

// Result contains the results of the preflight checks
type Result struct {
	PublicIP    string
	LocalIPs    []string
	NATDetected bool
	Port80Open  bool
	Port443Open bool
	WGSupported bool
}

// Run executes all preflight checks, waiting for network availability if necessary.
func Run(ctx context.Context, logger *zap.Logger) (*Result, error) {
	// Wait for network to be ready (local interface, gateway, internet)
	if err := WaitUntilNetworkReady(ctx, logger); err != nil {
		return nil, err
	}

	pubIP, err := network.GetPublicIP()
	if err != nil {
		return nil, fmt.Errorf("failed to get public IP: %w", err)
	}

	localIPs, err := network.GetLocalIPs()
	if err != nil {
		return nil, fmt.Errorf("failed to get local IPs: %w", err)
	}

	return RunWithIPs(pubIP, localIPs), nil
}

// CheckNetwork performs basic network connectivity checks
func CheckNetwork() error {
	// 1. Local interface up
	if !network.IsAnyInterfaceUp() {
		return fmt.Errorf("no local network interface is up")
	}

	// 2. Gateway reachable
	gw, err := network.GetDefaultGateway()
	if err != nil {
		// On non-linux systems this might fail, for now we might skip or handle differently
		// but since we are likely on Linux, we want this.
		// If we can't get it, we still check internet ping.
	} else {
		if !network.Ping(gw) {
			return fmt.Errorf("gateway %s is not reachable", gw)
		}
	}

	// 3. Internet reachable
	if !network.Ping("8.8.8.8") {
		return fmt.Errorf("internet is not reachable (ping 8.8.8.8 failed)")
	}

	return nil
}

// WaitUntilNetworkReady blocks until network connectivity is confirmed, retrying every 5 minutes.
func WaitUntilNetworkReady(ctx context.Context, logger *zap.Logger) error {
	for {
		err := CheckNetwork()
		if err == nil {
			logger.Info("Network connectivity verified")
			return nil
		}

		logger.Warn("Network not available, retrying in 5 minutes", zap.Error(err))

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Minute):
		}
	}
}

// RunWithIPs executes preflight checks with provided IPs (useful for testing)
func RunWithIPs(pubIP string, localIPs []string) *Result {
	res := &Result{
		PublicIP: pubIP,
		LocalIPs: localIPs,
	}

	res.NATDetected = true
	for _, ip := range localIPs {
		if ip == pubIP {
			res.NATDetected = false
			break
		}
	}

	// 2. Check ports 80 and 443
	res.Port80Open = checkPort("80")
	res.Port443Open = checkPort("443")

	// 3. Check WireGuard support
	res.WGSupported = checkWireGuard()

	return res
}

func checkPort(port string) bool {
	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return false
	}
	ln.Close()
	return true
}

func checkWireGuard() bool {
	// Try to create a wgctrl client
	client, err := wgctrl.New()
	if err != nil {
		return false
	}
	defer client.Close()

	// On Linux, we can check if the module is loaded by trying to list devices
	// Even if there are no devices, it shouldn't return an error if supported
	_, err = client.Devices()
	if err != nil {
		// On non-linux systems wgctrl might behave differently
		if runtime.GOOS != "linux" {
			// For now, if not linux, we might just return false or a different check
			// But the nodemanager is likely targeting Linux
			return false
		}
		return false
	}

	return true
}
