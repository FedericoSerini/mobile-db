# Admin UI Redesign

**Date:** 2026-06-04  
**Status:** Approved

## Problem

The existing admin UI has four broken or incomplete areas:

1. **Devices page** — no status filter; active and revoked tokens mixed with no way to separate them.
2. **Data browser** — non-functional: selecting a dataset fires an htmx GET that returns a full HTML page injected into a `<div>`, producing a nested-page blob. `ListDocs` drops `user_id`, `device_id`, `created_at`, `wall_time` columns. No way to view full document JSON.
3. **Sync page** — `LastSync` is faked with `time.Now()`. No real event history. No aggregates.
4. **Metrics** — Prometheus counters tracked in-memory but not surfaced in the admin UI.

## Approach: B — Fixes + sync event log

Fix all bugs, add missing columns, wire the metrics page, and add a real `sync_events` table for history. No new JS dependencies. htmx stays.

---

## Schema

New table in `meta.db`:

```sql
CREATE TABLE IF NOT EXISTS sync_events (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    app_id      TEXT NOT NULL,
    dataset_id  TEXT NOT NULL,
    user_id     TEXT NOT NULL,
    device_id   TEXT NOT NULL,
    op_count    INTEGER NOT NULL DEFAULT 0,
    synced_at   DATETIME NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_sync_events_app ON sync_events(app_id, synced_at DESC);
```

Added to `storage/crsqlite/meta.go` alongside the existing DDL block.

---

## Write Path

`server/handlers/sync.go` inserts one row into `sync_events` after a successful merge. Single `INSERT`, no transaction wrapping needed. `app_id`, `dataset_id`, `user_id`, `device_id` are available from the request context; `op_count` from the merge result.

---

## Pages

### Devices (`/admin/devices`)

- **Tab strip** above the table: `All (N)` · `Active (N)` · `Revoked (N)`.
- Tab state lives in `?status=all|active|revoked` query param (default: `all`).
- `ListDevices` gains an optional `status` filter passed from the handler.
- Counts for each tab fetched in one query (`SELECT status, COUNT(*) … GROUP BY status`).
- Revoke button stays htmx DELETE; response returns a replacement `<tr>` marking the row revoked (not an empty body).

### Data Browser (`/admin/data`)

**Two-level filter:** App ID form submit → dataset `<select>` → optional user_id input.

**Fixed htmx flow:**
- Dataset `<select>` fires `hx-get="/admin/data/docs"` — this endpoint returns **only** the `<table>` fragment, not the full page.
- New `GET /admin/data/docs` handler: renders `data_docs` template partial. Accepts `app_id`, `dataset_id`, `user_id` (optional) query params.

**Table columns:** `doc_id` · `user_id` · `data` (truncated to 60 chars) · `device_id` · `created_at` · Delete button.

**`ListDocs` query fix:**
```sql
SELECT doc_id, user_id, data, device_id, created_at
FROM snapshots
WHERE dataset_id = ?
  AND (? = '' OR user_id = ?)
ORDER BY created_at DESC
```

**Document detail — modal (option B):**
- Clicking a row's `doc_id` fires `hx-get="/admin/data/doc"` with `hx-target="#doc-modal"` `hx-swap="innerHTML"`.
- A fixed `<div id="doc-modal">` lives at the bottom of the page; JS toggles `display` on open/close.
- Modal shows: full pretty-printed JSON, `user_id`, `device_id`, `wall_time`, `created_at`.
- New `GET /admin/data/doc` handler returns the modal inner fragment.

**`deleteDoc` fix:** returns `200` with a `<tr>` containing a single `<td colspan="6">` struck-through "Deleted" cell so htmx `hx-swap="outerHTML"` has content to swap in, then fades out via a CSS transition.

### Sync (`/admin/sync`)

Two tabs via `?tab=events|aggregates` (default: `events`).

**Event Log tab:**
- Table: `synced_at` · `app_id` · `dataset_id` · `user_id` · `device_id` · `op_count`.
- Pagination: 25 rows/page, `?page=N`. Prev/Next buttons. Shows "Showing X–Y of Z".
- New store method: `ListSyncEvents(ctx, appID, page, pageSize int) ([]SyncEvent, total int, error)`.

**Aggregates tab:**
- Table: `app_id` · `dataset_id` · `total_ops` · `syncs_24h` · `last_sync`.
- Computed with two SQL expressions against `sync_events`.
- New store method: `ListSyncAggregates(ctx) ([]SyncAggregate, error)`.

`SyncAdminStore` interface gains both methods. `MiscHandlers.SyncPage` reads `?tab` and dispatches accordingly.

### Metrics (`/admin/metrics`)

New page. Reads from `*handlers.MetricsRegistry` passed into `AdminRouterDeps`.

**Six stat cards** (3-column grid):

| Metric | Source |
|--------|--------|
| Active Devices | `mobiledb_active_devices` gauge |
| CRDT Ops Total | `mobiledb_crdt_ops_total` counter |
| SSE Connections | `mobiledb_sse_connections` gauge |
| Compaction Runs | `mobiledb_compaction_runs_total` counter |
| Sync Requests | `mobiledb_sync_requests_total` (sum across all app_id labels) |
| DB Size | `mobiledb_db_size_bytes` (sum, formatted as MB) |

Below cards: per-app breakdown table (`app_id` · `sync_requests` · `db_size_bytes`).

Values read by calling `MetricsRegistry.Gather()` and walking the `dto.MetricFamily` slice — no Prometheus text parsing needed.

New files: `admin/templates/metrics.html`, `admin/handlers_metrics.go`.  
`AdminRouterDeps` gains `MetricsReg *handlers.MetricsRegistry`.  
Router adds `mux.HandleFunc("/admin/metrics", misc.MetricsPage)`.

---

## Styling Constraint

All text must meet WCAG AA contrast on its actual background. Watch specifically:

- `code`/`pre` blocks — use `#1e40af` text on `#eff6ff` background (blue-50), not dark-on-dark.
- Secondary gray text (`#6b7280`) only on white or `#f9f9f9` backgrounds.
- Badges (`badge-active`, `badge-revoked`) — existing colors are fine on white rows; never place them on colored row backgrounds.
- Nav links on `#1a1a2e` — existing `#a8b4d8` passes AA; do not darken further.

---

## Nav

Add `Metrics` link between `Sync` and `Keys` in `nav.html`.

---

## Files Changed

| File | Change |
|------|--------|
| `storage/crsqlite/meta.go` | Add `sync_events` DDL |
| `storage/crsqlite/adminstore.go` | Fix `ListDocs`, add `ListSyncEvents`, `ListSyncAggregates`, `ListDevicesWithCounts` |
| `server/handlers/sync.go` | Insert `sync_events` row on successful merge |
| `admin/router.go` | Add `/admin/metrics`, `/admin/data/docs`, `/admin/data/doc` routes; add `MetricsReg` to deps |
| `admin/handlers_devices.go` | Status tab counts, filter by `?status` |
| `admin/handlers_data.go` | New `docs` partial handler, new `doc` modal handler, fix `deleteDoc` response |
| `admin/handlers_misc.go` | Sync two-tab dispatch, new `MetricsPage` handler |
| `admin/handlers_metrics.go` | New file — reads `MetricsRegistry`, renders stat cards |
| `admin/templates/nav.html` | Add Metrics link |
| `admin/templates/devices.html` | Tab strip |
| `admin/templates/data.html` | Fix htmx targets, add columns, modal div, partial template |
| `admin/templates/sync.html` | Two-tab layout, pagination |
| `admin/templates/metrics.html` | New file — stat cards + per-app table |

---

## Error Handling

- All handler errors render an inline `<div class="error-banner">` (red, dismissible) rather than a bare `http.Error` plain-text response.
- Partial endpoints (docs table, modal) return htmx-compatible error fragments so the user sees feedback without a full page reload.
- Store errors are logged server-side; client sees a generic message.
