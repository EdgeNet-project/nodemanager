package onboarding

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/EdgeNet-project/nodemanager/pkg/models"
)

func TestCheckin(t *testing.T) {
	// Start a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check the request body
		var req models.CheckinRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		
		if req.ProductUUID != "test-uuid" {
			t.Errorf("expected product UUID 'test-uuid', got '%s'", req.ProductUUID)
		}

		// Respond with a dummy response
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(models.CheckinResponse{Status: "registered"})
	}))
	defer server.Close()

	// Call checkin
	_, err := checkin(server.URL, "1.1.1.1", "uuid", "test-uuid", "code", "arch", "distro", "version", "kernel")
	if err != nil {
		t.Fatalf("checkin failed: %v", err)
	}
}
