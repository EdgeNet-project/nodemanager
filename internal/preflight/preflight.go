package preflight

import (
	"context"
	"fmt"
	"net"
	"runtime"
	"time"

	"github.com/EdgeNet-project/nodemanager/internal/network"
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
func Run(ctx context.Context, logger *zap.Logger, orchestratorHost string) (*Result, error) {
	// Wait for network to be ready (local interface, gateway, internet)
	if err := WaitUntilNetworkReady(ctx, logger, orchestratorHost); err != nil {
		return nil, err
	}

	pubIP, err := network.GetPublicIP(orchestratorHost)
	if err != nil {
		return nil, fmt.Errorf("failed to get public IP: %w", err)
	}

	localIPs, err := network.GetLocalIPs()
	if err != nil {
		return nil, fmt.Errorf("failed to get local IPs: %w", err)
	}

	return RunWithIPs(pubIP, localIPs), nil
}

// CheckNetwork performs basic network connectivity checks.
// Note: ICMP may be filtered on some networks, so ping failures are only logged
// as warnings rather than treated as fatal errors.
func CheckNetwork(logger *zap.Logger, orchestratorHost string) error {
	// 1. Local interface up
	if !network.IsAnyInterfaceUp() {
		return fmt.Errorf("no local network interface is up")
	}

	// 2. Gateway reachable (warning only: ICMP might be filtered)
	gw, err := network.GetDefaultGateway()
	if err != nil {
		logger.Warn("Could not determine default gateway", zap.Error(err))
	} else if !network.Ping(gw) {
		logger.Warn("Default gateway is not reachable via ping (ICMP might be filtered)",
			zap.String("gateway", gw))
	}

	// 3. DNS server reachable (warning only: ICMP might be filtered)
	if !network.Ping("8.8.8.8") {
		logger.Warn("DNS server 8.8.8.8 is not reachable via ping (ICMP might be filtered)")
	}

	// 4. Resolve orchestrator host (this must succeed)
	if orchestratorHost == "" {
		return fmt.Errorf("orchestrator.host is not configured")
	}
	addrs, err := net.LookupHost(orchestratorHost)
	if err != nil {
		return fmt.Errorf("failed to resolve orchestrator host %q: %w", orchestratorHost, err)
	}
	if len(addrs) == 0 {
		return fmt.Errorf("no IP addresses returned for orchestrator host %q", orchestratorHost)
	}

	// 5. Ping orchestrator IP (warning only: ICMP might be filtered)
	orchestratorIP := addrs[0]
	if !network.Ping(orchestratorIP) {
		logger.Warn("Orchestrator is not reachable via ping (ICMP might be filtered)",
			zap.String("host", orchestratorHost),
			zap.String("ip", orchestratorIP))
	}

	return nil
}

// WaitUntilNetworkReady blocks until network connectivity is confirmed, retrying every 5 minutes.
func WaitUntilNetworkReady(ctx context.Context, logger *zap.Logger, orchestratorHost string) error {
	for {
		err := CheckNetwork(logger, orchestratorHost)
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
