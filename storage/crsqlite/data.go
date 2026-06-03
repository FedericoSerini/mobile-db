package crsqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/federicoserini/mobile-db/core"
)

// datasetTable returns the per-dataset op_log table name.
func datasetTable(appID, userID, datasetID string) string {
	return fmt.Sprintf("op_log_%s_%s_%s", sanitize(appID), sanitize(userID), sanitize(datasetID))
}

// sanitize replaces non-alphanumeric characters with underscores to make a
// string safe for use in a SQLite table name.
func sanitize(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			b[i] = c
		} else {
			b[i] = '_'
		}
	}
	return string(b)
}

// EnsureDataset creates the per-dataset op_log table and marks it as a CRR.
// Also ensures the shared snapshots table exists.
// Idempotent — safe to call on every open.
func EnsureDataset(ctx context.Context, db *sql.DB, appID, userID, datasetID string) error {
	// Ensure the shared snapshots table exists.
	const snapshotsDDL = `
CREATE TABLE IF NOT EXISTS snapshots (
    dataset_id  TEXT NOT NULL,
    user_id     TEXT NOT NULL,
    doc_id      TEXT NOT NULL,
    data        TEXT NOT NULL,
    wall_time   INTEGER NOT NULL,
    logical     INTEGER NOT NULL,
    device_id   TEXT NOT NULL,
    created_at  DATETIME NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (dataset_id, user_id, doc_id)
);`
	if _, err := db.ExecContext(ctx, snapshotsDDL); err != nil {
		return fmt.Errorf("create snapshots table: %w", err)
	}

	tbl := datasetTable(appID, userID, datasetID)
	ddl := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s (
    op_id     TEXT NOT NULL PRIMARY KEY,
    doc_id    TEXT,
    field     TEXT,
    value     BLOB,
    ts_wall   INTEGER DEFAULT 0,
    ts_logic  INTEGER DEFAULT 0,
    device_id TEXT    DEFAULT '',
    status    TEXT    NOT NULL DEFAULT 'active'
);`, tbl)

	if _, err := db.ExecContext(ctx, ddl); err != nil {
		return fmt.Errorf("create op_log table %s: %w", tbl, err)
	}

	// Check if already a CRR before calling crsql_as_crr again.
	// cr-sqlite creates a shadow clock table named <tbl>__crsql_clock.
	shadowTbl := tbl + "__crsql_clock"
	var count int
	err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, shadowTbl,
	).Scan(&count)
	if err != nil {
		return fmt.Errorf("check crsql shadow table: %w", err)
	}
	if count > 0 {
		return nil // already a CRR
	}

	var result string
	if err := db.QueryRowContext(ctx,
		fmt.Sprintf("SELECT crsql_as_crr('%s')", tbl),
	).Scan(&result); err != nil {
		return fmt.Errorf("crsql_as_crr %s: %w", tbl, err)
	}
	return nil
}

// InsertOps inserts ops into the per-dataset op_log table, ignoring duplicates.
func InsertOps(ctx context.Context, db *sql.DB, appID, userID, datasetID string, ops []core.CRDTOp) error {
	tbl := datasetTable(appID, userID, datasetID)
	stmt := fmt.Sprintf(
		`INSERT INTO %s (op_id, doc_id, field, value, ts_wall, ts_logic, device_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(op_id) DO NOTHING`, tbl)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	prepared, err := tx.PrepareContext(ctx, stmt)
	if err != nil {
		return fmt.Errorf("prepare insert: %w", err)
	}
	defer prepared.Close()

	for _, op := range ops {
		valueJSON, err := json.Marshal(op.Value)
		if err != nil {
			return fmt.Errorf("marshal value for op %s: %w", op.OpID, err)
		}
		if _, err := prepared.ExecContext(ctx,
			op.OpID, op.DocID, op.Field,
			string(valueJSON),
			op.Timestamp.WallTime, op.Timestamp.Logical,
			op.DeviceID,
		); err != nil {
			return fmt.Errorf("insert op %s: %w", op.OpID, err)
		}
	}
	return tx.Commit()
}

// QueryOps returns all ops strictly after `since` in HLC total order.
// When since.DeviceID is non-empty the full three-part tie-breaker is used;
// otherwise only wall_time and logical are compared (device_id is ignored so
// that callers using a partial HLC cursor get the expected results).
func QueryOps(ctx context.Context, db *sql.DB, appID, userID, datasetID string, since core.HLC) ([]core.CRDTOp, error) {
	tbl := datasetTable(appID, userID, datasetID)

	var query string
	var args []any
	if since.DeviceID != "" {
		query = fmt.Sprintf(`
SELECT op_id, doc_id, field, value, ts_wall, ts_logic, device_id
FROM %s
WHERE status = 'active'
  AND (ts_wall > ?
   OR (ts_wall = ? AND ts_logic > ?)
   OR (ts_wall = ? AND ts_logic = ? AND device_id > ?))
ORDER BY ts_wall ASC, ts_logic ASC, device_id ASC`, tbl)
		args = []any{
			since.WallTime,
			since.WallTime, since.Logical,
			since.WallTime, since.Logical, since.DeviceID,
		}
	} else {
		query = fmt.Sprintf(`
SELECT op_id, doc_id, field, value, ts_wall, ts_logic, device_id
FROM %s
WHERE status = 'active'
  AND (ts_wall > ?
   OR (ts_wall = ? AND ts_logic > ?))
ORDER BY ts_wall ASC, ts_logic ASC, device_id ASC`, tbl)
		args = []any{
			since.WallTime,
			since.WallTime, since.Logical,
		}
	}

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query ops: %w", err)
	}
	defer rows.Close()

	var ops []core.CRDTOp
	for rows.Next() {
		var op core.CRDTOp
		var valueJSON string
		if err := rows.Scan(
			&op.OpID, &op.DocID, &op.Field, &valueJSON,
			&op.Timestamp.WallTime, &op.Timestamp.Logical,
			&op.DeviceID,
		); err != nil {
			return nil, fmt.Errorf("scan op: %w", err)
		}
		op.Timestamp.DeviceID = op.DeviceID
		if err := json.Unmarshal([]byte(valueJSON), &op.Value); err != nil {
			return nil, fmt.Errorf("unmarshal value for op %s: %w", op.OpID, err)
		}
		ops = append(ops, op)
	}
	return ops, rows.Err()
}

// CountOps returns the number of active ops stored for the dataset.
func CountOps(ctx context.Context, db *sql.DB, appID, userID, datasetID string) (int, error) {
	tbl := datasetTable(appID, userID, datasetID)
	var count int
	err := db.QueryRowContext(ctx,
		fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE status = 'active'", tbl),
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count ops: %w", err)
	}
	return count, nil
}

// DeleteOps removes all ops for the dataset.
func DeleteOps(ctx context.Context, db *sql.DB, appID, userID, datasetID string) error {
	tbl := datasetTable(appID, userID, datasetID)
	if _, err := db.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s", tbl)); err != nil {
		return fmt.Errorf("delete ops: %w", err)
	}
	return nil
}

// ArchiveOps sets status='archived' on all active ops for the dataset.
func ArchiveOps(ctx context.Context, db *sql.DB, appID, userID, datasetID string) error {
	tbl := datasetTable(appID, userID, datasetID)
	if _, err := db.ExecContext(ctx,
		fmt.Sprintf("UPDATE %s SET status = 'archived' WHERE status = 'active'", tbl),
	); err != nil {
		return fmt.Errorf("archive ops: %w", err)
	}
	return nil
}

// ReactivateOps re-inserts ops as active rows; if an op_id already exists
// (e.g. archived), it sets status back to 'active'. Used by Compact to
// restore winning ops after archiving.
func ReactivateOps(ctx context.Context, db *sql.DB, appID, userID, datasetID string, ops []core.CRDTOp) error {
	tbl := datasetTable(appID, userID, datasetID)
	stmt := fmt.Sprintf(
		`INSERT INTO %s (op_id, doc_id, field, value, ts_wall, ts_logic, device_id, status)
		 VALUES (?, ?, ?, ?, ?, ?, ?, 'active')
		 ON CONFLICT(op_id) DO UPDATE SET status = 'active'`, tbl)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	prepared, err := tx.PrepareContext(ctx, stmt)
	if err != nil {
		return fmt.Errorf("prepare reactivate: %w", err)
	}
	defer prepared.Close()

	for _, op := range ops {
		valueJSON, err := json.Marshal(op.Value)
		if err != nil {
			return fmt.Errorf("marshal value for op %s: %w", op.OpID, err)
		}
		if _, err := prepared.ExecContext(ctx,
			op.OpID, op.DocID, op.Field,
			string(valueJSON),
			op.Timestamp.WallTime, op.Timestamp.Logical,
			op.DeviceID,
		); err != nil {
			return fmt.Errorf("reactivate op %s: %w", op.OpID, err)
		}
	}
	return tx.Commit()
}

// WriteSnapshot upserts the merged snapshot for (datasetID, userID, docID).
func WriteSnapshot(ctx context.Context, db *sql.DB, datasetID, userID, docID, data string, wallTime int64, logical uint32, deviceID string) error {
	_, err := db.ExecContext(ctx,
		`INSERT OR REPLACE INTO snapshots (dataset_id, user_id, doc_id, data, wall_time, logical, device_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		datasetID, userID, docID, data, wallTime, logical, deviceID,
	)
	if err != nil {
		return fmt.Errorf("write snapshot: %w", err)
	}
	return nil
}
