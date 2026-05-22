package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/federicoserini/mobile-db/server/handlers"
)

func TestHealthEndpoint(t *testing.T) {
	h := handlers.NewHealthHandler("1.0.0")
	req := httptest.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}
	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("not valid JSON: %v", err)
	}
	if resp["status"] != "ok" {
		t.Fatalf("want status=ok, got %v", resp["status"])
	}
	if resp["version"] != "1.0.0" {
		t.Fatalf("want version=1.0.0, got %v", resp["version"])
	}
	if _, ok := resp["uptime_seconds"]; !ok {
		t.Fatal("missing uptime_seconds")
	}
	if resp["db_status"] != "ok" {
		t.Fatalf("want db_status=ok, got %v", resp["db_status"])
	}
	if _, ok := resp["active_sse_connections"]; !ok {
		t.Fatal("missing active_sse_connections")
	}
}
