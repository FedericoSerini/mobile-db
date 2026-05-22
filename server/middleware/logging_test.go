package middleware_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"

	"github.com/federicoserini/mobile-db/server/middleware"
)

func TestLoggingMiddlewareLogs(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf)
	handler := middleware.RequestLogger(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest("GET", "/sync", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	var entry map[string]any
	if err := json.NewDecoder(&buf).Decode(&entry); err != nil {
		t.Fatalf("log not valid JSON: %v — raw: %s", err, buf.String())
	}
	if entry["method"] != "GET" || entry["path"] != "/sync" {
		t.Fatalf("unexpected log entry: %v", entry)
	}
	if _, ok := entry["duration_ms"]; !ok {
		t.Fatal("missing duration_ms")
	}
	if entry["status"] != float64(200) {
		t.Fatalf("unexpected status: %v", entry["status"])
	}
}
