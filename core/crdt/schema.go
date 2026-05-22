package crdt

import (
	"context"

	"github.com/federicoserini/mobile-db/core"
)

// NeedsSnapshot returns true when the server schema version is ahead of the client's.
// Client must discard incremental ops and download a full snapshot in that case.
func NeedsSnapshot(ctx context.Context, store core.StorageBackend, appID, datasetID string, clientVersion int) (bool, error) {
	serverVersion, err := store.SchemaVersion(ctx, appID, datasetID)
	if err != nil {
		return false, err
	}
	return serverVersion > clientVersion, nil
}
