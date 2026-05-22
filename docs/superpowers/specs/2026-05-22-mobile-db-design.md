# mobile-db — Design Spec
_Date: 2026-05-22_

## Overview

Self-hosted JSON sync system. Single Go binary server + client library (mobile + web). Offline-first, CRDT merge, delta sync, encrypted everywhere. Designed for clean extensibility toward LoRa/IoT.

---

## Decisions Log

| Question | Decision | Rationale |
|---|---|---|
| Who uses this? | Self-hosted: owner builds apps on top | Not a platform, not an SDK |
| Conflict resolution | CRDT automatic merge | No data loss, deterministic |
| Scale | Personal/small team (A) | Simplifies storage, no sharding |
| Offline behavior | Offline-first | Full local copy, sync on reconnect |
| Auth model | Hybrid: device key + user JWT | App-level + user-level isolation |
| Transport | HTTP/2 + SSE | Multiplexed up, server-push down |
| Architecture | Go backend + cr-sqlite clients | Single binary, SQLite everywhere |
| Admin console | Server-side Go templates + HTMX | Ships in binary, no SPA |

---

## 1. Architecture

### 1.1 Code Structure — Ports & Adapters

```
core/
  crdt/           ← CRDT merge, op log, compaction — zero external deps
  sync/           ← sync protocol, transport-agnostic interfaces
  dataset/        ← document model, segregation rules, schema_version

transport/        ← implements core/sync transport interface
  http2/          ← HTTP/2 + SSE (current)
  lora/           ← LoRa gateway webhook handler (future v2)

storage/          ← implements core storage interface
  crsqlite/       ← cr-sqlite backend (mobile, web, server)
  iot/            ← minimal key-value backend for MCUs (future v2)

compression/      ← implements core codec interface
  zstd/           ← default (HTTP/2 transport)
  cbor/           ← binary encoding for IoT payloads

admin/            ← Go template handlers, embedded via go:embed
server/           ← wiring: HTTP router, middleware chain, config
```

**Rule:** `core/` imports nothing outside itself. All external dependencies wire in through interfaces defined in `core/`. Adding LoRa = implement `transport.SyncTransport`, plug in. No changes to `core/`.

### 1.2 Components

**Server (Go binary):**
- Sync engine (CRDT merge, delta compute, SSE broadcast)
- Auth layer (device key middleware + JWT middleware + admin Basic Auth)
- Admin console (Go templates embedded via `go:embed`, HTMX)
- Storage (SQLCipher-encrypted SQLite: `data.db` per tenant, `meta.db` for auth/device metadata)

**Mobile clients (iOS / Android):**
- cr-sqlite (native SQLite extension)
- SQLCipher at rest, key in Secure Enclave / Android Keystore
- Sync client (HTTP/2, cert pinning via URLSession / OkHttp)
- Device secret stored in Keychain / Keystore

**Web client (Browser):**
- cr-sqlite compiled to WASM (~3MB, cached after first load)
- AES-GCM encryption via SubtleCrypto (non-extractable key)
- TLS 1.3 + HSTS preload (no cert pinning — browser limitation)
- Device secret in SubtleCrypto + IndexedDB

---

## 2. Data Model

### 2.1 Segregation

```
app_id / user_id / dataset_id / doc_id
```

- `app_id` — tenant namespace. One SQLite `data.db` file per app.
- `user_id` — user within tenant, extracted from JWT.
- `dataset_id` — logical collection. One SQLite table per dataset.
- `doc_id` — individual JSON document. Each doc is an independent CRDT.

No cross-tenant access possible by construction (separate DB files).

### 2.2 Dataset ACL

Each dataset carries an access rule evaluated server-side before any merge:

| Rule | Read | Write |
|---|---|---|
| `private` | owner only | owner only |
| `shared` | any user in app | owner only |
| `public` | no auth required | owner only |

### 2.3 Schema Evolution

Each dataset stores a `schema_version` integer. Rules:

- Client stores `last_synced_schema_version` in local metadata.
- If server response `schema_version == last_synced_schema_version` → incremental CRDT op merge.
- If server response `schema_version > last_synced_schema_version` → client discards incremental ops, requests full snapshot, updates local `last_synced_schema_version`.
- Schema version bumped explicitly by owner via admin console or API.
- Old ops against old schema version are never replayed against a newer snapshot.

This prevents CRDT corruption when dataset structure changes.

---

## 3. Auth Layer

### 3.1 Device Keys — 1 active key per device

```sql
device_keys (
  device_key_id  TEXT PRIMARY KEY,   -- uuid
  device_id      TEXT NOT NULL,      -- stable device identifier
  app_id         TEXT NOT NULL,      -- tenant
  key_hash       TEXT NOT NULL,      -- bcrypt hash of secret
  status         TEXT NOT NULL,      -- 'active' | 'revoked'
  registered_at  DATETIME NOT NULL,
  last_seen_at   DATETIME
)
```

**Registration (first launch):**
1. Device sends `POST /devices/register { app_id, device_id }`
2. Server issues `{ device_key_id, device_secret }`
3. Client stores secret in hardware-backed secure storage

**Rotation (atomic, no grace period):**
1. Client generates new secret
2. `POST /devices/rotate-key { device_key_id, old_secret, new_secret }`
3. Server atomic swap — old secret immediately invalid

**Remote wipe:** `DELETE /devices/:device_key_id` → status `revoked` → device cannot sync

**Request auth header:** `X-Device-Key: <device_key_id>:<device_secret>`

### 3.2 User JWT

- Access token: 15min TTL, signed HS256 with `JWT_SECRET`
- Payload: `{ app_id, user_id, exp }`
- Refresh token: 30-day TTL, one-time use, rotated on each use
- Refresh token reuse detected → all tokens for that user revoked

**Token issuance:** App calls `POST /auth/token { app_id, user_id, device_key_id, device_secret }`. Server validates device key, then issues JWT with the `user_id` the app provides. Server does not validate user credentials — that is the app's responsibility. Server only signs the token.

**Request header:** `Authorization: Bearer <access_token>`

### 3.3 Admin Auth

- Single admin password, bcrypt hash stored in env var `ADMIN_PASSWORD_HASH`
- HTTP Basic Auth over TLS on all `/admin/*` routes
- Server-side session cookie: 1-hour TTL, `HttpOnly`, `Secure`, `SameSite=Strict`
- Login rate-limited: 5 attempts per 15 minutes per IP
- Optional: `ADMIN_ALLOWED_IPS` env var for IP allowlist

---

## 4. Sync Protocol

### 4.1 Delta Format

All sync payloads encoded as **CBOR**, compressed with **Zstd** on HTTP/2 transport. LoRa transport uses CBOR only (no Zstd — payload already tiny, compression overhead exceeds savings).

**Client → Server (upload delta):**
```
{
  app_id:     string,
  user_id:    string,
  dataset_id: string,
  clock:      HLC timestamp,     -- hybrid logical clock, last known server state
  ops:        [ CRDTOp, ... ]    -- operations since last sync
}
```

**Server → Client (response delta):**
```
{
  new_clock:  HLC timestamp,
  ops:        [ CRDTOp, ... ],   -- server ops client has not seen
  snapshot:   Document | null    -- set if schema_version unrecognized by client
}
```

### 4.2 CRDT Operation

Each op is commutative and idempotent (cr-sqlite handles this):
```
{
  op_id:     uuid,
  doc_id:    string,
  field:     string,             -- JSON path
  value:     any,
  timestamp: HLC,
  device_id: string
}
```

### 4.3 Sync Flow

1. Client writes locally → CRDT op appended to local log
2. On connectivity: client sends delta (CBOR + Zstd) via `POST /sync`
3. Server verifies device key + JWT, checks dataset ACL
4. Server runs CRDT merge via cr-sqlite
5. Server computes diff: ops client has not seen (by HLC clock comparison)
6. Server responds with delta (CBOR + Zstd)
7. Server broadcasts SSE event to other active devices in same `app_id/user_id`
8. Other devices pull their delta on SSE notification

### 4.4 Transport Interface (extensibility)

```go
type SyncTransport interface {
    Receive(ctx context.Context) (<-chan SyncMessage, error)
    Send(ctx context.Context, msg SyncMessage) error
    Notify(ctx context.Context, appID, userID string) error
}
```

HTTP/2 + SSE implements this interface today. LoRa gateway webhook implements the same interface in v2. Core sync engine never changes.

---

## 5. Op Log Compaction

Problem: CRDT op logs grow unbounded, cold sync gets slow.

**Trigger:** Server compacts a dataset when op count exceeds `COMPACTION_THRESHOLD` (default: 1000 ops). Configurable via env var.

**Process:**
1. Server computes materialized snapshot from all ops
2. Writes snapshot to `snapshots` table with current HLC clock
3. Marks compacted ops as `archived` (retained for configurable TTL, default 30 days)
4. New clients sync snapshot + tail ops (ops after snapshot HLC)
5. Archived ops pruned after TTL

**Client cold sync:** server detects client clock predates oldest non-archived snapshot → returns full snapshot, client rebuilds local state.

---

## 6. LoRa Packet Fragmentation

LoRa max payload: 51–242 bytes (region/SF dependent). CBOR-encoded CRDT ops may exceed this.

**Fragmentation (gateway-side, transparent to core):**
- Delta split into numbered chunks: `{ frag_id, seq, total, data }`
- Chunks transmitted sequentially
- Gateway reassembles chunks before forwarding to server sync endpoint
- Core sync engine receives complete `SyncMessage` — never sees fragmentation

**Interface boundary:** fragmentation lives entirely in `transport/lora/`. Core unchanged.

**IoT device registration:** pre-provisioned credentials (flashed at firmware build time). Full interactive registration flow is v2. Interface left open in `transport/lora/`.

---

## 7. Schema Evolution

See §2.3. Summary:
- `schema_version` integer per dataset
- Unknown version → full snapshot download, no incremental merge
- Version bumped via admin console
- Old ops never replayed against newer snapshot

---

## 8. Observability

### 8.1 Logging
- Library: `zerolog` (structured JSON, zero allocation)
- Every request logged: `method`, `path`, `app_id`, `device_id`, `duration_ms`, `status`
- Errors logged with stack trace
- Log level configurable via `LOG_LEVEL` env var (default: `info`)

### 8.2 Health Check
`GET /health` — no auth required:
```json
{
  "status": "ok",
  "uptime_seconds": 3600,
  "db_status": "ok",
  "active_sse_connections": 4,
  "version": "1.0.0"
}
```

### 8.3 Metrics
`GET /metrics` — Prometheus format, protected by admin auth:
- `mobiledb_sync_requests_total` (by app_id)
- `mobiledb_crdt_ops_total`
- `mobiledb_active_devices`
- `mobiledb_sse_connections`
- `mobiledb_compaction_runs_total`
- `mobiledb_db_size_bytes` (by tenant)

---

## 9. Backup

### 9.1 Database Backup
- Cron job (configurable interval, default: daily) dumps SQLCipher-encrypted SQLite files
- Destination: local path or object storage (S3-compatible: Backblaze B2, MinIO, AWS S3)
- Configured via env vars: `BACKUP_DESTINATION`, `BACKUP_INTERVAL`, `BACKUP_S3_*`
- SQLite Online Backup API used (no lock contention during backup)

### 9.2 Key Backup
- DB encryption key never stored on server disk
- Key backed up separately, encrypted with a master password stored offline
- Admin console shows key backup reminder if last backup > 30 days

---

## 10. Admin Console

Routes (all under `/admin/*`, Basic Auth gated):

| Route | Function |
|---|---|
| `/admin/devices` | List devices, revoke, remote wipe |
| `/admin/apps` | Manage app registrations |
| `/admin/data` | Browse datasets + documents (read + delete) |
| `/admin/import` | Bulk JSON import into a dataset |
| `/admin/sync` | Sync activity monitor (op counts, last seen) |
| `/admin/keys` | Trigger DB key rotation, backup status |
| `/admin/metrics` | Embedded Prometheus metrics view |

Implementation: Go `html/template`, embedded via `go:embed`, HTMX for dynamic interactions. No JS framework, no build step.

---

## 11. Encryption Summary

| Layer | Mechanism | Key location |
|---|---|---|
| Transit (mobile) | TLS 1.3 + cert pinning | N/A |
| Transit (web) | TLS 1.3 + HSTS preload | N/A |
| Client at rest (mobile) | SQLCipher | Secure Enclave / Android Keystore |
| Client at rest (web) | AES-GCM (SubtleCrypto) | Non-extractable CryptoKey |
| Server at rest | SQLCipher | Env var, never written to disk |
| Server key backup | AES-256 (master password) | Offline, owner-held |

Cloud provider or VPS host sees encrypted binary blobs only.

---

## 12. Deployment

Single Go binary. Zero runtime dependencies.

```
ADMIN_PASSWORD_HASH=<bcrypt>
JWT_SECRET=<random 32 bytes>
DB_ENCRYPTION_KEY=<random 32 bytes>
BACKUP_DESTINATION=s3://my-bucket/mobile-db
LOG_LEVEL=info
COMPACTION_THRESHOLD=1000
```

Runs on any VPS, Docker container, or bare metal. SQLite files on attached volume.

---

## Future Work (v2)

- LoRa IoT device registration (pre-provisioned credentials + challenge-response over LoRa)
- Minimal IoT storage backend (`storage/iot/`) for MCUs without SQLite
- Multi-admin accounts with role-based access
- Dataset-level encryption (per-dataset keys, separate from DB-level SQLCipher)
