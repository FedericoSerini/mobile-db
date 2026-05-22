package sync_test

import (
	"context"
	"testing"

	"github.com/federicoserini/mobile-db/core"
	"github.com/federicoserini/mobile-db/core/crdt"
	coresync "github.com/federicoserini/mobile-db/core/sync"
)

type captureStorage struct {
	core.NopStorage
	merged    []core.CRDTOp
	opCount   int
	compacted bool
}

func (s *captureStorage) MergeOps(_ context.Context, _, _, _ string, ops []core.CRDTOp) error {
	s.merged = append(s.merged, ops...)
	s.opCount += len(ops)
	return nil
}

func (s *captureStorage) GetDelta(_ context.Context, _, _, _ string, _ core.HLC) ([]core.CRDTOp, core.HLC, error) {
	return nil, core.HLC{WallTime: 999}, nil
}

func (s *captureStorage) OpCount(_ context.Context, _, _, _ string) (int, error) { return s.opCount, nil }

func (s *captureStorage) Compact(_ context.Context, _, _, _ string) error {
	s.compacted = true
	return nil
}

type captureBroadcaster struct{ notified bool }

func (b *captureBroadcaster) Notify(_ context.Context, _, _ string) error {
	b.notified = true
	return nil
}

func TestEngineSyncMergesAndBroadcasts(t *testing.T) {
	store := &captureStorage{}
	bc := &captureBroadcaster{}
	clock := crdt.NewClock("server")
	eng := coresync.NewEngine(store, bc, clock, 1000)

	msg := core.SyncMessage{
		AppID: "app1", UserID: "user1", DatasetID: "ds1",
		Ops: []core.CRDTOp{
			{OpID: "op1", DocID: "doc1", Field: "x", Value: "v",
				Timestamp: core.HLC{WallTime: 1, DeviceID: "dev1"}},
		},
	}

	resp, err := eng.Sync(context.Background(), msg)
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if len(store.merged) != 1 {
		t.Fatal("engine must call MergeOps")
	}
	if !bc.notified {
		t.Fatal("engine must broadcast after merge")
	}
	if resp.NewClock.WallTime == 0 {
		t.Fatal("response must include non-zero clock")
	}
}

func TestEngineSyncEmptyOps(t *testing.T) {
	store := &captureStorage{}
	bc := &captureBroadcaster{}
	eng := coresync.NewEngine(store, bc, crdt.NewClock("server"), 1000)
	msg := core.SyncMessage{AppID: "app1", UserID: "user1", DatasetID: "ds1"}
	resp, err := eng.Sync(context.Background(), msg)
	if err != nil {
		t.Fatalf("Sync empty: %v", err)
	}
	if bc.notified {
		t.Fatal("must not broadcast when no ops")
	}
	_ = resp
}
