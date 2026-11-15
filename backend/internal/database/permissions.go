package database

import (
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

// DisablePermissionFix skips changing database file ownership and permissions.
var DisablePermissionFix bool

// FixPermissions ensures SQLite database files and their WAL/SHM artifacts
// are owned by the gowave service account and group-writable when executed as root.
func FixPermissions(databaseURL string) error {
	if DisablePermissionFix || os.Getenv("GOWAVE_KEEP_ROOT_DB") == "true" || os.Getenv("GOWAVE_KEEP_ROOT_DB") == "1" {
		return nil
	}
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		return nil
	}
	if os.Geteuid() != 0 {
		return nil
	}
	if databaseURL == "" || databaseURL == ":memory:" || strings.HasPrefix(databaseURL, "file::memory:") ||
		strings.HasPrefix(databaseURL, "postgres://") || strings.HasPrefix(databaseURL, "postgresql://") {
		return nil
	}

	u, err := user.Lookup("gowave")
	if err != nil {
		// No gowave system user; nothing to chown to.
		return nil
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return nil
	}
	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		return nil
	}

	cleanPath := strings.TrimPrefix(databaseURL, "file:")
	if idx := strings.Index(cleanPath, "?"); idx != -1 {
		cleanPath = cleanPath[:idx]
	}

	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return err
	}

	dir := filepath.Dir(absPath)
	base := filepath.Base(absPath)

	patterns := []string{
		filepath.Join(dir, base+"*"),
		filepath.Join(dir, "*.db*"),
		filepath.Join(dir, "*.sqlite*"),
	}

	seen := make(map[string]bool)
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		for _, file := range matches {
			if seen[file] {
				continue
			}
			seen[file] = true

			fi, err := os.Lstat(file)
			if err != nil || fi.IsDir() || fi.Mode()&os.ModeSymlink != 0 {
				continue
			}

			_ = os.Chown(file, uid, gid)
			_ = os.Chmod(file, 0o660)
		}
	}

	// Ensure the database directory has group gowave and group write/execute
	// permissions if it is a dedicated install directory (like /opt/gowave).
	if dir != "/" && dir != "/tmp" && dir != "/root" && !strings.HasPrefix(dir, "/home") {
		if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
			if stat, ok := fi.Sys().(*syscall.Stat_t); ok {
				if stat.Gid != uint32(gid) {
					_ = os.Chown(dir, int(stat.Uid), gid)
				}
				_ = os.Chmod(dir, fi.Mode().Perm()|0o070)
			}
		}
	}

	return nil
}
