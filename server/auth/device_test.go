package auth_test

import (
	"context"
	"testing"

	"github.com/federicoserini/mobile-db/server/auth"
)

type stubAuthStore struct {
	devices map[string]*auth.DeviceKey
}

func newStubStore() *stubAuthStore {
	return &stubAuthStore{devices: map[string]*auth.DeviceKey{}}
}

func (s *stubAuthStore) CreateDeviceKey(_ context.Context, dk auth.DeviceKey) error {
	s.devices[dk.DeviceKeyID] = &dk
	return nil
}

func (s *stubAuthStore) GetDeviceKey(_ context.Context, id string) (*auth.DeviceKey, error) {
	dk, ok := s.devices[id]
	if !ok {
		return nil, auth.ErrDeviceNotFound
	}
	return dk, nil
}

func (s *stubAuthStore) UpdateDeviceKey(_ context.Context, dk auth.DeviceKey) error {
	s.devices[dk.DeviceKeyID] = &dk
	return nil
}

func TestRegisterDevice(t *testing.T) {
	svc := auth.NewDeviceService(newStubStore())
	result, err := svc.Register(context.Background(), "app1", "dev1")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if result.DeviceKeyID == "" || result.DeviceSecret == "" {
		t.Fatal("must return non-empty DeviceKeyID and DeviceSecret")
	}
}

func TestVerifyCorrectSecret(t *testing.T) {
	svc := auth.NewDeviceService(newStubStore())
	reg, _ := svc.Register(context.Background(), "app1", "dev1")
	if err := svc.Verify(context.Background(), reg.DeviceKeyID, reg.DeviceSecret); err != nil {
		t.Fatalf("Verify correct secret: %v", err)
	}
}

func TestVerifyWrongSecret(t *testing.T) {
	svc := auth.NewDeviceService(newStubStore())
	reg, _ := svc.Register(context.Background(), "app1", "dev1")
	if err := svc.Verify(context.Background(), reg.DeviceKeyID, "bad"); err == nil {
		t.Fatal("wrong secret must fail")
	}
}

func TestRotateKey(t *testing.T) {
	svc := auth.NewDeviceService(newStubStore())
	reg, _ := svc.Register(context.Background(), "app1", "dev1")
	if err := svc.RotateKey(context.Background(), reg.DeviceKeyID, reg.DeviceSecret, "newSecret123"); err != nil {
		t.Fatalf("RotateKey: %v", err)
	}
	if err := svc.Verify(context.Background(), reg.DeviceKeyID, reg.DeviceSecret); err == nil {
		t.Fatal("old secret must be rejected after rotation")
	}
	if err := svc.Verify(context.Background(), reg.DeviceKeyID, "newSecret123"); err != nil {
		t.Fatalf("new secret must work: %v", err)
	}
}

func TestRevokeDevice(t *testing.T) {
	svc := auth.NewDeviceService(newStubStore())
	reg, _ := svc.Register(context.Background(), "app1", "dev1")
	if err := svc.Revoke(context.Background(), reg.DeviceKeyID); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	if err := svc.Verify(context.Background(), reg.DeviceKeyID, reg.DeviceSecret); err == nil {
		t.Fatal("revoked device must be rejected")
	}
}
