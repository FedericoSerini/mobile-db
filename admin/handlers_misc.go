package admin

import (
	"context"
	"html/template"
	"net/http"
	"strings"
	"time"
)

type SyncEntry struct {
	AppID     string
	DatasetID string
	OpCount   int
	LastSync  time.Time
}

type SyncAdminStore interface {
	ListSyncActivity(ctx context.Context) ([]SyncEntry, error)
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
	entries, _ := h.syncStore.ListSyncActivity(r.Context())
	var content strings.Builder
	h.tmpl.ExecuteTemplate(&content, "sync", map[string]any{"Entries": entries})
	h.tmpl.ExecuteTemplate(w, "base", map[string]any{
		"Title":   "Sync Activity",
		"Content": template.HTML(content.String()),
	})
}

func (h *MiscHandlers) KeysPage(w http.ResponseWriter, r *http.Request) {
	var content strings.Builder
	h.tmpl.ExecuteTemplate(&content, "keys", map[string]any{
		"KeyBackupStale": h.keyBackupStale,
	})
	h.tmpl.ExecuteTemplate(w, "base", map[string]any{
		"Title":   "Key Management",
		"Content": template.HTML(content.String()),
	})
}
