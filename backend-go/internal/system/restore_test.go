package system

import (
	"bytes"
	"compress/gzip"
	"testing"

	"remnawave-go/internal/database"
	"remnawave-go/internal/testdb"
)

func TestRestoreRepairsSequencesWithMixedPrimaryKeys(t *testing.T) {
	db := testdb.Open(t)
	var dump bytes.Buffer
	writer := gzip.NewWriter(&dump)
	// The schema also contains tables without id and tables with a non-serial
	// id, so the restore must skip both when repairing sequences.
	if _, err := writer.Write([]byte("COPY public.users (id, short_uuid, username) FROM stdin;\n42\trestored\trestored\n\\.\n")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := NewBackupRestoreService(db).restoreSqlGz(&dump); err != nil {
		t.Fatal(err)
	}
	var restored database.User
	if err := db.First(&restored, 42).Error; err != nil {
		t.Fatal(err)
	}
	next := database.User{Username: "next", ShortUUID: "next"}
	if err := db.Create(&next).Error; err != nil {
		t.Fatal(err)
	}
	if next.ID != 43 {
		t.Fatalf("next user ID = %d, want 43", next.ID)
	}
}
