package crsqlite

import (
	"context"
	"database/sql"
	"time"

	"github.com/federicoserini/mobile-db/admin"
	"github.com/federicoserini/mobile-db/server/auth"
)

// AdminDeviceStore implements admin.DeviceAdminStore on top of a raw *sql.DB.
type AdminDeviceStore struct {
	db *sql.DB
}

func NewAdminDeviceStore(meta *MetaDB) *AdminDeviceStore {
	return &AdminDeviceStore{db: meta.DB()}
}

// NewAdminDeviceStoreFromDB wraps a raw *sql.DB — used in tests.
func NewAdminDeviceStoreFromDB(db *sql.DB) *AdminDeviceStore {
	return &AdminDeviceStore{db: db}
}

// ListDevices returns device keys, optionally filtered by status ("active", "revoked", or "" for all).
func (s *AdminDeviceStore) ListDevices(ctx context.Context, status string) ([]auth.DeviceKey, error) {
	q := `SELECT device_key_id, device_id, app_id, key_hash, status, registered_at
	      FROM device_keys
	      WHERE (?='' OR status=?)
	      ORDER BY registered_at DESC`
	rows, err := s.db.QueryContext(ctx, q, status, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var devices []auth.DeviceKey
	for rows.Next() {
		var dk auth.DeviceKey
		rows.Scan(&dk.DeviceKeyID, &dk.DeviceID, &dk.AppID, &dk.KeyHash, &dk.Status, &dk.RegisteredAt)
		devices = append(devices, dk)
	}
	return devices, rows.Err()
}

// DeviceCounts returns total, active, and revoked device counts in one query.
func (s *AdminDeviceStore) DeviceCounts(ctx context.Context) (all, active, revoked int, err error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT status, COUNT(*) FROM device_keys GROUP BY status`)
	if err != nil {
		return 0, 0, 0, err
	}
	defer rows.Close()
	for rows.Next() {
		var status string
		var count int
		rows.Scan(&status, &count)
		all += count
		switch status {
		case "active":
			active = count
		case "revoked":
			revoked = count
		}
	}
	return all, active, revoked, rows.Err()
}

func (s *AdminDeviceStore) RevokeDevice(ctx context.Context, deviceKeyID string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE device_keys SET status='revoked' WHERE device_key_id=?`, deviceKeyID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return auth.ErrDeviceNotFound
	}
	return nil
}

// AdminDataStore implements admin.DataAdminStore on top of Backend.
// Since Backend uses a single SQLite file containing both metadata and op_log
// tables, datasets and docs are queried from the same connection.
type AdminDataStore struct{ b *Backend }

func NewAdminDataStore(b *Backend) *AdminDataStore { return &AdminDataStore{b: b} }

func (s *AdminDataStore) ListDatasets(ctx context.Context, appID string) ([]string, error) {
	rows, err := s.b.DB().QueryContext(ctx,
		`SELECT dataset_id FROM datasets WHERE app_id=?`, appID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var datasets []string
	for rows.Next() {
		var id string
		rows.Scan(&id)
		datasets = append(datasets, id)
	}
	return datasets, rows.Err()
}

func (s *AdminDataStore) ListDocs(ctx context.Context, appID, datasetID string) ([]map[string]any, error) {
	rows, err := s.b.DB().QueryContext(ctx,
		`SELECT doc_id, data FROM snapshots WHERE dataset_id=?`, datasetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var docs []map[string]any
	for rows.Next() {
		var docID, data string
		rows.Scan(&docID, &data)
		docs = append(docs, map[string]any{"doc_id": docID, "data": data})
	}
	return docs, rows.Err()
}

func (s *AdminDataStore) DeleteDoc(ctx context.Context, appID, datasetID, docID string) error {
	_, err := s.b.DB().ExecContext(ctx,
		`DELETE FROM snapshots WHERE dataset_id=? AND doc_id=?`, datasetID, docID)
	return err
}

// AdminSyncStore implements admin.SyncAdminStore on top of Backend.
type AdminSyncStore struct{ b *Backend }

func NewAdminSyncStore(b *Backend) *AdminSyncStore { return &AdminSyncStore{b: b} }

func (s *AdminSyncStore) ListSyncActivity(ctx context.Context) ([]admin.SyncEntry, error) {
	rows, err := s.b.DB().QueryContext(ctx,
		`SELECT app_id, dataset_id FROM datasets ORDER BY app_id, dataset_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entries []admin.SyncEntry
	for rows.Next() {
		var appID, datasetID string
		rows.Scan(&appID, &datasetID)
		// Use empty userID to get aggregate count across all users for this dataset.
		count, _ := s.b.OpCount(ctx, appID, "", datasetID)
		entries = append(entries, admin.SyncEntry{
			AppID:     appID,
			DatasetID: datasetID,
			OpCount:   count,
			LastSync:  time.Now(),
		})
	}
	return entries, rows.Err()
}

// AdminEventStore implements handlers.SyncEventWriter on top of a raw *sql.DB
// (the meta database).
type AdminEventStore struct{ db *sql.DB }

// NewAdminEventStore wraps a MetaDB for use in production wiring.
func NewAdminEventStore(meta *MetaDB) *AdminEventStore {
	return &AdminEventStore{db: meta.DB()}
}

// NewAdminEventStoreFromDB wraps a raw *sql.DB — used in tests.
func NewAdminEventStoreFromDB(db *sql.DB) *AdminEventStore {
	return &AdminEventStore{db: db}
}

func (s *AdminEventStore) WriteSyncEvent(ctx context.Context, appID, datasetID, userID, deviceKeyID string, opCount int) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO sync_events (app_id, dataset_id, user_id, device_key_id, op_count)
		 VALUES (?,?,?,?,?)`,
		appID, datasetID, userID, deviceKeyID, opCount)
	return err
}
