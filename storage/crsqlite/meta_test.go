package crsqlite_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/federicoserini/mobile-db/storage/crsqlite"
)

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
