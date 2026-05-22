package crsqlite

import (
	"context"
	"database/sql"
	"fmt"
)

const metaSchema = `
CREATE TABLE IF NOT EXISTS mdb_meta (
    app_id         TEXT NOT NULL,
    dataset_id     TEXT NOT NULL,
    schema_version INTEGER NOT NULL DEFAULT 1,
    created_at     INTEGER NOT NULL DEFAULT (unixepoch()),
    PRIMARY KEY (app_id, dataset_id)
);
`

// EnsureMeta creates the metadata table if it does not exist.
func EnsureMeta(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, metaSchema); err != nil {
		return fmt.Errorf("ensure meta table: %w", err)
	}
	return nil
}

// GetSchemaVersion returns the schema version for (appID, datasetID).
// Returns 1 if no record exists yet.
func GetSchemaVersion(ctx context.Context, db *sql.DB, appID, datasetID string) (int, error) {
	var version int
	err := db.QueryRowContext(ctx,
		`SELECT schema_version FROM mdb_meta WHERE app_id = ? AND dataset_id = ?`,
		appID, datasetID,
	).Scan(&version)
	if err == sql.ErrNoRows {
		return 1, nil
	}
	if err != nil {
		return 0, fmt.Errorf("get schema version: %w", err)
	}
	return version, nil
}

// SetSchemaVersion upserts (appID, datasetID) with the given schema version.
func SetSchemaVersion(ctx context.Context, db *sql.DB, appID, datasetID string, version int) error {
	_, err := db.ExecContext(ctx,
		`INSERT INTO mdb_meta (app_id, dataset_id, schema_version)
		 VALUES (?, ?, ?)
		 ON CONFLICT (app_id, dataset_id) DO UPDATE SET schema_version = excluded.schema_version`,
		appID, datasetID, version,
	)
	if err != nil {
		return fmt.Errorf("set schema version: %w", err)
	}
	return nil
}
