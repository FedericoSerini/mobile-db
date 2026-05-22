package crsqlite

import (
	"context"
	"database/sql"
	"time"

	"github.com/federicoserini/mobile-db/server/auth"
)

// AuthStore implements auth.AuthStore and auth.RefreshStore on top of MetaDB.
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

func (s *AuthStore) CreateRefreshToken(ctx context.Context, rt auth.RefreshToken) error {
	_, err := s.db.DB().ExecContext(ctx,
		`INSERT INTO refresh_tokens (token_hash, app_id, user_id, expires_at, used) VALUES (?,?,?,?,0)`,
		rt.TokenHash, rt.AppID, rt.UserID, rt.ExpiresAt.UTC())
	return err
}

func (s *AuthStore) GetAndInvalidateRefreshToken(ctx context.Context, hash string) (*auth.RefreshToken, error) {
	tx, err := s.db.DB().BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var rt auth.RefreshToken
	var used int
	err = tx.QueryRowContext(ctx,
		`SELECT token_hash, app_id, user_id, expires_at, used FROM refresh_tokens WHERE token_hash=?`,
		hash).Scan(&rt.TokenHash, &rt.AppID, &rt.UserID, &rt.ExpiresAt, &used)
	if err == sql.ErrNoRows || used == 1 {
		return nil, auth.ErrRefreshTokenInvalid
	}
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE refresh_tokens SET used=1 WHERE token_hash=?`, hash); err != nil {
		return nil, err
	}
	rt.Used = used == 1
	return &rt, tx.Commit()
}

func (s *AuthStore) RevokeAllForUser(ctx context.Context, appID, userID string) error {
	_, err := s.db.DB().ExecContext(ctx,
		`UPDATE refresh_tokens SET used=1 WHERE app_id=? AND user_id=?`, appID, userID)
	return err
}
