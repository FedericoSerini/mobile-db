package admin

import (
	"context"
	"html/template"
	"net/http"
	"strings"

	"github.com/federicoserini/mobile-db/server/auth"
)

type DeviceAdminStore interface {
	ListDevices(ctx context.Context, appID string) ([]auth.DeviceKey, error)
	RevokeDevice(ctx context.Context, deviceKeyID string) error
}

type devicesHandler struct {
	store DeviceAdminStore
	tmpl  *template.Template
}

func NewDevicesHandler(store DeviceAdminStore, tmpl *template.Template) http.Handler {
	return &devicesHandler{store: store, tmpl: tmpl}
}

func (h *devicesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.list(w, r)
	case http.MethodDelete:
		h.revoke(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *devicesHandler) list(w http.ResponseWriter, r *http.Request) {
	devices, err := h.store.ListDevices(r.Context(), "")
	if err != nil {
		http.Error(w, "failed to list devices", http.StatusInternalServerError)
		return
	}
	var content strings.Builder
	if err := h.tmpl.ExecuteTemplate(&content, "devices", map[string]any{"Devices": devices}); err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	var page strings.Builder
	if err := h.tmpl.ExecuteTemplate(&page, "base", map[string]any{
		"Title":   "Devices",
		"Content": template.HTML(content.String()),
	}); err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(page.String()))
}

func (h *devicesHandler) revoke(w http.ResponseWriter, r *http.Request) {
	idx := strings.LastIndex(r.URL.Path, "/")
	deviceKeyID := r.URL.Path[idx+1:]
	if deviceKeyID == "" || deviceKeyID == "devices" {
		http.Error(w, "missing device_key_id", http.StatusBadRequest)
		return
	}
	if err := h.store.RevokeDevice(r.Context(), deviceKeyID); err != nil {
		http.Error(w, "revoke failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`<tr><td colspan="6" style="color:#888">Revoked</td></tr>`))
}
