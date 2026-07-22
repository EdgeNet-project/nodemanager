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

// Wireguard represents the WireGuard configuration
type Wireguard struct {
	Endpoint            string `json:"endpoint"`
	EndpointKey         string `json:"endpoint_key"`
	Address             string `json:"address"`
	AllowedIPs          string `json:"allowed_ips"`
	MTU                 int    `json:"mtu"`
	PersistentKeepalive int    `json:"persistent_keepalive"`
	PrivateKey          string `json:"private_key"`
	PublicKey           string `json:"public_key"`
}

// HardwareInfo represents the hardware details
type HardwareInfo struct {
	Family  string `json:"product_family"`
	Name    string `json:"product_name"`
	Serial  string `json:"product_serial"`
	SKU     string `json:"product_sku"`
	UUID    string `json:"product_uuid"`
	Version string `json:"product_version"`
	Vendor  string `json:"sys_vendor"`
}

// CheckinRequest represents the parameters for the checkin API
type CheckinRequest struct {
	IP         string       `json:"ip"`
	SystemUUID string       `json:"uuid"`
	Code       string       `json:"code"`
	Arch       string       `json:"arch"`
	Distro     string       `json:"distro"`
	Version    string       `json:"version"`
	Kernel     string       `json:"kernel"`
	Hardware   HardwareInfo `json:"hardware"`
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

// PingRequest represents the parameters for the ping API
type PingRequest struct {
	SystemUUID string          `json:"uuid"`
	Metrics    *MetricsPayload `json:"metrics,omitempty"`
}

type MetricsPayload struct {
	CPU     CPUMetrics     `json:"cpu"`
	IO      IOMetrics      `json:"io"`
	Network NetworkMetrics `json:"network"`
}

type CPUMetrics struct {
	UsagePercent float64 `json:"usage_percent"`
	Load1        float64 `json:"load1"`
	Load5        float64 `json:"load5"`
	Load15       float64 `json:"load15"`
}

type IOMetrics struct {
	ReadBytesPerSec  float64 `json:"read_bytes_per_sec"`
	WriteBytesPerSec float64 `json:"write_bytes_per_sec"`
}

type NetworkMetrics struct {
	Interfaces []InterfaceMetrics `json:"interfaces"`
}

type InterfaceMetrics struct {
	Name string `json:"name"`

	RxBytes uint64 `json:"rx_bytes"`
	TxBytes uint64 `json:"tx_bytes"`

	RxBytesPerSec float64 `json:"rx_bytes_per_sec"`
	TxBytesPerSec float64 `json:"tx_bytes_per_sec"`
}

// PingResponse represents the response from the ping API
type PingResponse struct {
	Enabled bool   `json:"enabled"`
	Status  string `json:"status"`
}

// ReadyRequest represents the parameters for the kubernetes ready API
type ReadyRequest struct {
	SystemUUID string `json:"uuid"`
	Name       string `json:"name"`
}
