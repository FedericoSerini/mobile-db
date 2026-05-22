package sync

import (
	"context"

	"github.com/federicoserini/mobile-db/core"
	"github.com/federicoserini/mobile-db/core/crdt"
)

type Notifier interface {
	Notify(ctx context.Context, appID, userID string) error
}

type Engine struct {
	store     core.StorageBackend
	notifier  Notifier
	clock     *crdt.Clock
	threshold int
}

func NewEngine(store core.StorageBackend, notifier Notifier, clock *crdt.Clock, compactionThreshold int) *Engine {
	return &Engine{store: store, notifier: notifier, clock: clock, threshold: compactionThreshold}
}

func (e *Engine) Sync(ctx context.Context, msg core.SyncMessage) (*core.SyncResponse, error) {
	serverClock := e.clock.Receive(msg.Clock)

	needsSnap, err := crdt.NeedsSnapshot(ctx, e.store, msg.AppID, msg.DatasetID, msg.ClientSchemaVersion)
	if err != nil {
		return nil, err
	}

	var snapshot any
	if needsSnap {
		snap, _, err := e.store.GetSnapshot(ctx, msg.AppID, msg.UserID, msg.DatasetID)
		if err != nil {
			return nil, err
		}
		snapshot = snap
	}

	if len(msg.Ops) > 0 {
		if err := e.store.MergeOps(ctx, msg.AppID, msg.UserID, msg.DatasetID, msg.Ops); err != nil {
			return nil, err
		}
		_ = e.notifier.Notify(ctx, msg.AppID, msg.UserID)
		_ = crdt.MaybeCompact(ctx, e.store, msg.AppID, msg.UserID, msg.DatasetID, e.threshold)
	}

	delta, newClock, err := e.store.GetDelta(ctx, msg.AppID, msg.UserID, msg.DatasetID, msg.Clock)
	if err != nil {
		return nil, err
	}
	if newClock.WallTime == 0 {
		newClock = serverClock
	}

	schemaVer, _ := e.store.SchemaVersion(ctx, msg.AppID, msg.DatasetID)

	return &core.SyncResponse{
		NewClock:      newClock,
		Ops:           delta,
		Snapshot:      snapshot,
		SchemaVersion: schemaVer,
	}, nil
}
