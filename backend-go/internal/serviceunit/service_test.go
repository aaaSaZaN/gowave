package serviceunit

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallerMigratesExistingUnitAndKeepsBoundary(t *testing.T) {
	// The installer resolves symlinks (macOS keeps temp directories under /private).
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	dir, units, bin := filepath.Join(root, "install"), filepath.Join(root, "units"), filepath.Join(root, "bin")
	for _, d := range []string{dir, units, bin, filepath.Join(dir, "frontend", "dist")} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}
	// Layout left by the previous policy: everything writable by the service account,
	// a leftover update binary and a link planted inside the dashboard files.
	for name, body := range map[string]string{
		"gowave-server": "bin", "gowave-server.new": "planted", ".env": "APP_SECRET=x",
		"gowave.db": "", "gowave.db-wal": "", "frontend/dist/index.html": "<html>",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0666); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink("/etc/passwd", filepath.Join(dir, "frontend", "dist", "leak")); err != nil {
		t.Fatal(err)
	}
	// Ownership commands are observed; account lookup never touches system accounts.
	mock := `#!/bin/sh
case "${0##*/}" in
id) case "$*" in "-u") echo 0 ;; "-u gowave") echo 10001 ;; esac ;;
*) printf '%s %s\n' "${0##*/}" "$*" >> "$SERVICE_TEST_EVENTS" ;;
esac
`
	for _, name := range []string{"id", "chown", "useradd", "systemctl", "getent", "groupadd"} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(mock), 0755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	events := filepath.Join(root, "events")
	t.Setenv("SERVICE_TEST_EVENTS", events)
	oldUnit := "[Service]\nUser=root\nEnvironment=EXISTING_SETTING=preserved\n"
	unit := filepath.Join(units, "gowave.service")
	if err := os.WriteFile(unit, []byte(oldUnit), 0644); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		cmd := exec.Command("bash", "-s", "--", dir, units)
		cmd.Stdin = strings.NewReader(installer)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("install: %v: %s", err, out)
		}
	}
	data, err := os.ReadFile(filepath.Join(units, "gowave.service.d/90-unprivileged.conf"))
	if err != nil {
		t.Fatal(err)
	}
	for _, directive := range []string{"User=gowave", "Group=gowave", "ProtectSystem=strict", "ProtectHome=true", "PrivateDevices=true", "PrivateTmp=true", "NoNewPrivileges=true", "MemoryDenyWriteExecute=true", "ReadWritePaths=" + dir, "Restart=always"} {
		if !strings.Contains(string(data), directive+"\n") {
			t.Errorf("missing %s", directive)
		}
	}
	unchanged, _ := os.ReadFile(unit)
	if string(unchanged) != oldUnit {
		t.Fatal("installer overwrote existing unit configuration")
	}
	calls, _ := os.ReadFile(events)
	for _, call := range []string{
		"chown root:gowave " + dir,
		"chown -hR root:root " + dir + "/gowave-server",
		"chown -hR root:root " + dir + "/frontend",
		"chown root:gowave " + dir + "/.env",
		"systemctl daemon-reload",
		"systemctl enable --now gowave-update.path",
	} {
		if !strings.Contains(string(calls), call+"\n") {
			t.Errorf("missing %q in %s", call, calls)
		}
	}
	if strings.Contains(string(calls), "gowave:gowave "+dir+"\n") || strings.Contains(string(calls), "gowave:gowave "+dir+"/gowave-server") {
		t.Fatalf("code handed to the service account: %s", calls)
	}
	for _, db := range []string{"/gowave.db", "/gowave.db-wal"} {
		if !strings.Contains(string(calls), dir+db) {
			t.Errorf("database file %s not given to the service account: %s", db, calls)
		}
	}
	if info, err := os.Stat(dir); err != nil || info.Mode()&os.ModeSticky == 0 || info.Mode().Perm() != 0770 {
		t.Fatalf("install dir mode: %v %v", info.Mode(), err)
	}
	for name, perm := range map[string]os.FileMode{"gowave-server": 0644, "frontend/dist/index.html": 0644, ".env": 0640} {
		if info, err := os.Stat(filepath.Join(dir, name)); err != nil || info.Mode().Perm() != perm {
			t.Errorf("%s mode %v, want %v (%v)", name, info.Mode().Perm(), perm, err)
		}
	}
	for _, gone := range []string{"gowave-server.new", "frontend/dist/leak"} {
		if _, err := os.Lstat(filepath.Join(dir, gone)); !os.IsNotExist(err) {
			t.Errorf("%s was kept", gone)
		}
	}
	helper, err := os.ReadFile(filepath.Join(units, "gowave-update.service"))
	if err != nil || !strings.Contains(string(helper), "ExecStart="+dir+"/gowave-server update apply "+dir+"\n") {
		t.Fatalf("update helper unit: %s %v", helper, err)
	}
	// A path pointing at the system binary directory must never be recursively chowned.
	cmd := exec.Command("bash", "-s", "--", "/usr/local/bin", units)
	cmd.Stdin = strings.NewReader(installer)
	if err := cmd.Run(); err == nil {
		t.Fatal("accepted a shared system directory")
	}
}

func TestMigrationOnlyInsideGowaveUnit(t *testing.T) {
	old := cgroupFile
	t.Cleanup(func() { cgroupFile = old })
	for content, want := range map[string]bool{
		"0::/system.slice/gowave.service\n":                                     true,
		"12:pids:/system.slice/gowave.service\n0::/system.slice/gowave.service": true,
		"0::/system.slice/gowave-panel.service\n":                               false,
		"0::/system.slice/my-gowave.service\n":                                  false,
		"0::/user.slice/user-0.slice/session-1.scope\n":                         false,
	} {
		cgroupFile = filepath.Join(t.TempDir(), "cgroup")
		if err := os.WriteFile(cgroupFile, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		if got := inUnit("gowave.service"); got != want {
			t.Errorf("inUnit(%q) = %v, want %v", content, got, want)
		}
	}
}
