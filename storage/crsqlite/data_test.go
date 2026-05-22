package crsqlite_test

import (
	"context"
	"testing"

	"github.com/federicoserini/mobile-db/core"
	"github.com/federicoserini/mobile-db/storage/crsqlite"
)

const (
	testApp     = "app1"
	testUser    = "user1"
	testDataset = "contacts"
)

func setupDataset(t *testing.T) *crsqlite.Handle {
	t.Helper()
	h := openTestDB(t)
	ctx := context.Background()
	if err := crsqlite.EnsureMeta(ctx, h.DB()); err != nil {
		t.Fatalf("EnsureMeta: %v", err)
	}
	if err := crsqlite.EnsureDataset(ctx, h.DB(), testApp, testUser, testDataset); err != nil {
		t.Fatalf("EnsureDataset: %v", err)
	}
	return h
}

func TestEnsureDatasetIdempotent(t *testing.T) {
	h := openTestDB(t)
	ctx := context.Background()

	if err := crsqlite.EnsureDataset(ctx, h.DB(), testApp, testUser, testDataset); err != nil {
		t.Fatalf("EnsureDataset (1st): %v", err)
	}
	// Must be safe to call again.
	if err := crsqlite.EnsureDataset(ctx, h.DB(), testApp, testUser, testDataset); err != nil {
		t.Fatalf("EnsureDataset (2nd): %v", err)
	}
}

func TestInsertAndQueryOps(t *testing.T) {
	h := setupDataset(t)
	ctx := context.Background()

	ops := []core.CRDTOp{
		{OpID: "op1", DocID: "doc1", Field: "name", Value: "Alice",
			Timestamp: core.HLC{WallTime: 1000, Logical: 0, DeviceID: "devA"}, DeviceID: "devA"},
		{OpID: "op2", DocID: "doc1", Field: "age", Value: float64(30),
			Timestamp: core.HLC{WallTime: 1001, Logical: 0, DeviceID: "devA"}, DeviceID: "devA"},
		{OpID: "op3", DocID: "doc2", Field: "name", Value: "Bob",
			Timestamp: core.HLC{WallTime: 1002, Logical: 0, DeviceID: "devB"}, DeviceID: "devB"},
	}

	if err := crsqlite.InsertOps(ctx, h.DB(), testApp, testUser, testDataset, ops); err != nil {
		t.Fatalf("InsertOps: %v", err)
	}

	// Query since zero — should return all 3.
	got, err := crsqlite.QueryOps(ctx, h.DB(), testApp, testUser, testDataset, core.HLC{})
	if err != nil {
		t.Fatalf("QueryOps: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 ops, got %d", len(got))
	}

	// Query since ts_wall=1000 — should return ops with ts_wall > 1000.
	got2, err := crsqlite.QueryOps(ctx, h.DB(), testApp, testUser, testDataset,
		core.HLC{WallTime: 1000})
	if err != nil {
		t.Fatalf("QueryOps since 1000: %v", err)
	}
	if len(got2) != 2 {
		t.Fatalf("expected 2 ops after ts_wall=1000, got %d", len(got2))
	}
}

func TestInsertOpsIdempotent(t *testing.T) {
	h := setupDataset(t)
	ctx := context.Background()

	op := core.CRDTOp{
		OpID: "op-dup", DocID: "doc1", Field: "x", Value: 42.0,
		Timestamp: core.HLC{WallTime: 500, Logical: 0, DeviceID: "devA"}, DeviceID: "devA",
	}

	if err := crsqlite.InsertOps(ctx, h.DB(), testApp, testUser, testDataset, []core.CRDTOp{op}); err != nil {
		t.Fatalf("InsertOps (1st): %v", err)
	}
	// Same op again — must be idempotent.
	if err := crsqlite.InsertOps(ctx, h.DB(), testApp, testUser, testDataset, []core.CRDTOp{op}); err != nil {
		t.Fatalf("InsertOps (2nd duplicate): %v", err)
	}

	count, err := crsqlite.CountOps(ctx, h.DB(), testApp, testUser, testDataset)
	if err != nil {
		t.Fatalf("CountOps: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 op after duplicate insert, got %d", count)
	}
}

func TestDeleteOps(t *testing.T) {
	h := setupDataset(t)
	ctx := context.Background()

	ops := []core.CRDTOp{
		{OpID: "op1", DocID: "doc1", Field: "x", Value: 1.0,
			Timestamp: core.HLC{WallTime: 1, Logical: 0, DeviceID: "devA"}, DeviceID: "devA"},
	}
	if err := crsqlite.InsertOps(ctx, h.DB(), testApp, testUser, testDataset, ops); err != nil {
		t.Fatalf("InsertOps: %v", err)
	}
	if err := crsqlite.DeleteOps(ctx, h.DB(), testApp, testUser, testDataset); err != nil {
		t.Fatalf("DeleteOps: %v", err)
	}
	count, err := crsqlite.CountOps(ctx, h.DB(), testApp, testUser, testDataset)
	if err != nil {
		t.Fatalf("CountOps: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 ops after delete, got %d", count)
	}
}
