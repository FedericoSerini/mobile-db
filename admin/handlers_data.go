package admin

import (
	"context"
	"html/template"
	"net/http"
	"strings"
)

type DataAdminStore interface {
	ListDatasets(ctx context.Context, appID string) ([]string, error)
	ListDocs(ctx context.Context, appID, datasetID, userID string) ([]map[string]any, error)
	GetDoc(ctx context.Context, datasetID, userID, docID string) (map[string]any, error)
	DeleteDoc(ctx context.Context, datasetID, userID, docID string) error
}

type dataHandler struct {
	store DataAdminStore
	tmpl  *template.Template
}

func NewDataHandler(store DataAdminStore, tmpl *template.Template) http.Handler {
	return &dataHandler{store: store, tmpl: tmpl}
}

func (h *dataHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.browse(w, r)
	case http.MethodDelete:
		h.deleteDoc(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *dataHandler) browse(w http.ResponseWriter, r *http.Request) {
	appID := r.URL.Query().Get("app_id")
	datasetID := r.URL.Query().Get("dataset_id")
	var datasets []string
	var docs []map[string]any
	if appID != "" {
		var err error
		datasets, err = h.store.ListDatasets(r.Context(), appID)
		if err != nil {
			http.Error(w, "failed to list datasets: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}
	if appID != "" && datasetID != "" {
		userID := r.URL.Query().Get("user_id")
		var err error
		docs, err = h.store.ListDocs(r.Context(), appID, datasetID, userID)
		if err != nil {
			http.Error(w, "failed to list docs: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}
	data := map[string]any{
		"AppID": appID, "DatasetID": datasetID,
		"Datasets": datasets, "Docs": docs,
	}
	var content strings.Builder
	if err := h.tmpl.ExecuteTemplate(&content, "data", data); err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	var page strings.Builder
	if err := h.tmpl.ExecuteTemplate(&page, "base", map[string]any{
		"Title":   "Data Browser",
		"Content": template.HTML(content.String()),
	}); err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(page.String()))
}

func (h *dataHandler) deleteDoc(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	datasetID, userID, docID := q.Get("dataset_id"), q.Get("user_id"), q.Get("doc_id")
	if datasetID == "" || userID == "" || docID == "" {
		http.Error(w, "missing query params", http.StatusBadRequest)
		return
	}
	if err := h.store.DeleteDoc(r.Context(), datasetID, userID, docID); err != nil {
		http.Error(w, "delete failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}
