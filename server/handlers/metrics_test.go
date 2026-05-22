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
	req := httptest.NewRequest("GET", "/metrics", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "mobiledb_sync_requests_total") {
		t.Fatal("missing mobiledb_sync_requests_total metric")
	}
}
