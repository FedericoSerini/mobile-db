package crsqlite

import (
	"context"
	"database/sql"
	"time"

	"github.com/federicoserini/mobile-db/server/auth"
)

// AuthStore implements auth.AuthStore on top of MetaDB.
type AuthStore struct{ db *MetaDB }

func NewAuthStore(db *MetaDB) *AuthStore { return &AuthStore{db: db} }

func (s *AuthStore) CreateDeviceKey(ctx context.Context, dk auth.DeviceKey) error {
	_, err := s.db.DB().ExecContext(ctx,
		`INSERT INTO device_keys (device_key_id, device_id, app_id, key_hash, status, registered_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		dk.DeviceKeyID, dk.DeviceID, dk.AppID, dk.KeyHash, dk.Status, dk.RegisteredAt.UTC())
	return err
}

func (s *AuthStore) GetDeviceKey(ctx context.Context, deviceKeyID string) (*auth.DeviceKey, error) {
	var dk auth.DeviceKey
	var lastSeen sql.NullTime
	err := s.db.DB().QueryRowContext(ctx,
		`SELECT device_key_id, device_id, app_id, key_hash, status, registered_at, last_seen_at
		 FROM device_keys WHERE device_key_id = ?`, deviceKeyID).
		Scan(&dk.DeviceKeyID, &dk.DeviceID, &dk.AppID, &dk.KeyHash, &dk.Status, &dk.RegisteredAt, &lastSeen)
	if err == sql.ErrNoRows {
		return nil, auth.ErrDeviceNotFound
	}
	if err != nil {
		return nil, err
	}
	if lastSeen.Valid {
		t := lastSeen.Time
		dk.LastSeenAt = &t
	}
	return &dk, nil
}

func (s *AuthStore) UpdateDeviceKey(ctx context.Context, dk auth.DeviceKey) error {
	_, err := s.db.DB().ExecContext(ctx,
		`UPDATE device_keys SET key_hash=?, status=?, last_seen_at=? WHERE device_key_id=?`,
		dk.KeyHash, dk.Status, time.Now().UTC(), dk.DeviceKeyID)
	return err
}
