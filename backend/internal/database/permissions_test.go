package database

import (
	"os"
	"path/filepath"
	"testing"
)

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
