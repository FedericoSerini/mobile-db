package admin_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/federicoserini/mobile-db/admin"
	"github.com/federicoserini/mobile-db/server/auth"
)

type stubDeviceStore struct {
	devices []auth.DeviceKey
}

func (s *stubDeviceStore) ListDevices(_ context.Context, _ string) ([]auth.DeviceKey, error) {
	return s.devices, nil
}

func (s *stubDeviceStore) RevokeDevice(_ context.Context, id string) error {
	for i, d := range s.devices {
		if d.DeviceKeyID == id {
			s.devices[i].Status = "revoked"
			return nil
		}
	}
	return auth.ErrDeviceNotFound
}

func TestDevicesPageRenders(t *testing.T) {
	store := &stubDeviceStore{
		devices: []auth.DeviceKey{
			{DeviceKeyID: "dk1", DeviceID: "phone-1", AppID: "app1",
				Status: "active", RegisteredAt: time.Now()},
		},
	}
	tmpl, err := admin.LoadTemplates()
	if err != nil {
		t.Fatalf("LoadTemplates: %v", err)
	}
	h := admin.NewDevicesHandler(store, tmpl)
	req := httptest.NewRequest("GET", "/admin/devices", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body)
	}
	if !strings.Contains(rr.Body.String(), "phone-1") {
		t.Fatal("page must contain device_id")
	}
}
