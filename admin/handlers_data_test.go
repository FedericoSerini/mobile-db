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
func (s *stubDataStore) ListDocs(_ context.Context, _, _, _ string) ([]map[string]any, error) {
	return []map[string]any{{
		"doc_id":     "doc1",
		"user_id":    "user-1",
		"data":       `{"name":"Alice"}`,
		"device_id":  "dev-1",
		"created_at": "2026-06-01 10:00:00",
		"wall_time":  int64(0),
	}}, nil
}
func (s *stubDataStore) GetDoc(_ context.Context, _, _, _ string) (map[string]any, error) {
	return map[string]any{
		"doc_id":     "doc1",
		"user_id":    "user-1",
		"data":       `{"name":"Alice"}`,
		"device_id":  "dev-1",
		"created_at": "2026-06-01",
		"wall_time":  int64(0),
	}, nil
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

func TestDocsPartialReturnsFragment(t *testing.T) {
	store := &stubDataStore{}
	tmpl, _ := admin.LoadTemplates()
	h := admin.NewDataHandler(store, tmpl)

	req := httptest.NewRequest("GET", "/admin/data/docs?app_id=app1&dataset_id=ds1", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	// Must be a fragment — no full HTML boilerplate
	if strings.Contains(body, "<!DOCTYPE") {
		t.Error("docs partial must not return a full HTML page")
	}
	if !strings.Contains(body, "doc1") {
		t.Error("docs partial must contain doc_id")
	}
	if !strings.Contains(body, "user-1") {
		t.Error("docs partial must contain user_id")
	}
}

func TestDocModalReturnsFragment(t *testing.T) {
	store := &stubDataStore{}
	tmpl, _ := admin.LoadTemplates()
	h := admin.NewDataHandler(store, tmpl)

	req := httptest.NewRequest("GET", "/admin/data/doc?dataset_id=ds1&user_id=user-1&doc_id=doc1", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	if strings.Contains(body, "<!DOCTYPE") {
		t.Error("doc modal must not return a full HTML page")
	}
	if !strings.Contains(body, `{"name":"Alice"}`) {
		t.Error("doc modal must contain full data JSON")
	}
}

func TestDeleteDocReturnsRow(t *testing.T) {
	store := &stubDataStore{}
	tmpl, _ := admin.LoadTemplates()
	h := admin.NewDataHandler(store, tmpl)

	req := httptest.NewRequest("DELETE", "/admin/data/doc?dataset_id=ds1&user_id=user-1&doc_id=doc1", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "<tr") {
		t.Error("deleteDoc must return a <tr> element for htmx swap")
	}
}
