package system

import (
	"testing"
)

func TestParseOsRelease(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantID   string
		wantVer  string
	}{
		{
			name:    "ubuntu",
			content: "ID=ubuntu\nVERSION_ID=\"22.04\"\n",
			wantID:  "ubuntu",
			wantVer: "22.04",
		},
		{
			name:    "rocky",
			content: "ID=\"rocky\"\nVERSION_ID=9.4\n",
			wantID:  "rocky",
			wantVer: "9.4",
		},
		{
			name:    "no version",
			content: "ID=debian\n",
			wantID:  "debian",
			wantVer: "unknown",
		},
		{
			name:    "mixed order",
			content: "VERSION_ID=12\nID=debian\n",
			wantID:  "debian",
			wantVer: "12",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, version := parseOsRelease(tt.content)
			if id != tt.wantID || version != tt.wantVer {
				t.Errorf("parseOsRelease() = %v, %v; want %v, %v", id, version, tt.wantID, tt.wantVer)
			}
		})
	}
}
