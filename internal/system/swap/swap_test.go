package swap

import (
	"testing"
)

func TestIsEnabled(t *testing.T) {
	// Just check it doesn't crash
	_ = IsEnabled()
}
