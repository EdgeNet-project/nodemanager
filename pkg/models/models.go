package models

import "encoding/json"

// NodeInfo represents the information sent during registration
type NodeInfo struct {
	NodeCode    string `json:"node_code"`
	PublicIP    string `json:"public_ip"`
	LocalIP     string `json:"local_ip"`
	WGPubKey    string `json:"wg_pubkey"`
	Arch        string `json:"arch"`
	OS          string `json:"os"`
	NATDetected bool   `json:"nat_detected"`
}

// RegistrationResponse represents the response from the registration API
type RegistrationResponse struct {
	NodeID string `json:"node_id"`
	Status string `json:"status"`
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
	Location json.RawMessage `json:"location"`
}

// NodeStatus represents the detailed status of a node
type NodeStatus struct {
	Status            string          `json:"status"`
	Hostname          string          `json:"hostname"`
	Role              string          `json:"role"`
	WGGatewayPubKey   string          `json:"wg_gateway_pubkey"`
	WGGatewayEndpoint string          `json:"wg_gateway_endpoint"`
	WGAssignedIP      string          `json:"wg_assigned_ip"`
	Provisioner       string          `json:"provisioner"`
	ProvisionerConfig json.RawMessage `json:"provisioner_config"`
}

// ProvisionInfo contains everything needed for provisioning
type ProvisionInfo struct {
	NodeID            string          `json:"node_id"`
	Hostname          string          `json:"hostname"`
	Token             string          `json:"token"`
	Endpoint          string          `json:"endpoint"`
	ProvisionerConfig json.RawMessage `json:"provisioner_config"`
}
