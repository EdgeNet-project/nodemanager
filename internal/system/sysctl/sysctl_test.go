package sysctl

import (
	"testing"
)

func TestGet(t *testing.T) {
	val, err := Get("net.ipv4.ip_forward")
	if err != nil {
		t.Skip("sysctl net.ipv4.ip_forward not accessible")
	}
	if val != "0" && val != "1" {
		t.Errorf("Unexpected sysctl value: %s", val)
	}
}
