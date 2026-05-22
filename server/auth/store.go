package auth

import (
	"context"
	"errors"
	"time"
)

var (
	ErrDeviceNotFound = errors.New("device not found")
	ErrDeviceRevoked  = errors.New("device revoked")
	ErrInvalidSecret  = errors.New("invalid device secret")
)

type DeviceKey struct {
	DeviceKeyID  string
	DeviceID     string
	AppID        string
	KeyHash      string
	Status       string
	RegisteredAt time.Time
	LastSeenAt   *time.Time
}

type AuthStore interface {
	CreateDeviceKey(ctx context.Context, dk DeviceKey) error
	GetDeviceKey(ctx context.Context, deviceKeyID string) (*DeviceKey, error)
	UpdateDeviceKey(ctx context.Context, dk DeviceKey) error
}
