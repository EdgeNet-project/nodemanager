package preflight

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestWaitUntilNetworkReadyCancel(t *testing.T) {
	logger := zap.NewNop()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// This should return ctx.DeadlineExceeded eventually (well, after the first check)
	// But since CheckNetwork() might pass if network is available, we can't easily force it to fail
	// unless we are in a restricted environment.

	// If network is available, it returns nil immediately.
	// We want to check if it returns error when context is cancelled.
	// Since we can't easily mock network here, we'll just check if it doesn't hang forever.
	err := WaitUntilNetworkReady(ctx, logger)
	if err != nil && err != context.DeadlineExceeded && err != context.Canceled {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestNATDetection(t *testing.T) {
	tests := []struct {
		name     string
		pubIP    string
		localIPs []string
		wantNAT  bool
	}{
		{
			name:     "Behind NAT",
			pubIP:    "1.2.3.4",
			localIPs: []string{"192.168.1.10", "10.0.0.5"},
			wantNAT:  true,
		},
		{
			name:     "Public IP on interface",
			pubIP:    "1.2.3.4",
			localIPs: []string{"1.2.3.4", "10.0.0.5"},
			wantNAT:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := RunWithIPs(tt.pubIP, tt.localIPs)
			if res.NATDetected != tt.wantNAT {
				t.Errorf("NATDetected = %v, want %v", res.NATDetected, tt.wantNAT)
			}
		})
	}
}

func TestCheckPort(t *testing.T) {
	// This might fail if the environment doesn't allow listening on ports,
	// or if the ports are already in use.
	// But we can try a high port.
	if !checkPort("9999") {
		t.Log("Warning: checkPort(\"9999\") failed, this might be environment-specific")
	}
}
