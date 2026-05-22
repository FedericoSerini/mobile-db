package crsqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/federicoserini/mobile-db/core"
)

// Backend implements core.StorageBackend using SQLite with the cr-sqlite extension.
type Backend struct {
	handle *Handle
}

// NewBackend opens the database at path and returns a Backend ready for use.
// extPath is the path to the crsqlite shared library.
// key is reserved for future SQLCipher use; currently unused.
func NewBackend(path, key, extPath string) (*Backend, error) {
	h, err := Open(path, key, extPath)
	if err != nil {
		return nil, fmt.Errorf("new backend: %w", err)
	}
	ctx := context.Background()
	if err := EnsureMeta(ctx, h.DB()); err != nil {
		h.Close()
		return nil, fmt.Errorf("ensure meta: %w", err)
	}
	return &Backend{handle: h}, nil
}

// Close closes the underlying database.
func (b *Backend) Close() error {
	return b.handle.Close()
}

// DB returns the underlying *sql.DB. Used by auth/admin adapters.
func (b *Backend) DB() *sql.DB { return b.handle.DB() }

// Meta returns a MetaDB view of the backend's underlying database.
// It shares the same connection — do NOT call Close on the returned MetaDB.
func (b *Backend) Meta() *MetaDB { return &MetaDB{db: b.handle.DB()} }

// MergeOps persists the incoming ops, deduplicating by op_id.
// Implements core.StorageBackend.
func (b *Backend) MergeOps(ctx context.Context, appID, userID, datasetID string, ops []core.CRDTOp) error {
	if err := EnsureDataset(ctx, b.handle.DB(), appID, userID, datasetID); err != nil {
		return fmt.Errorf("MergeOps ensure dataset: %w", err)
	}
	return InsertOps(ctx, b.handle.DB(), appID, userID, datasetID, ops)
}

// GetDelta returns all ops stored after `since` and the maximum HLC seen.
// Implements core.StorageBackend.
func (b *Backend) GetDelta(ctx context.Context, appID, userID, datasetID string, since core.HLC) ([]core.CRDTOp, core.HLC, error) {
	if err := EnsureDataset(ctx, b.handle.DB(), appID, userID, datasetID); err != nil {
		return nil, core.HLC{}, fmt.Errorf("GetDelta ensure dataset: %w", err)
	}
	ops, err := QueryOps(ctx, b.handle.DB(), appID, userID, datasetID, since)
	if err != nil {
		return nil, core.HLC{}, err
	}

	// Compute the maximum HLC across all returned ops.
	var maxHLC core.HLC
	for _, op := range ops {
		if beforeHLC(maxHLC, op.Timestamp) {
			maxHLC = op.Timestamp
		}
	}
	return ops, maxHLC, nil
}

// GetSnapshot returns the LWW-merged view of all ops for the dataset.
// Implements core.StorageBackend.
func (b *Backend) GetSnapshot(ctx context.Context, appID, userID, datasetID string) (map[string]any, core.HLC, error) {
	if err := EnsureDataset(ctx, b.handle.DB(), appID, userID, datasetID); err != nil {
		return nil, core.HLC{}, fmt.Errorf("GetSnapshot ensure dataset: %w", err)
	}
	ops, err := QueryOps(ctx, b.handle.DB(), appID, userID, datasetID, core.HLC{})
	if err != nil {
		return nil, core.HLC{}, err
	}

	snapshot := mergeOpsLocal(ops)

	// Compute the maximum HLC.
	var maxHLC core.HLC
	for _, op := range ops {
		if beforeHLC(maxHLC, op.Timestamp) {
			maxHLC = op.Timestamp
		}
	}

	// Flatten doc/field nesting into a single map keyed by "docID.field".
	flat := make(map[string]any, len(ops))
	for docID, fields := range snapshot {
		for field, val := range fields {
			flat[docID+"."+field] = val
		}
	}
	return flat, maxHLC, nil
}

// SchemaVersion returns the schema version for (appID, datasetID).
// Implements core.StorageBackend.
func (b *Backend) SchemaVersion(ctx context.Context, appID, datasetID string) (int, error) {
	return GetSchemaVersion(ctx, b.handle.DB(), appID, datasetID)
}

// OpCount returns the number of ops stored for the dataset.
// Implements core.StorageBackend.
func (b *Backend) OpCount(ctx context.Context, appID, userID, datasetID string) (int, error) {
	if err := EnsureDataset(ctx, b.handle.DB(), appID, userID, datasetID); err != nil {
		return 0, fmt.Errorf("OpCount ensure dataset: %w", err)
	}
	return CountOps(ctx, b.handle.DB(), appID, userID, datasetID)
}

// Compact materializes the LWW-merged state into the snapshots table, archives
// all active ops (status='archived'), and re-inserts only the winning op per
// (doc, field) as fresh active rows. After compaction the active op count
// equals the number of unique (doc, field) pairs.
// Implements core.StorageBackend.
func (b *Backend) Compact(ctx context.Context, appID, userID, datasetID string) error {
	if err := EnsureDataset(ctx, b.handle.DB(), appID, userID, datasetID); err != nil {
		return fmt.Errorf("Compact ensure dataset: %w", err)
	}

	ops, err := QueryOps(ctx, b.handle.DB(), appID, userID, datasetID, core.HLC{})
	if err != nil {
		return fmt.Errorf("Compact query: %w", err)
	}

	// LWW merge — determine the winning op per (doc, field).
	type winnerEntry struct {
		op    core.CRDTOp
		found bool
	}
	type docFieldKey struct{ doc, field string }
	winMap := make(map[docFieldKey]winnerEntry)
	for _, op := range ops {
		key := docFieldKey{op.DocID, op.Field}
		cur := winMap[key]
		if !cur.found || beforeHLC(cur.op.Timestamp, op.Timestamp) {
			winMap[key] = winnerEntry{op: op, found: true}
		}
	}

	var kept []core.CRDTOp
	for _, w := range winMap {
		kept = append(kept, w.op)
	}

	// Compute snapshot HLC (max across winners).
	var snapHLC core.HLC
	for _, op := range kept {
		if beforeHLC(snapHLC, op.Timestamp) {
			snapHLC = op.Timestamp
		}
	}

	// Build merged doc map for snapshot storage.
	type docState struct {
		fields  map[string]any
		hlc     core.HLC
		devID   string
	}
	docMap := make(map[string]*docState)
	for _, op := range kept {
		ds, ok := docMap[op.DocID]
		if !ok {
			ds = &docState{fields: make(map[string]any)}
			docMap[op.DocID] = ds
		}
		ds.fields[op.Field] = op.Value
		if beforeHLC(ds.hlc, op.Timestamp) {
			ds.hlc = op.Timestamp
			ds.devID = op.DeviceID
		}
	}

	// Write one snapshot row per doc.
	for docID, ds := range docMap {
		data, err := json.Marshal(ds.fields)
		if err != nil {
			return fmt.Errorf("Compact marshal snapshot for doc %s: %w", docID, err)
		}
		if err := WriteSnapshot(ctx, b.handle.DB(),
			datasetID, userID, docID,
			string(data),
			ds.hlc.WallTime, ds.hlc.Logical, ds.devID,
		); err != nil {
			return fmt.Errorf("Compact write snapshot: %w", err)
		}
	}

	// Archive all currently active ops.
	if err := ArchiveOps(ctx, b.handle.DB(), appID, userID, datasetID); err != nil {
		return fmt.Errorf("Compact archive: %w", err)
	}

	// Reactivate winning ops (sets status='active' on archived rows with matching op_id).
	if len(kept) > 0 {
		if err := ReactivateOps(ctx, b.handle.DB(), appID, userID, datasetID, kept); err != nil {
			return fmt.Errorf("Compact reactivate winners: %w", err)
		}
	}
	return nil
}

// mergeOpsLocal is a local copy of crdt.Merge to avoid import cycles.
// It applies LWW (last-write-wins by HLC total order) over ops and returns
// the winning value per doc and field.
func mergeOpsLocal(ops []core.CRDTOp) map[string]map[string]any {
	type winner struct{ op core.CRDTOp }
	wins := map[string]map[string]winner{}

	for _, op := range ops {
		if _, ok := wins[op.DocID]; !ok {
			wins[op.DocID] = map[string]winner{}
		}
		cur, exists := wins[op.DocID][op.Field]
		if !exists || beforeHLC(cur.op.Timestamp, op.Timestamp) {
			wins[op.DocID][op.Field] = winner{op}
		}
	}

	result := make(map[string]map[string]any, len(wins))
	for docID, fields := range wins {
		result[docID] = make(map[string]any, len(fields))
		for field, w := range fields {
			result[docID][field] = w.op.Value
		}
	}
	return result
}

// beforeHLC returns true if a is strictly before b in HLC total order.
func beforeHLC(a, b core.HLC) bool {
	if a.WallTime != b.WallTime {
		return a.WallTime < b.WallTime
	}
	if a.Logical != b.Logical {
		return a.Logical < b.Logical
	}
	return a.DeviceID < b.DeviceID
}
