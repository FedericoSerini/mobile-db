package admin_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/federicoserini/mobile-db/admin"
	"golang.org/x/crypto/bcrypt"
)

type stubSyncStore struct{}

func (s *stubSyncStore) ListSyncEvents(_ context.Context, _ string, _, _ int) ([]admin.SyncEvent, int, error) {
	return nil, 0, nil
}

func (s *stubSyncStore) ListSyncAggregates(_ context.Context) ([]admin.SyncAggregate, error) {
	return nil, nil
}

func TestAdminRouterRequiresAuth(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("pass"), 4)
	tmpl, _ := admin.LoadTemplates()
	router := admin.NewAdminRouter(admin.AdminRouterDeps{
		PasswordHash: string(hash),
		Tmpl:         tmpl,
		DeviceStore:  &stubDeviceStore{},
		DataStore:    &stubDataStore{},
		SyncStore:    &stubSyncStore{},
		MetricsReg:   &stubMetrics{},
	})
	req := httptest.NewRequest("GET", "/admin/devices", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 without auth, got %d", rr.Code)
	}
}

func TestAdminRouterWithAuth(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("pass"), 4)
	tmpl, _ := admin.LoadTemplates()
	router := admin.NewAdminRouter(admin.AdminRouterDeps{
		PasswordHash: string(hash),
		Tmpl:         tmpl,
		DeviceStore:  &stubDeviceStore{},
		DataStore:    &stubDataStore{},
		SyncStore:    &stubSyncStore{},
		MetricsReg:   &stubMetrics{},
	})
	req := httptest.NewRequest("GET", "/admin/devices", nil)
	req.SetBasicAuth("admin", "pass")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body)
	}
}
