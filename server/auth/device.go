package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const bcryptCost = 12

type RegistrationResult struct {
	DeviceKeyID  string
	DeviceSecret string
}

type DeviceService struct {
	store AuthStore
}

func NewDeviceService(store AuthStore) *DeviceService {
	return &DeviceService{store: store}
}

func (s *DeviceService) Register(ctx context.Context, appID, deviceID string) (*RegistrationResult, error) {
	secret, err := randomHex(32)
	if err != nil {
		return nil, fmt.Errorf("generate secret: %w", err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(secret), bcryptCost)
	if err != nil {
		return nil, fmt.Errorf("hash secret: %w", err)
	}
	dk := DeviceKey{
		DeviceKeyID:  uuid.NewString(),
		DeviceID:     deviceID,
		AppID:        appID,
		KeyHash:      string(hash),
		Status:       "active",
		RegisteredAt: time.Now().UTC(),
	}
	if err := s.store.CreateDeviceKey(ctx, dk); err != nil {
		return nil, err
	}
	return &RegistrationResult{DeviceKeyID: dk.DeviceKeyID, DeviceSecret: secret}, nil
}

func (s *DeviceService) Verify(ctx context.Context, deviceKeyID, secret string) error {
	dk, err := s.store.GetDeviceKey(ctx, deviceKeyID)
	if err != nil {
		return ErrDeviceNotFound
	}
	if dk.Status != "active" {
		return ErrDeviceRevoked
	}
	if err := bcrypt.CompareHashAndPassword([]byte(dk.KeyHash), []byte(secret)); err != nil {
		return ErrInvalidSecret
	}
	return nil
}

func (s *DeviceService) RotateKey(ctx context.Context, deviceKeyID, oldSecret, newSecret string) error {
	dk, err := s.store.GetDeviceKey(ctx, deviceKeyID)
	if err != nil {
		return ErrDeviceNotFound
	}
	if dk.Status != "active" {
		return ErrDeviceRevoked
	}
	if err := bcrypt.CompareHashAndPassword([]byte(dk.KeyHash), []byte(oldSecret)); err != nil {
		return ErrInvalidSecret
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newSecret), bcryptCost)
	if err != nil {
		return err
	}
	dk.KeyHash = string(hash)
	return s.store.UpdateDeviceKey(ctx, *dk)
}

func (s *DeviceService) Revoke(ctx context.Context, deviceKeyID string) error {
	dk, err := s.store.GetDeviceKey(ctx, deviceKeyID)
	if err != nil {
		return ErrDeviceNotFound
	}
	dk.Status = "revoked"
	return s.store.UpdateDeviceKey(ctx, *dk)
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
