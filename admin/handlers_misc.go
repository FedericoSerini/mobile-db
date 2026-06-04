package admin

import (
	"context"
	"html/template"
	"net/http"
	"strconv"
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

const syncPageSize = 25

func (h *MiscHandlers) SyncPage(w http.ResponseWriter, r *http.Request) {
	tab := r.URL.Query().Get("tab")
	if tab == "" {
		tab = "events"
	}

	var data map[string]any

	switch tab {
	case "aggregates":
		aggs, err := h.syncStore.ListSyncAggregates(r.Context())
		if err != nil {
			http.Error(w, "failed to load aggregates: "+err.Error(), http.StatusInternalServerError)
			return
		}
		data = map[string]any{"Tab": "aggregates", "Aggregates": aggs}

	default: // "events"
		pageStr := r.URL.Query().Get("page")
		page := 1
		if n, err := strconv.Atoi(pageStr); err == nil && n > 1 {
			page = n
		}
		events, total, err := h.syncStore.ListSyncEvents(r.Context(), "", page, syncPageSize)
		if err != nil {
			http.Error(w, "failed to load sync events: "+err.Error(), http.StatusInternalServerError)
			return
		}
		from := (page-1)*syncPageSize + 1
		to := from + len(events) - 1
		if len(events) == 0 {
			from = 0
		}
		data = map[string]any{
			"Tab":      "events",
			"Events":   events,
			"Total":    total,
			"Page":     page,
			"PageSize": syncPageSize,
			"From":     from,
			"To":       to,
			"HasPrev":  page > 1,
			"HasNext":  to < total,
			"PrevPage": page - 1,
			"NextPage": page + 1,
		}
	}

	var content strings.Builder
	if err := h.tmpl.ExecuteTemplate(&content, "sync", data); err != nil {
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
