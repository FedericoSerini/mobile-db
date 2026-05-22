package crsqlite_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/federicoserini/mobile-db/storage/crsqlite"
)

func TestOpenCreatesDB(t *testing.T) {
	extPath := os.Getenv("CRSQLITE_EXT_PATH")
	if extPath == "" {
		t.Skip("CRSQLITE_EXT_PATH not set")
	}
	dir := t.TempDir()
	db, err := crsqlite.Open(filepath.Join(dir, "test.db"), "", extPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	// crsql_db_version() is always present once the cr-sqlite extension loads.
	// (crsql_version() was removed in v0.16.x; crsql_db_version() returns the
	// current logical clock for the database — 0 on a fresh DB is fine.)
	var dbVersion int64
	if err := db.DB().QueryRowContext(context.Background(), "SELECT crsql_db_version()").Scan(&dbVersion); err != nil {
		t.Fatalf("crsql_db_version: %v", err)
	}
	t.Logf("cr-sqlite loaded OK; crsql_db_version()=%d", dbVersion)

	// Also verify the virtual table exists.
	rows, err := db.DB().QueryContext(context.Background(), "SELECT COUNT(*) FROM crsql_changes")
	if err != nil {
		t.Fatalf("crsql_changes virtual table: %v", err)
	}
	rows.Close()
}
