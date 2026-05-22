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
// Idempotent — safe to call on every open.
func EnsureDataset(ctx context.Context, db *sql.DB, appID, userID, datasetID string) error {
	tbl := datasetTable(appID, userID, datasetID)
	ddl := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s (
    op_id     TEXT NOT NULL PRIMARY KEY,
    doc_id    TEXT,
    field     TEXT,
    value     BLOB,
    ts_wall   INTEGER DEFAULT 0,
    ts_logic  INTEGER DEFAULT 0,
    device_id TEXT    DEFAULT ''
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

// QueryOps returns all ops in the table with ts_wall > since.WallTime,
// or ts_wall == since.WallTime && ts_logic > since.Logical.
func QueryOps(ctx context.Context, db *sql.DB, appID, userID, datasetID string, since core.HLC) ([]core.CRDTOp, error) {
	tbl := datasetTable(appID, userID, datasetID)
	query := fmt.Sprintf(`
SELECT op_id, doc_id, field, value, ts_wall, ts_logic, device_id
FROM %s
WHERE ts_wall > ?
   OR (ts_wall = ? AND ts_logic > ?)
ORDER BY ts_wall ASC, ts_logic ASC, device_id ASC`, tbl)

	rows, err := db.QueryContext(ctx, query, since.WallTime, since.WallTime, since.Logical)
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
		if err := json.Unmarshal([]byte(valueJSON), &op.Value); err != nil {
			return nil, fmt.Errorf("unmarshal value for op %s: %w", op.OpID, err)
		}
		ops = append(ops, op)
	}
	return ops, rows.Err()
}

// CountOps returns the total number of ops stored for the dataset.
func CountOps(ctx context.Context, db *sql.DB, appID, userID, datasetID string) (int, error) {
	tbl := datasetTable(appID, userID, datasetID)
	var count int
	err := db.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", tbl)).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count ops: %w", err)
	}
	return count, nil
}

// DeleteOps removes all ops for the dataset (used during compaction).
func DeleteOps(ctx context.Context, db *sql.DB, appID, userID, datasetID string) error {
	tbl := datasetTable(appID, userID, datasetID)
	if _, err := db.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s", tbl)); err != nil {
		return fmt.Errorf("delete ops: %w", err)
	}
	return nil
}
