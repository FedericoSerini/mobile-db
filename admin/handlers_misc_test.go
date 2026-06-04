package admin_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/federicoserini/mobile-db/admin"
)

// fixedSyncStore returns deterministic data for handler-level tests.
// (stubSyncStore with empty returns is defined in router_test.go; this one
// returns real rows so we can assert on rendered content.)
type fixedSyncStore struct{}

func (s *fixedSyncStore) ListSyncEvents(_ context.Context, _ string, _, _ int) ([]admin.SyncEvent, int, error) {
	return []admin.SyncEvent{
		{ID: 1, AppID: "app1", DatasetID: "notes", UserID: "u1", DeviceKeyID: "dk1", OpCount: 5, SyncedAt: time.Now()},
	}, 1, nil
}

func (s *fixedSyncStore) ListSyncAggregates(_ context.Context) ([]admin.SyncAggregate, error) {
	return []admin.SyncAggregate{
		{AppID: "app1", DatasetID: "notes", TotalOps: 100, Syncs24h: 5, LastSync: time.Now()},
	}, nil
}

func TestSyncPageEventLogTab(t *testing.T) {
	tmpl, _ := admin.LoadTemplates()
	h := admin.NewMiscHandlers(&fixedSyncStore{}, false, tmpl)

	req := httptest.NewRequest("GET", "/admin/sync?tab=events", nil)
	rr := httptest.NewRecorder()
	h.SyncPage(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "app1") {
		t.Error("event log must contain app_id")
	}
	if !strings.Contains(body, "notes") {
		t.Error("event log must contain dataset_id")
	}
}

func TestSyncPageAggregatesTab(t *testing.T) {
	tmpl, _ := admin.LoadTemplates()
	h := admin.NewMiscHandlers(&fixedSyncStore{}, false, tmpl)

	req := httptest.NewRequest("GET", "/admin/sync?tab=aggregates", nil)
	rr := httptest.NewRecorder()
	h.SyncPage(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "100") {
		t.Error("aggregates tab must show TotalOps")
	}
}
