// Package serviceunit installs the native service's privilege boundary.
package serviceunit

import (
	_ "embed"
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
