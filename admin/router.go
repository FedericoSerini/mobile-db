package admin

import (
	"html/template"
	"net/http"

	"github.com/federicoserini/mobile-db/server/middleware"
)

type AdminRouterDeps struct {
	PasswordHash string
	AllowedIPs   []string
	DeviceStore  DeviceAdminStore
	DataStore    DataAdminStore
	SyncStore    SyncAdminStore
	Tmpl         *template.Template
}

func NewAdminRouter(d AdminRouterDeps) http.Handler {
	mux := http.NewServeMux()

	devices := NewDevicesHandler(d.DeviceStore, d.Tmpl)
	data := NewDataHandler(d.DataStore, d.Tmpl)
	misc := NewMiscHandlers(d.SyncStore, false, d.Tmpl)

	mux.Handle("/admin/devices", devices)
	mux.Handle("/admin/devices/", devices)
	mux.Handle("/admin/data", data)
	mux.Handle("/admin/data/", data)
	mux.HandleFunc("/admin/sync", misc.SyncPage)
	mux.HandleFunc("/admin/keys", misc.KeysPage)
	mux.HandleFunc("/admin/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/admin/devices", http.StatusFound)
	})

	return middleware.RequireAdminAuth(d.PasswordHash, d.AllowedIPs)(mux)
}
