package crdt_test

import (
	"context"
	"testing"

	"github.com/federicoserini/mobile-db/core"
	"github.com/federicoserini/mobile-db/core/crdt"
)

type stubStorage struct {
	core.NopStorage
	opCount   int
	compacted bool
}

func (s *stubStorage) OpCount(_ context.Context, _, _, _ string) (int, error) {
	return s.opCount, nil
}

func (s *stubStorage) Compact(_ context.Context, _, _, _ string) error {
	s.compacted = true
	return nil
}

func TestMaybeCompactAboveThreshold(t *testing.T) {
	store := &stubStorage{opCount: 1001}
	err := crdt.MaybeCompact(context.Background(), store, "app1", "user1", "ds1", 1000)
	if err != nil {
		t.Fatalf("MaybeCompact: %v", err)
	}
	if !store.compacted {
		t.Fatal("Compact must be called when op count > threshold")
	}
}

func TestMaybeCompactBelowThreshold(t *testing.T) {
	store := &stubStorage{opCount: 999}
	_ = crdt.MaybeCompact(context.Background(), store, "app1", "user1", "ds1", 1000)
	if store.compacted {
		t.Fatal("Compact must NOT be called when op count <= threshold")
	}
}

func TestMaybeCompactAtThreshold(t *testing.T) {
	store := &stubStorage{opCount: 1000}
	_ = crdt.MaybeCompact(context.Background(), store, "app1", "user1", "ds1", 1000)
	if store.compacted {
		t.Fatal("Compact must NOT be called at exactly threshold (only above)")
	}
}
