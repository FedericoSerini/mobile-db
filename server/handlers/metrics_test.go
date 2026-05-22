package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/federicoserini/mobile-db/server/handlers"
)

func TestMetricsEndpoint(t *testing.T) {
	reg := handlers.NewMetricsRegistry()
	h := handlers.NewMetricsHandler(reg)
	reg.SyncRequests.WithLabelValues("app1").Inc()
	reg.DBSizeBytes.WithLabelValues("app1").Set(0) // initialize label so metric appears in output
	req := httptest.NewRequest("GET", "/metrics", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "mobiledb_sync_requests_total") {
		t.Fatal("missing mobiledb_sync_requests_total metric")
	}
	body := rr.Body.String()
	for _, name := range []string{
		"mobiledb_sync_requests_total",
		"mobiledb_crdt_ops_total",
		"mobiledb_active_devices",
		"mobiledb_sse_connections",
		"mobiledb_compaction_runs_total",
		"mobiledb_db_size_bytes",
	} {
		if !strings.Contains(body, name) {
			t.Errorf("missing metric: %s", name)
		}
	}
}
