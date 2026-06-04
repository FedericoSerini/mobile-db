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
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/admin/data":
		h.browse(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/admin/data/docs":
		h.docsPartial(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/admin/data/doc":
		h.docModal(w, r)
	case r.Method == http.MethodDelete && r.URL.Path == "/admin/data/doc":
		h.deleteDoc(w, r)
	default:
		http.Error(w, "not found", http.StatusNotFound)
	}
}

func (h *dataHandler) browse(w http.ResponseWriter, r *http.Request) {
	appID := r.URL.Query().Get("app_id")
	var datasets []string
	if appID != "" {
		var err error
		datasets, err = h.store.ListDatasets(r.Context(), appID)
		if err != nil {
			http.Error(w, "failed to list datasets: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}
	data := map[string]any{"AppID": appID, "Datasets": datasets}
	var content strings.Builder
	if err := h.tmpl.ExecuteTemplate(&content, "data", data); err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	h.renderPage(w, "Data Browser", content.String())
}

func (h *dataHandler) docsPartial(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	appID, datasetID, userID := q.Get("app_id"), q.Get("dataset_id"), q.Get("user_id")
	docs, err := h.store.ListDocs(r.Context(), appID, datasetID, userID)
	if err != nil {
		http.Error(w, "failed to list docs: "+err.Error(), http.StatusInternalServerError)
		return
	}
	var buf strings.Builder
	if err := h.tmpl.ExecuteTemplate(&buf, "data_docs", map[string]any{
		"Docs":      docs,
		"AppID":     appID,
		"DatasetID": datasetID,
		"UserID":    userID,
	}); err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(buf.String()))
}

func (h *dataHandler) docModal(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	datasetID, userID, docID := q.Get("dataset_id"), q.Get("user_id"), q.Get("doc_id")
	if datasetID == "" || userID == "" || docID == "" {
		http.Error(w, "missing query params", http.StatusBadRequest)
		return
	}
	doc, err := h.store.GetDoc(r.Context(), datasetID, userID, docID)
	if err != nil {
		http.Error(w, "doc not found: "+err.Error(), http.StatusNotFound)
		return
	}
	var buf strings.Builder
	if err := h.tmpl.ExecuteTemplate(&buf, "data_doc_modal", doc); err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(buf.String()))
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
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`<tr class="row-deleted"><td colspan="6" style="color:#9ca3af;text-align:center;font-style:italic">deleted</td></tr>`))
}

func (h *dataHandler) renderPage(w http.ResponseWriter, title, contentHTML string) {
	var page strings.Builder
	if err := h.tmpl.ExecuteTemplate(&page, "base", map[string]any{
		"Title":   title,
		"Content": template.HTML(contentHTML),
	}); err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(page.String()))
}
