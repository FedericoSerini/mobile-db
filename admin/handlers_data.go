package admin

import (
	"context"
	"html/template"
	"net/http"
	"strings"
)

type DataAdminStore interface {
	ListDatasets(ctx context.Context, appID string) ([]string, error)
	ListDocs(ctx context.Context, appID, datasetID string) ([]map[string]any, error)
	DeleteDoc(ctx context.Context, appID, datasetID, docID string) error
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
		datasets, _ = h.store.ListDatasets(r.Context(), appID)
	}
	if appID != "" && datasetID != "" {
		docs, _ = h.store.ListDocs(r.Context(), appID, datasetID)
	}
	data := map[string]any{
		"AppID": appID, "DatasetID": datasetID,
		"Datasets": datasets, "Docs": docs,
	}
	var content strings.Builder
	h.tmpl.ExecuteTemplate(&content, "data", data)
	h.tmpl.ExecuteTemplate(w, "base", map[string]any{
		"Title":   "Data Browser",
		"Content": template.HTML(content.String()),
	})
}

func (h *dataHandler) deleteDoc(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	appID, datasetID, docID := q.Get("app_id"), q.Get("dataset_id"), q.Get("doc_id")
	if appID == "" || datasetID == "" || docID == "" {
		http.Error(w, "missing query params", http.StatusBadRequest)
		return
	}
	if err := h.store.DeleteDoc(r.Context(), appID, datasetID, docID); err != nil {
		http.Error(w, "delete failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}
