package admin

import (
	"context"
	"html/template"
	"net/http"
	"strings"
	"time"
)

type SyncEvent struct {
	ID          int64
	AppID       string
	DatasetID   string
	UserID      string
	DeviceKeyID string
	OpCount     int
	SyncedAt    time.Time
}

type SyncAggregate struct {
	AppID     string
	DatasetID string
	TotalOps  int
	Syncs24h  int
	LastSync  time.Time
}

type SyncAdminStore interface {
	ListSyncEvents(ctx context.Context, appID string, page, pageSize int) ([]SyncEvent, int, error)
	ListSyncAggregates(ctx context.Context) ([]SyncAggregate, error)
}

type MiscHandlers struct {
	syncStore      SyncAdminStore
	keyBackupStale bool
	tmpl           *template.Template
}

func NewMiscHandlers(syncStore SyncAdminStore, keyBackupStale bool, tmpl *template.Template) *MiscHandlers {
	return &MiscHandlers{syncStore: syncStore, keyBackupStale: keyBackupStale, tmpl: tmpl}
}

func (h *MiscHandlers) SyncPage(w http.ResponseWriter, r *http.Request) {
	aggregates, err := h.syncStore.ListSyncAggregates(r.Context())
	if err != nil {
		http.Error(w, "failed to list sync activity: "+err.Error(), http.StatusInternalServerError)
		return
	}
	var content strings.Builder
	if err := h.tmpl.ExecuteTemplate(&content, "sync", map[string]any{"Aggregates": aggregates}); err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	var page strings.Builder
	if err := h.tmpl.ExecuteTemplate(&page, "base", map[string]any{
		"Title":   "Sync Activity",
		"Content": template.HTML(content.String()),
	}); err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(page.String()))
}

func (h *MiscHandlers) KeysPage(w http.ResponseWriter, r *http.Request) {
	var content strings.Builder
	if err := h.tmpl.ExecuteTemplate(&content, "keys", map[string]any{
		"KeyBackupStale": h.keyBackupStale,
	}); err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	var page strings.Builder
	if err := h.tmpl.ExecuteTemplate(&page, "base", map[string]any{
		"Title":   "Key Management",
		"Content": template.HTML(content.String()),
	}); err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(page.String()))
}
