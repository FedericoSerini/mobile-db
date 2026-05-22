package crdt

import (
	"context"

	"github.com/federicoserini/mobile-db/core"
)

// MaybeCompact calls store.Compact when op count exceeds threshold.
// Call after every successful MergeOps to keep the log bounded.
func MaybeCompact(ctx context.Context, store core.StorageBackend, appID, userID, datasetID string, threshold int) error {
	count, err := store.OpCount(ctx, appID, userID, datasetID)
	if err != nil {
		return err
	}
	if count > threshold {
		return store.Compact(ctx, appID, userID, datasetID)
	}
	return nil
}
