package models

import "encoding/json"

// Node represents the unified node state used across all phases
type Node struct {
	Name     string `json:"name"`
	Code     string `json:"code"`
	Enabled  bool   `json:"enabled"`
	PublicIP string `json:"public_ip"`
	LocalIP  string `json:"local_ip"`
}

// Wiregard represents the WireGuard configuration
type Wiregard struct {
	Endpoint            string `json:"endpoint"`
	EndpointKey         string `json:"endpoint_key"`
	Address             string `json:"address"`
	AllowedIPs          string `json:"allowed_ips"`
	MTU                 int    `json:"mtu"`
	PersistentKeepalive int    `json:"persistent_keepalive"`
	PrivateKey          string `json:"private_key"`
	PublicKey           string `json:"public_key"`
}

// CheckinRequest represents the parameters for the checkin API
type CheckinRequest struct {
	IP         string `json:"ip"`
	SystemUUID string `json:"uuid"`
	Code       string `json:"code"`
}

// CheckinResponse represents the response from the checkin API
type CheckinResponse struct {
	Name     string          `json:"name"`
	PublicIP string          `json:"public_ip"`
	Status   string          `json:"status"`
	Enabled  bool            `json:"enabled"`
	Location json.RawMessage `json:"location"`
}

// ActivateRequest represents the parameters for the activate API
type ActivateRequest struct {
	SystemUUID string `json:"uuid"`
	Code       string `json:"code"`
	PublicKey  string `json:"public_key"`
}
