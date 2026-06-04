package crsqlite_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/federicoserini/mobile-db/storage/crsqlite"
)

func TestSyncEventsTableCreated(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := crsqlite.ApplyMetaDDL(context.Background(), db); err != nil {
		t.Fatalf("ApplyMetaDDL: %v", err)
	}
	// table must exist
	var name string
	err = db.QueryRowContext(context.Background(),
		`SELECT name FROM sqlite_master WHERE type='table' AND name='sync_events'`).Scan(&name)
	if err != nil {
		t.Fatalf("sync_events table not found: %v", err)
	}
}

func openTestDB(t *testing.T) *crsqlite.Handle {
	t.Helper()
	extPath := os.Getenv("CRSQLITE_EXT_PATH")
	if extPath == "" {
		t.Skip("CRSQLITE_EXT_PATH not set")
	}
	dir := t.TempDir()
	db, err := crsqlite.Open(filepath.Join(dir, "test.db"), "", extPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestEnsureMeta(t *testing.T) {
	h := openTestDB(t)
	ctx := context.Background()

	if err := crsqlite.EnsureMeta(ctx, h.DB()); err != nil {
		t.Fatalf("EnsureMeta: %v", err)
	}
	// Idempotent — second call must not error.
	if err := crsqlite.EnsureMeta(ctx, h.DB()); err != nil {
		t.Fatalf("EnsureMeta (2nd call): %v", err)
	}
}

func TestGetSchemaVersionDefault(t *testing.T) {
	h := openTestDB(t)
	ctx := context.Background()

	if err := crsqlite.EnsureMeta(ctx, h.DB()); err != nil {
		t.Fatalf("EnsureMeta: %v", err)
	}
	v, err := crsqlite.GetSchemaVersion(ctx, h.DB(), "app1", "ds1")
	if err != nil {
		t.Fatalf("GetSchemaVersion: %v", err)
	}
	if v != 1 {
		t.Fatalf("expected default version 1, got %d", v)
	}
}

func TestSetAndGetSchemaVersion(t *testing.T) {
	h := openTestDB(t)
	ctx := context.Background()

	if err := crsqlite.EnsureMeta(ctx, h.DB()); err != nil {
		t.Fatalf("EnsureMeta: %v", err)
	}
	if err := crsqlite.SetSchemaVersion(ctx, h.DB(), "app1", "ds1", 3); err != nil {
		t.Fatalf("SetSchemaVersion: %v", err)
	}
	v, err := crsqlite.GetSchemaVersion(ctx, h.DB(), "app1", "ds1")
	if err != nil {
		t.Fatalf("GetSchemaVersion: %v", err)
	}
	if v != 3 {
		t.Fatalf("expected version 3, got %d", v)
	}

	// Upsert with a new version.
	if err := crsqlite.SetSchemaVersion(ctx, h.DB(), "app1", "ds1", 5); err != nil {
		t.Fatalf("SetSchemaVersion upsert: %v", err)
	}
	v, err = crsqlite.GetSchemaVersion(ctx, h.DB(), "app1", "ds1")
	if err != nil {
		t.Fatalf("GetSchemaVersion after upsert: %v", err)
	}
	if v != 5 {
		t.Fatalf("expected version 5, got %d", v)
	}
}
