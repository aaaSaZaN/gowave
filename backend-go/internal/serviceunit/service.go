// Package serviceunit installs the native service's privilege boundary.
package serviceunit

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

//go:embed install.sh
var installer string

func Install(dir, unitDir string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command("bash", "-s", "--", dir, unitDir, exe)
	cmd.Stdin = strings.NewReader(installer)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}

// cgroupFile is replaced in tests.
var cgroupFile = "/proc/self/cgroup"

// inUnit reports whether this process belongs to the named systemd unit.
func inUnit(name string) bool {
	data, err := os.ReadFile(cgroupFile)
	if err != nil {
		return false
	}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if strings.HasSuffix(line, "/"+name) {
			return true
		}
	}
	return false
}

// MigrateLegacy runs only inside the installer's root gowave.service, before any
// application work: older installers and in-panel updates started the panel as
// root. It applies the unprivileged policy and asks systemd to restart the unit,
// which then runs as the service account. Other root services are left alone and
// fail the root check instead of having their configuration rewritten.
func MigrateLegacy() (bool, error) {
	if os.Geteuid() != 0 || os.Getenv("INVOCATION_ID") == "" || !inUnit("gowave.service") {
		return false, nil
	}
	dir, err := os.Getwd()
	if err != nil {
		return true, err
	}
	if dir == "/" {
		dir = "/opt/gowave"
	}
	if err := Install(dir, "/etc/systemd/system"); err != nil {
		return true, fmt.Errorf("migrate unprivileged service: %w", err)
	}
	// --no-block: the restart stops this process, so do not wait for it.
	if err := exec.Command("systemctl", "--no-block", "restart", "gowave.service").Run(); err != nil {
		return true, fmt.Errorf("restart migrated service: %w", err)
	}
	return true, nil
}
