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

func (s *stubDeviceStore) ListDevices(_ context.Context, status string) ([]auth.DeviceKey, error) {
	if status == "" {
		return s.devices, nil
	}
	var out []auth.DeviceKey
	for _, d := range s.devices {
		if d.Status == status {
			out = append(out, d)
		}
	}
	return out, nil
}

func (s *stubDeviceStore) DeviceCounts(_ context.Context) (all, active, revoked int, err error) {
	for _, d := range s.devices {
		all++
		switch d.Status {
		case "active":
			active++
		case "revoked":
			revoked++
		}
	}
	return all, active, revoked, nil
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

func TestDevicesTabStrip(t *testing.T) {
	store := &stubDeviceStore{
		devices: []auth.DeviceKey{
			{DeviceKeyID: "dk1", DeviceID: "phone-1", AppID: "app1", Status: "active", RegisteredAt: time.Now()},
			{DeviceKeyID: "dk2", DeviceID: "phone-2", AppID: "app1", Status: "revoked", RegisteredAt: time.Now()},
		},
	}
	tmpl, _ := admin.LoadTemplates()
	h := admin.NewDevicesHandler(store, tmpl)

	// Active tab should only show active devices
	req := httptest.NewRequest("GET", "/admin/devices?status=active", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "phone-1") {
		t.Error("active tab must show active device")
	}
	if strings.Contains(body, "phone-2") {
		t.Error("active tab must not show revoked device")
	}
	if !strings.Contains(body, "Active") {
		t.Error("page must contain tab labels")
	}
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
