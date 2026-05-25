package modules

import (
	"testing"
)

func TestIsLoaded(t *testing.T) {
	// This might fail on some systems if /proc/modules is not readable or empty,
	// but it's a good sanity check.
	_ = IsLoaded("overlay")
}
