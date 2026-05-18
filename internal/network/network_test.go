package network

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetPublicIP(t *testing.T) {
	expectedIP := "1.2.3.4"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(expectedIP))
	}))
	defer server.Close()

	// Temporarily override services for testing
	oldServices := publicIPServices
	publicIPServices = []string{server.URL}
	defer func() { publicIPServices = oldServices }()

	ip, err := GetPublicIP()
	if err != nil {
		t.Fatalf("GetPublicIP failed: %v", err)
	}
	if ip != expectedIP {
		t.Errorf("Expected IP %s, got %s", expectedIP, ip)
	}
}

func TestGetLocalIPs(t *testing.T) {
	ips, err := GetLocalIPs()
	if err != nil {
		t.Fatalf("GetLocalIPs failed: %v", err)
	}
	if len(ips) == 0 {
		t.Log("No local IPs found (might be normal in some environments, but unlikely)")
	}
	for _, ip := range ips {
		if ip == "127.0.0.1" {
			t.Errorf("GetLocalIPs should not return loopback address")
		}
	}
}
