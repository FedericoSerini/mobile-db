package crsqlite

import (
	"context"
	"database/sql"
	"fmt"
)

const metaSchema = `
CREATE TABLE IF NOT EXISTS device_keys (
    device_key_id  TEXT PRIMARY KEY,
    device_id      TEXT NOT NULL,
    app_id         TEXT NOT NULL,
    key_hash       TEXT NOT NULL,
    status         TEXT NOT NULL DEFAULT 'active',
    registered_at  DATETIME NOT NULL DEFAULT (datetime('now')),
    last_seen_at   DATETIME
);

CREATE TABLE IF NOT EXISTS datasets (
    app_id         TEXT NOT NULL,
    dataset_id     TEXT NOT NULL,
    acl            TEXT NOT NULL DEFAULT 'private',
    schema_version INTEGER NOT NULL DEFAULT 1,
    PRIMARY KEY (app_id, dataset_id)
);

CREATE TABLE IF NOT EXISTS refresh_tokens (
    token_hash TEXT PRIMARY KEY,
    app_id     TEXT NOT NULL,
    user_id    TEXT NOT NULL,
    expires_at DATETIME NOT NULL,
    used       INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_device_keys_app ON device_keys(app_id);
CREATE INDEX IF NOT EXISTS idx_device_keys_device ON device_keys(device_id);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user ON refresh_tokens(app_id, user_id);
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
		`SELECT schema_version FROM datasets WHERE app_id = ? AND dataset_id = ?`,
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
		`INSERT INTO datasets (app_id, dataset_id, schema_version)
		 VALUES (?, ?, ?)
		 ON CONFLICT (app_id, dataset_id) DO UPDATE SET schema_version = excluded.schema_version`,
		appID, datasetID, version,
	)
	if err != nil {
		return fmt.Errorf("set schema version: %w", err)
	}
	return nil
}
