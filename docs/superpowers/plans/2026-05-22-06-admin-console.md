# Admin Console Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the server-side admin console using Go html/template + HTMX embedded in the binary. All routes under `/admin/*` protected by admin Basic Auth middleware (Plan 04).

**Architecture:** `admin/` package holds handlers and Go template files. Templates embedded via `go:embed`. HTMX handles dynamic interactions (no JS framework, no build step). One handler file per admin section for clarity.

**Tech Stack:** Go html/template, go:embed, HTMX 1.x (served from CDN or embedded), net/http. Depends on Plans 01–04 (auth, storage).

**Prerequisite:** Plans 01–04 complete and passing.

---

### Task 1: Template loader + base layout

**Files:**
- Create: `admin/templates/base.html`
- Create: `admin/templates/nav.html`
- Create: `admin/embed.go`
- Create: `admin/embed_test.go`

- [ ] **Step 1: Write failing test**

```go
// admin/embed_test.go
package admin_test

import (
	"strings"
	"testing"

	"github.com/yourusername/mobile-db/admin"
)

func TestTemplatesLoad(t *testing.T) {
	tmpl, err := admin.LoadTemplates()
	if err != nil {
		t.Fatalf("LoadTemplates: %v", err)
	}
	if tmpl == nil {
		t.Fatal("templates must not be nil")
	}
	// Verify base template is defined
	if tmpl.Lookup("base") == nil {
		t.Fatal("template 'base' must be defined")
	}
}

func TestBaseTemplateRendersTitle(t *testing.T) {
	tmpl, _ := admin.LoadTemplates()
	var buf strings.Builder
	err := tmpl.ExecuteTemplate(&buf, "base", map[string]any{
		"Title":   "Test Page",
		"Content": "hello",
	})
	if err != nil {
		t.Fatalf("ExecuteTemplate: %v", err)
	}
	if !strings.Contains(buf.String(), "Test Page") {
		t.Fatal("rendered output must contain page title")
	}
}
```

- [ ] **Step 2: Run to verify fails**

```bash
go test ./admin/ -v
```

Expected: compile error `no Go files`.

- [ ] **Step 3: Write base.html**

```html
<!-- admin/templates/base.html -->
{{define "base"}}
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>{{.Title}} — mobile-db admin</title>
  <script src="https://unpkg.com/htmx.org@1.9.12"></script>
  <style>
    body { font-family: system-ui, sans-serif; margin: 0; background: #f9f9f9; }
    nav { background: #1a1a2e; color: white; padding: 0.75rem 1.5rem; display: flex; gap: 1.5rem; align-items: center; }
    nav a { color: #a8b4d8; text-decoration: none; font-size: 0.9rem; }
    nav a:hover { color: white; }
    nav .brand { font-weight: 700; color: white; margin-right: auto; }
    main { padding: 2rem; max-width: 1200px; margin: 0 auto; }
    table { width: 100%; border-collapse: collapse; background: white; border-radius: 6px; overflow: hidden; box-shadow: 0 1px 3px rgba(0,0,0,0.1); }
    th, td { padding: 0.75rem 1rem; text-align: left; border-bottom: 1px solid #eee; }
    th { background: #f0f2f5; font-weight: 600; font-size: 0.85rem; }
    .badge { display: inline-block; padding: 0.2rem 0.5rem; border-radius: 999px; font-size: 0.75rem; font-weight: 600; }
    .badge-active { background: #d1fae5; color: #065f46; }
    .badge-revoked { background: #fee2e2; color: #991b1b; }
    .btn { padding: 0.4rem 0.8rem; border: none; border-radius: 4px; cursor: pointer; font-size: 0.85rem; }
    .btn-danger { background: #ef4444; color: white; }
    .btn-primary { background: #3b82f6; color: white; }
    h1 { font-size: 1.5rem; margin-bottom: 1.5rem; }
  </style>
</head>
<body>
  {{template "nav" .}}
  <main>
    <h1>{{.Title}}</h1>
    {{.Content}}
  </main>
</body>
</html>
{{end}}
```

- [ ] **Step 4: Write nav.html**

```html
<!-- admin/templates/nav.html -->
{{define "nav"}}
<nav>
  <span class="brand">mobile-db</span>
  <a href="/admin/devices">Devices</a>
  <a href="/admin/apps">Apps</a>
  <a href="/admin/data">Data</a>
  <a href="/admin/sync">Sync</a>
  <a href="/admin/keys">Keys</a>
  <a href="/admin/metrics">Metrics</a>
</nav>
{{end}}
```

- [ ] **Step 5: Write embed.go**

```go
// admin/embed.go
package admin

import (
	"embed"
	"html/template"
)

//go:embed templates/*.html
var templateFS embed.FS

// LoadTemplates parses all templates from the embedded filesystem.
func LoadTemplates() (*template.Template, error) {
	return template.ParseFS(templateFS, "templates/*.html")
}
```

- [ ] **Step 6: Run tests**

```bash
go test ./admin/ -run TestTemplates -v
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add admin/templates/base.html admin/templates/nav.html admin/embed.go admin/embed_test.go
git commit -m "feat: admin base template + embed loader"
```

---

### Task 2: Devices page (/admin/devices)

**Files:**
- Create: `admin/templates/devices.html`
- Create: `admin/handlers_devices.go`
- Create: `admin/handlers_devices_test.go`

- [ ] **Step 1: Write failing test**

```go
// admin/handlers_devices_test.go
package admin_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yourusername/mobile-db/admin"
	"github.com/yourusername/mobile-db/server/auth"
)

type stubDeviceStore struct {
	devices []auth.DeviceKey
}

func (s *stubDeviceStore) ListDevices(_ context.Context, _ string) ([]auth.DeviceKey, error) {
	return s.devices, nil
}

func (s *stubDeviceStore) RevokeDevice(_ context.Context, deviceKeyID string) error {
	for i, d := range s.devices {
		if d.DeviceKeyID == deviceKeyID {
			s.devices[i].Status = "revoked"
			return nil
		}
	}
	return auth.ErrDeviceNotFound
}

func TestDevicesPageRendersTable(t *testing.T) {
	store := &stubDeviceStore{
		devices: []auth.DeviceKey{
			{DeviceKeyID: "dk1", DeviceID: "phone-1", AppID: "app1",
				Status: "active", RegisteredAt: time.Now()},
		},
	}
	tmpl, _ := admin.LoadTemplates()
	h := admin.NewDevicesHandler(store, tmpl)

	req := httptest.NewRequest("GET", "/admin/devices", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "phone-1") {
		t.Fatal("page must contain device_id")
	}
}
```

- [ ] **Step 2: Run to verify fails**

```bash
go test ./admin/ -run TestDevices -v
```

Expected: compile error `undefined: admin.NewDevicesHandler`.

- [ ] **Step 3: Write devices.html template**

```html
<!-- admin/templates/devices.html -->
{{define "devices"}}
<table>
  <thead>
    <tr>
      <th>Device Key ID</th>
      <th>Device ID</th>
      <th>App ID</th>
      <th>Status</th>
      <th>Registered</th>
      <th>Last Seen</th>
      <th>Actions</th>
    </tr>
  </thead>
  <tbody>
    {{range .Devices}}
    <tr>
      <td><code>{{.DeviceKeyID}}</code></td>
      <td>{{.DeviceID}}</td>
      <td>{{.AppID}}</td>
      <td><span class="badge {{if eq .Status "active"}}badge-active{{else}}badge-revoked{{end}}">{{.Status}}</span></td>
      <td>{{.RegisteredAt.Format "2006-01-02 15:04"}}</td>
      <td>{{if .LastSeenAt}}{{.LastSeenAt.Format "2006-01-02 15:04"}}{{else}}—{{end}}</td>
      <td>
        {{if eq .Status "active"}}
        <button class="btn btn-danger"
          hx-delete="/admin/devices/{{.DeviceKeyID}}"
          hx-confirm="Revoke device {{.DeviceKeyID}}?"
          hx-target="closest tr"
          hx-swap="outerHTML">Revoke</button>
        {{end}}
      </td>
    </tr>
    {{else}}
    <tr><td colspan="7">No devices registered.</td></tr>
    {{end}}
  </tbody>
</table>
{{end}}
```

- [ ] **Step 4: Write handlers_devices.go**

```go
// admin/handlers_devices.go
package admin

import (
	"context"
	"html/template"
	"net/http"
	"strings"

	"github.com/yourusername/mobile-db/server/auth"
)

// DeviceAdminStore is the persistence interface for admin device operations.
type DeviceAdminStore interface {
	ListDevices(ctx context.Context, appID string) ([]auth.DeviceKey, error)
	RevokeDevice(ctx context.Context, deviceKeyID string) error
}

type devicesHandler struct {
	store DeviceAdminStore
	tmpl  *template.Template
}

// NewDevicesHandler handles GET /admin/devices and DELETE /admin/devices/{id}.
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
	devices, err := h.store.ListDevices(r.Context(), "") // "" = all apps
	if err != nil {
		http.Error(w, "failed to list devices", http.StatusInternalServerError)
		return
	}
	var content strings.Builder
	if err := h.tmpl.ExecuteTemplate(&content, "devices", map[string]any{"Devices": devices}); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}
	h.tmpl.ExecuteTemplate(w, "base", map[string]any{
		"Title":   "Devices",
		"Content": template.HTML(content.String()),
	})
}

func (h *devicesHandler) revoke(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/admin/devices/")
	if path == "" {
		http.Error(w, "missing device_key_id", http.StatusBadRequest)
		return
	}
	if err := h.store.RevokeDevice(r.Context(), path); err != nil {
		http.Error(w, "revoke failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	// HTMX: return empty row to swap out the revoked row
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`<tr><td colspan="7" style="color:#888">Revoked</td></tr>`))
}
```

- [ ] **Step 5: Run tests**

```bash
go test ./admin/ -run TestDevices -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add admin/templates/devices.html admin/handlers_devices.go admin/handlers_devices_test.go
git commit -m "feat: admin devices page with HTMX revoke action"
```

---

### Task 3: Data browser (/admin/data)

**Files:**
- Create: `admin/templates/data.html`
- Create: `admin/handlers_data.go`
- Create: `admin/handlers_data_test.go`

- [ ] **Step 1: Write failing test**

```go
// admin/handlers_data_test.go
package admin_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yourusername/mobile-db/admin"
)

type stubDataStore struct {
	datasets []string
}

func (s *stubDataStore) ListDatasets(_ context.Context, appID string) ([]string, error) {
	return s.datasets, nil
}

func (s *stubDataStore) ListDocs(_ context.Context, appID, datasetID string) ([]map[string]any, error) {
	return []map[string]any{{"doc_id": "doc1", "name": "Alice"}}, nil
}

func (s *stubDataStore) DeleteDoc(_ context.Context, appID, datasetID, docID string) error {
	return nil
}

func TestDataPageListsDatasets(t *testing.T) {
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
		t.Fatal("page must contain dataset name")
	}
}
```

- [ ] **Step 2: Run to verify fails**

```bash
go test ./admin/ -run TestDataPage -v
```

Expected: compile error `undefined: admin.NewDataHandler`.

- [ ] **Step 3: Write data.html**

```html
<!-- admin/templates/data.html -->
{{define "data"}}
<form method="get" action="/admin/data" style="margin-bottom:1rem">
  <label>App ID: <input name="app_id" value="{{.AppID}}" required></label>
  <button class="btn btn-primary" type="submit">Load</button>
</form>
{{if .Datasets}}
<label>Dataset:
  <select name="dataset_id" hx-get="/admin/data/docs" hx-target="#docs-table"
          hx-vals='{"app_id":"{{.AppID}}"}'>
    <option value="">— choose —</option>
    {{range .Datasets}}<option value="{{.}}">{{.}}</option>{{end}}
  </select>
</label>
{{end}}
<div id="docs-table">
  {{if .Docs}}
  <table>
    <thead><tr><th>Doc ID</th><th>Data</th><th></th></tr></thead>
    <tbody>
      {{range .Docs}}
      <tr>
        <td><code>{{index . "doc_id"}}</code></td>
        <td><pre style="margin:0;font-size:0.8rem">{{json .}}</pre></td>
        <td>
          <button class="btn btn-danger"
            hx-delete="/admin/data/doc?app_id={{$.AppID}}&dataset_id={{$.DatasetID}}&doc_id={{index . "doc_id"}}"
            hx-confirm="Delete this document?"
            hx-target="closest tr" hx-swap="outerHTML">Delete</button>
        </td>
      </tr>
      {{end}}
    </tbody>
  </table>
  {{end}}
</div>
{{end}}
```

- [ ] **Step 4: Write handlers_data.go**

```go
// admin/handlers_data.go
package admin

import (
	"context"
	"encoding/json"
	"html/template"
	"net/http"
	"strings"
)

// DataAdminStore is the persistence interface for admin data browsing.
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
		"AppID":     appID,
		"DatasetID": datasetID,
		"Datasets":  datasets,
		"Docs":      docs,
	}

	funcMap := template.FuncMap{
		"json": func(v any) string {
			b, _ := json.MarshalIndent(v, "", "  ")
			return string(b)
		},
	}
	tmpl, err := h.tmpl.Funcs(funcMap).Clone()
	if err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}

	var content strings.Builder
	tmpl.ExecuteTemplate(&content, "data", data)
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
```

- [ ] **Step 5: Run tests**

```bash
go test ./admin/ -v
```

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add admin/templates/data.html admin/handlers_data.go admin/handlers_data_test.go
git commit -m "feat: admin data browser with doc listing and delete"
```

---

### Task 4: Sync activity + Keys pages

**Files:**
- Create: `admin/templates/sync.html`
- Create: `admin/templates/keys.html`
- Create: `admin/handlers_misc.go`

- [ ] **Step 1: Write sync.html**

```html
<!-- admin/templates/sync.html -->
{{define "sync"}}
<table>
  <thead><tr><th>App ID</th><th>Dataset ID</th><th>Op Count</th><th>Last Sync</th></tr></thead>
  <tbody>
    {{range .Entries}}
    <tr>
      <td>{{.AppID}}</td>
      <td>{{.DatasetID}}</td>
      <td>{{.OpCount}}</td>
      <td>{{.LastSync.Format "2006-01-02 15:04:05"}}</td>
    </tr>
    {{else}}
    <tr><td colspan="4">No sync activity.</td></tr>
    {{end}}
  </tbody>
</table>
<button class="btn btn-primary" hx-get="/admin/sync" hx-target="body" hx-push-url="true"
        style="margin-top:1rem">Refresh</button>
{{end}}
```

- [ ] **Step 2: Write keys.html**

```html
<!-- admin/templates/keys.html -->
{{define "keys"}}
<section>
  <h2>Database Encryption Key</h2>
  {{if .KeyBackupStale}}
  <div style="background:#fee2e2;padding:1rem;border-radius:6px;margin-bottom:1rem">
    ⚠ Last key backup was over 30 days ago. Please back up your encryption key.
  </div>
  {{end}}
  <button class="btn btn-primary"
    hx-post="/admin/keys/rotate"
    hx-confirm="Rotate DB encryption key? All data will be re-encrypted. This may take a moment."
    hx-target="#key-status">Rotate Key</button>
  <div id="key-status"></div>
</section>
{{end}}
```

- [ ] **Step 3: Write handlers_misc.go**

```go
// admin/handlers_misc.go
package admin

import (
	"context"
	"html/template"
	"net/http"
	"strings"
	"time"
)

// SyncEntry is one row in the sync activity table.
type SyncEntry struct {
	AppID     string
	DatasetID string
	OpCount   int
	LastSync  time.Time
}

// SyncAdminStore is the persistence interface for sync activity data.
type SyncAdminStore interface {
	ListSyncActivity(ctx context.Context) ([]SyncEntry, error)
}

// MiscHandlers holds handlers for sync activity and key management pages.
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
```

- [ ] **Step 4: Build**

```bash
go build ./admin/...
```

Expected: no errors.

- [ ] **Step 5: Commit**

```bash
git add admin/templates/sync.html admin/templates/keys.html admin/handlers_misc.go
git commit -m "feat: admin sync activity and key management pages"
```

---

### Task 5: Admin router

**Files:**
- Create: `admin/router.go`
- Create: `admin/router_test.go`

- [ ] **Step 1: Write failing test**

```go
// admin/router_test.go
package admin_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yourusername/mobile-db/admin"
	"golang.org/x/crypto/bcrypt"
)

func TestAdminRouterRequiresAuth(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("pass"), 4)
	tmpl, _ := admin.LoadTemplates()
	router := admin.NewAdminRouter(admin.AdminRouterDeps{
		PasswordHash: string(hash),
		Tmpl:         tmpl,
	})

	req := httptest.NewRequest("GET", "/admin/devices", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 without auth, got %d", rr.Code)
	}
}

func TestAdminRouterDevicesWithAuth(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("pass"), 4)
	tmpl, _ := admin.LoadTemplates()
	router := admin.NewAdminRouter(admin.AdminRouterDeps{
		PasswordHash: string(hash),
		DeviceStore:  &stubDeviceStore{},
		DataStore:    &stubDataStore{},
		SyncStore:    &stubSyncStore{},
		Tmpl:         tmpl,
	})

	req := httptest.NewRequest("GET", "/admin/devices", nil)
	req.SetBasicAuth("admin", "pass")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body)
	}
}

type stubSyncStore struct{}

func (s *stubSyncStore) ListSyncActivity(_ context.Context) ([]admin.SyncEntry, error) {
	return nil, nil
}
```

- [ ] **Step 2: Run to verify fails**

```bash
go test ./admin/ -run TestAdminRouter -v
```

Expected: compile error `undefined: admin.NewAdminRouter`.

- [ ] **Step 3: Write router.go**

```go
// admin/router.go
package admin

import (
	"html/template"
	"net/http"

	"github.com/yourusername/mobile-db/server/middleware"
)

// AdminRouterDeps contains all dependencies for the admin sub-router.
type AdminRouterDeps struct {
	PasswordHash string
	AllowedIPs   []string
	DeviceStore  DeviceAdminStore
	DataStore    DataAdminStore
	SyncStore    SyncAdminStore
	Tmpl         *template.Template
}

// NewAdminRouter returns an http.Handler for all /admin/* routes.
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
```

- [ ] **Step 4: Run all admin tests**

```bash
go test ./admin/ -v
```

Expected: all PASS.

- [ ] **Step 5: Run full suite**

```bash
go test ./... -v
```

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add admin/router.go admin/router_test.go
git commit -m "feat: admin sub-router with Basic Auth protection on all /admin/* routes"
```
