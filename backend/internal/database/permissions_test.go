package database

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestFixDirectoryPermissionsPreservesStickyBit(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("Unix directory permissions required")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, os.ModeSticky|0o700); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := fixDirectoryPermissions(dir, os.Getgid()); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(dir)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode()&os.ModeSticky == 0 || info.Mode().Perm() != 0o770 {
			t.Fatalf("directory mode = %v, want sticky 0770", info.Mode())
		}
	}
}

func TestFixPermissionsNoop(t *testing.T) {
	// In-memory and PostgreSQL paths should always succeed without errors.
	if err := FixPermissions(":memory:"); err != nil {
		t.Fatalf("FixPermissions(:memory:) = %v", err)
	}
	if err := FixPermissions("file::memory:?cache=shared"); err != nil {
		t.Fatalf("FixPermissions(file::memory:) = %v", err)
	}
	if err := FixPermissions("postgresql://user:pass@localhost:5432/gowave"); err != nil {
		t.Fatalf("FixPermissions(postgres) = %v", err)
	}
	if err := FixPermissions(""); err != nil {
		t.Fatalf("FixPermissions('') = %v", err)
	}
}

func TestFixPermissionsLocalFile(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "gowave.db")
	walPath := filepath.Join(tmp, "gowave.db-wal")
	shmPath := filepath.Join(tmp, "gowave.db-shm")

	if err := os.WriteFile(dbPath, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(walPath, []byte("wal"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(shmPath, []byte("shm"), 0o644); err != nil {
		t.Fatal(err)
	}

	// When not running as root, FixPermissions is a safe no-op.
	if err := FixPermissions(dbPath); err != nil {
		t.Fatalf("FixPermissions(%q) = %v", dbPath, err)
	}
}

func TestFixPermissionsDisabled(t *testing.T) {
	DisablePermissionFix = true
	defer func() { DisablePermissionFix = false }()
	if err := FixPermissions("/opt/gowave/gowave.db"); err != nil {
		t.Fatalf("FixPermissions with DisablePermissionFix = %v", err)
	}

	t.Setenv("GOWAVE_KEEP_ROOT_DB", "1")
	DisablePermissionFix = false
	if err := FixPermissions("/opt/gowave/gowave.db"); err != nil {
		t.Fatalf("FixPermissions with env = %v", err)
	}
}
