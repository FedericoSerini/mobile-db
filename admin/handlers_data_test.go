package admin_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/federicoserini/mobile-db/admin"
)

type stubDataStore struct{ datasets []string }

func (s *stubDataStore) ListDatasets(_ context.Context, _ string) ([]string, error) {
	return s.datasets, nil
}
func (s *stubDataStore) ListDocs(_ context.Context, _, _ string) ([]map[string]any, error) {
	return []map[string]any{{"doc_id": "doc1", "data": `{"name":"Alice"}`}}, nil
}
func (s *stubDataStore) DeleteDoc(_ context.Context, _, _, _ string) error { return nil }

func TestDataPageRenders(t *testing.T) {
	store := &stubDataStore{datasets: []string{"users", "posts"}}
	tmpl, _ := admin.LoadTemplates()
	h := admin.NewDataHandler(store, tmpl)
	req := httptest.NewRequest("GET", "/admin/data?app_id=app1", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "users") {
		t.Fatal("page must contain dataset names")
	}
}
