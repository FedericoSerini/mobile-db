package crsqlite_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/federicoserini/mobile-db/core"
	"github.com/federicoserini/mobile-db/storage/crsqlite"
)

// Compile-time interface compliance check.
var _ core.StorageBackend = (*crsqlite.Backend)(nil)

func openTestBackend(t *testing.T) *crsqlite.Backend {
	t.Helper()
	extPath := os.Getenv("CRSQLITE_EXT_PATH")
	if extPath == "" {
		t.Skip("CRSQLITE_EXT_PATH not set")
	}
	dir := t.TempDir()
	b, err := crsqlite.NewBackend(filepath.Join(dir, "backend.db"), "", extPath)
	if err != nil {
		t.Fatalf("NewBackend: %v", err)
	}
	t.Cleanup(func() { b.Close() })
	return b
}

func TestBackendMergeOpsAndGetDelta(t *testing.T) {
	b := openTestBackend(t)
	ctx := context.Background()

	ops := []core.CRDTOp{
		{OpID: "op1", DocID: "doc1", Field: "name", Value: "Alice",
			Timestamp: core.HLC{WallTime: 100, Logical: 0, DeviceID: "devA"}, DeviceID: "devA"},
		{OpID: "op2", DocID: "doc1", Field: "age", Value: float64(30),
			Timestamp: core.HLC{WallTime: 200, Logical: 0, DeviceID: "devA"}, DeviceID: "devA"},
	}

	if err := b.MergeOps(ctx, "app1", "user1", "ds1", ops); err != nil {
		t.Fatalf("MergeOps: %v", err)
	}

	// GetDelta from zero should return both ops.
	got, maxHLC, err := b.GetDelta(ctx, "app1", "user1", "ds1", core.HLC{})
	if err != nil {
		t.Fatalf("GetDelta: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 ops, got %d", len(got))
	}
	if maxHLC.WallTime != 200 {
		t.Fatalf("expected maxHLC.WallTime=200, got %d", maxHLC.WallTime)
	}

	// GetDelta since WallTime=100 should return only op2.
	got2, _, err := b.GetDelta(ctx, "app1", "user1", "ds1", core.HLC{WallTime: 100})
	if err != nil {
		t.Fatalf("GetDelta since 100: %v", err)
	}
	if len(got2) != 1 {
		t.Fatalf("expected 1 op after ts=100, got %d", len(got2))
	}
	if got2[0].OpID != "op2" {
		t.Fatalf("expected op2, got %s", got2[0].OpID)
	}
}

func TestBackendMergeOpsIdempotent(t *testing.T) {
	b := openTestBackend(t)
	ctx := context.Background()

	op := core.CRDTOp{
		OpID: "op1", DocID: "doc1", Field: "x", Value: 1.0,
		Timestamp: core.HLC{WallTime: 10, Logical: 0, DeviceID: "devA"}, DeviceID: "devA",
	}

	if err := b.MergeOps(ctx, "app1", "user1", "ds1", []core.CRDTOp{op}); err != nil {
		t.Fatalf("MergeOps (1st): %v", err)
	}
	if err := b.MergeOps(ctx, "app1", "user1", "ds1", []core.CRDTOp{op}); err != nil {
		t.Fatalf("MergeOps (2nd duplicate): %v", err)
	}

	count, err := b.OpCount(ctx, "app1", "user1", "ds1")
	if err != nil {
		t.Fatalf("OpCount: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 op after duplicate MergeOps, got %d", count)
	}
}

func TestBackendGetSnapshot(t *testing.T) {
	b := openTestBackend(t)
	ctx := context.Background()

	// Two ops for the same field — LWW should keep the higher-timestamp one.
	ops := []core.CRDTOp{
		{OpID: "op1", DocID: "doc1", Field: "name", Value: "Alice",
			Timestamp: core.HLC{WallTime: 100, Logical: 0, DeviceID: "devA"}, DeviceID: "devA"},
		{OpID: "op2", DocID: "doc1", Field: "name", Value: "Alicia",
			Timestamp: core.HLC{WallTime: 200, Logical: 0, DeviceID: "devA"}, DeviceID: "devA"},
	}
	if err := b.MergeOps(ctx, "app1", "user1", "ds1", ops); err != nil {
		t.Fatalf("MergeOps: %v", err)
	}

	snap, maxHLC, err := b.GetSnapshot(ctx, "app1", "user1", "ds1")
	if err != nil {
		t.Fatalf("GetSnapshot: %v", err)
	}
	if snap["doc1.name"] != "Alicia" {
		t.Fatalf("expected LWW winner 'Alicia', got %v", snap["doc1.name"])
	}
	if maxHLC.WallTime != 200 {
		t.Fatalf("expected maxHLC.WallTime=200, got %d", maxHLC.WallTime)
	}
}

func TestBackendSchemaVersion(t *testing.T) {
	b := openTestBackend(t)
	ctx := context.Background()

	v, err := b.SchemaVersion(ctx, "app1", "ds1")
	if err != nil {
		t.Fatalf("SchemaVersion: %v", err)
	}
	if v != 1 {
		t.Fatalf("expected default schema version 1, got %d", v)
	}
}

func TestBackendCompact(t *testing.T) {
	b := openTestBackend(t)
	ctx := context.Background()

	// Insert 3 ops: 2 updates to the same field, 1 to a different field.
	ops := []core.CRDTOp{
		{OpID: "op1", DocID: "doc1", Field: "name", Value: "Alice",
			Timestamp: core.HLC{WallTime: 100, Logical: 0, DeviceID: "devA"}, DeviceID: "devA"},
		{OpID: "op2", DocID: "doc1", Field: "name", Value: "Alicia",
			Timestamp: core.HLC{WallTime: 200, Logical: 0, DeviceID: "devA"}, DeviceID: "devA"},
		{OpID: "op3", DocID: "doc1", Field: "age", Value: float64(30),
			Timestamp: core.HLC{WallTime: 150, Logical: 0, DeviceID: "devA"}, DeviceID: "devA"},
	}
	if err := b.MergeOps(ctx, "app1", "user1", "ds1", ops); err != nil {
		t.Fatalf("MergeOps: %v", err)
	}

	if err := b.Compact(ctx, "app1", "user1", "ds1"); err != nil {
		t.Fatalf("Compact: %v", err)
	}

	// After compaction: 1 op for name (winner: op2) + 1 op for age = 2 total.
	count, err := b.OpCount(ctx, "app1", "user1", "ds1")
	if err != nil {
		t.Fatalf("OpCount after compact: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 ops after compact, got %d", count)
	}

	// The snapshot should still reflect the correct winner.
	snap, _, err := b.GetSnapshot(ctx, "app1", "user1", "ds1")
	if err != nil {
		t.Fatalf("GetSnapshot after compact: %v", err)
	}
	if snap["doc1.name"] != "Alicia" {
		t.Fatalf("expected 'Alicia' after compact, got %v", snap["doc1.name"])
	}
}

func TestBackendOpCountEmpty(t *testing.T) {
	b := openTestBackend(t)
	ctx := context.Background()

	count, err := b.OpCount(ctx, "app1", "user1", "ds_empty")
	if err != nil {
		t.Fatalf("OpCount on empty dataset: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0, got %d", count)
	}
}
