# Running mobile-db

## Required env vars

| Var | Notes |
|---|---|
| `ADMIN_PASSWORD_HASH` | bcrypt hash — generate with `htpasswd -bnBC 10 '' 'yourpass' \| tr -d ':\n'` |
| `JWT_SECRET` | ≥32 bytes |
| `DB_ENCRYPTION_KEY` | ≥32 bytes |

## Optional env vars

| Var | Default | Notes |
|---|---|---|
| `CRSQLITE_EXT_PATH` | `./crsqlite.so` | macOS: use `./crsqlite.dylib` (already in repo root) |
| `DATA_DIR` | `./data` | SQLite files stored here |
| `LISTEN_ADDR` | `:8443` | |
| `BACKUP_DESTINATION` | `""` | Empty = disabled |
| `BACKUP_INTERVAL` | `24h` | |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `COMPACTION_THRESHOLD` | `1000` | Op-log entries before compaction |
| `ADMIN_ALLOWED_IPS` | all | Comma-separated, e.g. `127.0.0.1,::1` |

## Build & run

```bash
go build -o mobile-db .
./mobile-db
```

or without building:

```bash
go run .
```

## Dev quickstart (macOS)

```bash
export ADMIN_PASSWORD_HASH='$2y$10$sEOqdKzPKd.Pj1kUjZwfk.ObV0dTDe85Tpe80aEBTHJDlurMFInEe'
export JWT_SECRET='dev-jwt-secret-change-in-production-!!!'
export DB_ENCRYPTION_KEY='dev-enc-key-change-in-production-!!!!'
export CRSQLITE_EXT_PATH=./crsqlite.dylib
export LISTEN_ADDR=:8080

go run .
```

> Password for the hash above: `devpassword`

## Smoke test

```bash
curl http://localhost:8080/health
curl http://localhost:8080/metrics
```

## Admin UI

`http://localhost:8080/admin` — Basic Auth, user `admin`, password `devpassword`
