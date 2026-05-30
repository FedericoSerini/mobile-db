# mobile-db

Self-hosted SQLite sync server for offline-first mobile and web apps. Single Go binary. Uses [cr-sqlite](https://github.com/vlcn-io/cr-sqlite) for deterministic CRDT merge — no conflicts, no data loss.

## Architecture

| Layer | Technology |
|---|---|
| Sync | CRDT op-log, delta sync, SSE push |
| Auth | Device keys + JWT (app-level + user-level isolation) |
| Transport | HTTP/2 + SSE |
| Storage | SQLite via cr-sqlite extension |
| Admin UI | Go templates + HTMX (embedded in binary) |

## Deployment

### Docker Compose (recommended)

```yaml
services:
  mobile-db:
    image: ghcr.io/federicoserini/mobile-db:latest
    ports:
      - "8443:8443"
    volumes:
      - ./data:/data
    environment:
      ADMIN_PASSWORD_HASH: "$2y$10$..."   # see below
      JWT_SECRET: "at-least-32-bytes-change-me-!!"
      DB_ENCRYPTION_KEY: "at-least-32-bytes-change-me-!"
    restart: unless-stopped
```

Generate the admin password hash:

```bash
htpasswd -bnBC 10 '' 'yourpassword' | tr -d ':\n'
```

### Environment variables

| Variable | Required | Default | Description |
|---|---|---|---|
| `ADMIN_PASSWORD_HASH` | **Yes** | — | bcrypt hash of the admin password |
| `DB_ENCRYPTION_KEY` | **Yes** | — | ≥32 bytes, reserved for SQLCipher |
| `KEYCLOAK_URL` | **Yes** | — | Keycloak base URL, e.g. `https://gatekeeper.federicoserini.com` |
| `KEYCLOAK_REALM` | **Yes** | — | Realm name, e.g. `homelab` |
| `KEYCLOAK_CLIENT_ID` | No | — | Validates `azp` claim; leave empty to accept any client |
| `CRSQLITE_EXT_PATH` | No | `/app/crsqlite.so` | Path to cr-sqlite extension (pre-set in image) |
| `DATA_DIR` | No | `/data` | Directory for SQLite databases |
| `LISTEN_ADDR` | No | `:8443` | Bind address |
| `BACKUP_DESTINATION` | No | — | Backup path/URL; empty = disabled |
| `BACKUP_INTERVAL` | No | `24h` | Backup frequency (Go duration string) |
| `LOG_LEVEL` | No | `info` | `debug` / `info` / `warn` / `error` |
| `COMPACTION_THRESHOLD` | No | `1000` | Op-log entries before compaction runs |
| `ADMIN_ALLOWED_IPS` | No | all | Comma-separated IPs allowed to reach `/admin` |

### Keycloak setup

1. Create a realm in your Keycloak instance (e.g. `homelab`).
2. Create a client per app (e.g. `mobile-db-app`). Set **Access Type** → `confidential` or `public` depending on your client.
3. Mobile/web clients obtain a Keycloak access token via standard OIDC (authorization code or device flow).
4. Pass the token on every sync request: `Authorization: Bearer <keycloak_access_token>`.

The server fetches signing keys from `{KEYCLOAK_URL}/realms/{KEYCLOAK_REALM}/protocol/openid-connect/certs` and caches them for 10 minutes.

### Endpoints

| Path | Auth | Description |
|---|---|---|
| `GET /health` | none | Health check |
| `GET /metrics` | none | Prometheus metrics |
| `POST /devices/register` | none | Register a new device |
| `POST /auth/token` | device key (+ optional Keycloak token) | Verify device; returns `app_id`/`user_id` if token provided |
| `POST /sync` | device key + Keycloak token | Push / pull CRDT changes |
| `GET /events` | Keycloak token | SSE push stream |
| `GET /admin` | Basic Auth | Admin console |

## CI/CD

GitHub Actions builds and pushes multi-arch images (`linux/amd64`, `linux/arm64`) to GHCR on every push to `main` and on semver tags (`v*`).

Images are published at `ghcr.io/federicoserini/mobile-db`.

| Tag | When |
|---|---|
| `latest` | push to `main` |
| `sha-<short>` | every push |
| `v1.2.3` / `v1.2` | semver tag |

To update the cr-sqlite version, change `CRSQLITE_VERSION` in `.github/workflows/docker-build-push.yaml` and `container/Dockerfile`. Check available versions at [vlcn-io/cr-sqlite/releases](https://github.com/vlcn-io/cr-sqlite/releases).

## Building locally

```bash
# macOS / Linux (requires gcc for CGO)
go build -o mobile-db .

# Container (multi-arch)
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  --build-arg CRSQLITE_VERSION=0.16.1 \
  -f container/Dockerfile \
  -t mobile-db:local .
```

## Dev quickstart (macOS)

```bash
export ADMIN_PASSWORD_HASH='$2y$10$sEOqdKzPKd.Pj1kUjZwfk.ObV0dTDe85Tpe80aEBTHJDlurMFInEe'
export DB_ENCRYPTION_KEY='dev-enc-key-change-in-production-!!!!'
export KEYCLOAK_URL='https://gatekeeper.federicoserini.com'
export KEYCLOAK_REALM='homelab'
export CRSQLITE_EXT_PATH=./crsqlite.dylib
export LISTEN_ADDR=:8080

go run .
```

Password for the hash above: `devpassword`

Smoke test:

```bash
curl http://localhost:8080/health
curl http://localhost:8080/metrics
# Admin UI: http://localhost:8080/admin  (user: admin, pass: devpassword)
```
