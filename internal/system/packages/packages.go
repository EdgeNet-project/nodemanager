package packages

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// GetVersion returns the installed version of a package.
func GetVersion(ctx context.Context, name string) (string, error) {
	if _, err := exec.LookPath("dnf"); err == nil {
		return getVersionDnf(ctx, name)
	} else if _, err := exec.LookPath("apt-get"); err == nil {
		return getVersionApt(ctx, name)
	}
	return "", fmt.Errorf("no supported package manager found")
}

func getVersionDnf(ctx context.Context, name string) (string, error) {
	out, err := exec.CommandContext(ctx, "rpm", "-q", name, "--queryformat", "%{VERSION}").Output()
	if err != nil {
		return "", fmt.Errorf("package %s not installed", name)
	}
	return strings.TrimSpace(string(out)), nil
}

func getVersionApt(ctx context.Context, name string) (string, error) {
	out, err := exec.CommandContext(ctx, "dpkg-query", "-W", "-f=${Version}", name).Output()
	if err != nil {
		return "", fmt.Errorf("package %s not installed", name)
	}
	return strings.TrimSpace(string(out)), nil
}
