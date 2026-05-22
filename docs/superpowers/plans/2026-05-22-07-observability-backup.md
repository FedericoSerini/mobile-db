# Observability + Backup + Final Wiring Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add structured logging (zerolog), /health endpoint, Prometheus /metrics endpoint, SQLite backup cron job, and wire everything into a runnable binary.

**Architecture:** Logging middleware in `server/middleware/`. Health + metrics handlers in `server/handlers/`. Backup runner in `server/backup/`. Final wiring in `main.go` using `server.Config`.

**Tech Stack:** github.com/rs/zerolog, github.com/prometheus/client_golang, Go stdlib (net/http, os, time). Depends on Plans 01–06.

**Prerequisite:** Plans 01–06 complete and passing.

---

### Task 1: Zerolog middleware

**Files:**
- Create: `server/middleware/logging.go`
- Create: `server/middleware/logging_test.go`

- [ ] **Step 1: Write failing tests**

```go
// server/middleware/logging_test.go
package middleware_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"
	"github.com/yourusername/mobile-db/server/middleware"
)

func TestLoggingMiddlewareLogsRequest(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf)

	handler := middleware.RequestLogger(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/sync", nil)
	req.Header.Set("X-Device-Key", "key-id:secret")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	var entry map[string]any
	if err := json.NewDecoder(&buf).Decode(&entry); err != nil {
		t.Fatalf("log entry not valid JSON: %v\nraw: %s", err, buf.String())
	}
	if entry["method"] != "GET" {
		t.Fatalf("missing method in log: %v", entry)
	}
	if entry["path"] != "/sync" {
		t.Fatalf("missing path in log: %v", entry)
	}
	if _, ok := entry["duration_ms"]; !ok {
		t.Fatal("log entry must contain duration_ms")
	}
	if entry["status"] != float64(200) {
		t.Fatalf("unexpected status: %v", entry)
	}
}
```

- [ ] **Step 2: Run to verify fails**

```bash
go test ./server/middleware/ -run TestLogging -v
```

Expected: compile error `undefined: middleware.RequestLogger`.

- [ ] **Step 3: Write logging.go**

```go
// server/middleware/logging.go
package middleware

import (
	"net/http"
	"time"

	"github.com/rs/zerolog"
)

// responseWriter wraps http.ResponseWriter to capture status code.
type responseWriter struct {
	http.ResponseWriter
	code int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.code = code
	rw.ResponseWriter.WriteHeader(code)
}

// RequestLogger returns a middleware that logs every request as structured JSON.
func RequestLogger(log zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rw := &responseWriter{ResponseWriter: w, code: http.StatusOK}
			next.ServeHTTP(rw, r)

			log.Info().
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Int("status", rw.code).
				Int64("duration_ms", time.Since(start).Milliseconds()).
				Str("remote_addr", r.RemoteAddr).
				Msg("request")
		})
	}
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./server/middleware/ -run TestLogging -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add server/middleware/logging.go server/middleware/logging_test.go
git commit -m "feat: zerolog request logging middleware (method, path, status, duration_ms)"
```

---

### Task 2: /health endpoint

**Files:**
- Create: `server/handlers/health.go`
- Create: `server/handlers/health_test.go`

- [ ] **Step 1: Write failing tests**

```go
// server/handlers/health_test.go
package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yourusername/mobile-db/server/handlers"
)

func TestHealthEndpoint(t *testing.T) {
	h := handlers.NewHealthHandler("1.0.0")

	req := httptest.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}

	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("response not valid JSON: %v", err)
	}
	if resp["status"] != "ok" {
		t.Fatalf("want status=ok, got %v", resp["status"])
	}
	if resp["version"] != "1.0.0" {
		t.Fatalf("want version=1.0.0, got %v", resp["version"])
	}
	if _, ok := resp["uptime_seconds"]; !ok {
		t.Fatal("response must include uptime_seconds")
	}
}
```

- [ ] **Step 2: Run to verify fails**

```bash
go test ./server/handlers/ -run TestHealth -v
```

Expected: compile error `undefined: handlers.NewHealthHandler`.

- [ ] **Step 3: Write health.go**

```go
// server/handlers/health.go
package handlers

import (
	"encoding/json"
	"net/http"
	"sync/atomic"
	"time"
)

var activeSSEConnections atomic.Int64

// SSEConnect increments the active SSE connection counter.
func SSEConnect()    { activeSSEConnections.Add(1) }
// SSEDisconnect decrements the active SSE connection counter.
func SSEDisconnect() { activeSSEConnections.Add(-1) }

type healthHandler struct {
	startTime time.Time
	version   string
	dbOK      func() bool
}

// NewHealthHandler returns a handler for GET /health.
// dbOK is called on each request to check database liveness; pass nil to always report ok.
func NewHealthHandler(version string) *healthHandler {
	return &healthHandler{startTime: time.Now(), version: version, dbOK: func() bool { return true }}
}

// WithDBCheck attaches a database liveness function.
func (h *healthHandler) WithDBCheck(fn func() bool) *healthHandler {
	h.dbOK = fn
	return h
}

func (h *healthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	dbStatus := "ok"
	if h.dbOK != nil && !h.dbOK() {
		dbStatus = "error"
	}

	resp := map[string]any{
		"status":                 "ok",
		"uptime_seconds":         int64(time.Since(h.startTime).Seconds()),
		"db_status":              dbStatus,
		"active_sse_connections": activeSSEConnections.Load(),
		"version":                h.version,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./server/handlers/ -run TestHealth -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add server/handlers/health.go server/handlers/health_test.go
git commit -m "feat: /health endpoint (uptime, db_status, sse_connections, version)"
```

---

### Task 3: Prometheus /metrics endpoint

**Files:**
- Create: `server/handlers/metrics.go`
- Create: `server/handlers/metrics_test.go`

- [ ] **Step 1: Write failing tests**

```go
// server/handlers/metrics_test.go
package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yourusername/mobile-db/server/handlers"
)

func TestMetricsEndpointRegistersCounters(t *testing.T) {
	reg := handlers.NewMetricsRegistry()
	h := handlers.NewMetricsHandler(reg)

	// Record a sync request
	reg.SyncRequests.WithLabelValues("app1").Inc()

	req := httptest.NewRequest("GET", "/metrics", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "mobiledb_sync_requests_total") {
		t.Fatal("metrics must include mobiledb_sync_requests_total")
	}
}
```

- [ ] **Step 2: Run to verify fails**

```bash
go test ./server/handlers/ -run TestMetrics -v
```

Expected: compile error `undefined: handlers.NewMetricsRegistry`.

- [ ] **Step 3: Write metrics.go**

```go
// server/handlers/metrics.go
package handlers

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MetricsRegistry holds all Prometheus metrics.
type MetricsRegistry struct {
	SyncRequests     *prometheus.CounterVec
	CRDTOpsTotal     prometheus.Counter
	ActiveDevices    prometheus.Gauge
	SSEConnections   prometheus.Gauge
	CompactionRuns   prometheus.Counter
	DBSizeBytes      *prometheus.GaugeVec
	reg              *prometheus.Registry
}

// NewMetricsRegistry creates and registers all metrics.
func NewMetricsRegistry() *MetricsRegistry {
	reg := prometheus.NewRegistry()

	m := &MetricsRegistry{reg: reg}

	m.SyncRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "mobiledb_sync_requests_total",
		Help: "Total sync requests by app_id.",
	}, []string{"app_id"})

	m.CRDTOpsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "mobiledb_crdt_ops_total",
		Help: "Total CRDT ops merged.",
	})

	m.ActiveDevices = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "mobiledb_active_devices",
		Help: "Number of active (non-revoked) devices.",
	})

	m.SSEConnections = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "mobiledb_sse_connections",
		Help: "Number of active SSE connections.",
	})

	m.CompactionRuns = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "mobiledb_compaction_runs_total",
		Help: "Total compaction runs.",
	})

	m.DBSizeBytes = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mobiledb_db_size_bytes",
		Help: "SQLite database file size per tenant.",
	}, []string{"app_id"})

	reg.MustRegister(
		m.SyncRequests, m.CRDTOpsTotal, m.ActiveDevices,
		m.SSEConnections, m.CompactionRuns, m.DBSizeBytes,
	)
	return m
}

// NewMetricsHandler returns an http.Handler serving Prometheus text format.
func NewMetricsHandler(m *MetricsRegistry) http.Handler {
	return promhttp.HandlerFor(m.reg, promhttp.HandlerOpts{})
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./server/handlers/ -v
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add server/handlers/metrics.go server/handlers/metrics_test.go
git commit -m "feat: Prometheus /metrics endpoint with all mobiledb_ counters"
```

---

### Task 4: SQLite backup cron

**Files:**
- Create: `server/backup/backup.go`
- Create: `server/backup/backup_test.go`

- [ ] **Step 1: Write failing tests**

```go
// server/backup/backup_test.go
package backup_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yourusername/mobile-db/server/backup"
)

func TestLocalBackupCopiesFile(t *testing.T) {
	src := filepath.Join(t.TempDir(), "data.db")
	dst := filepath.Join(t.TempDir(), "backups")

	// Create a dummy source file
	if err := os.WriteFile(src, []byte("sqlite-data"), 0600); err != nil {
		t.Fatal(err)
	}

	b := backup.NewRunner(backup.Config{
		SourcePaths: []string{src},
		Destination: dst,
		Interval:    time.Hour,
	})

	if err := b.RunOnce(); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	// Verify a backup file was written
	entries, err := os.ReadDir(dst)
	if err != nil {
		t.Fatalf("read backup dir: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("backup dir must contain at least one file after RunOnce")
	}
}

func TestBackupRunnerStops(t *testing.T) {
	b := backup.NewRunner(backup.Config{
		SourcePaths: []string{},
		Destination: t.TempDir(),
		Interval:    time.Hour,
	})
	done := make(chan struct{})
	go func() {
		b.Start()
		close(done)
	}()
	b.Stop()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Stop must cause Start to return within 1 second")
	}
}
```

- [ ] **Step 2: Run to verify fails**

```bash
go test ./server/backup/ -v
```

Expected: compile error `no Go files`.

- [ ] **Step 3: Write backup.go**

```go
// server/backup/backup.go
package backup

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// Config holds backup configuration.
type Config struct {
	SourcePaths []string // SQLite DB files to back up
	Destination string   // local directory path or "s3://bucket/prefix"
	Interval    time.Duration
}

// Runner runs periodic SQLite backups using file copy (safe: WAL mode).
type Runner struct {
	cfg  Config
	stop chan struct{}
}

func NewRunner(cfg Config) *Runner {
	return &Runner{cfg: cfg, stop: make(chan struct{})}
}

// Start blocks until Stop is called, running backups at the configured interval.
func (r *Runner) Start() {
	ticker := time.NewTicker(r.cfg.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			_ = r.RunOnce()
		case <-r.stop:
			return
		}
	}
}

// Stop signals the runner to exit.
func (r *Runner) Stop() {
	select {
	case r.stop <- struct{}{}:
	default:
	}
}

// RunOnce performs a single backup run for all source paths.
func (r *Runner) RunOnce() error {
	timestamp := time.Now().UTC().Format("2006-01-02T15-04-05Z")
	for _, src := range r.cfg.SourcePaths {
		if err := r.backupFile(src, timestamp); err != nil {
			return err
		}
	}
	return nil
}

func (r *Runner) backupFile(src, timestamp string) error {
	dst := r.cfg.Destination
	if err := os.MkdirAll(dst, 0700); err != nil {
		return fmt.Errorf("mkdir backup dest: %w", err)
	}
	name := fmt.Sprintf("%s_%s.db", filepath.Base(src), timestamp)
	dstPath := filepath.Join(dst, name)
	return copyFile(src, dstPath)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./server/backup/ -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add server/backup/backup.go server/backup/backup_test.go
git commit -m "feat: periodic SQLite backup runner with configurable interval and destination"
```

---

### Task 5: Final main.go wiring

**Files:**
- Modify: `main.go`
- Create: `server/server.go`

- [ ] **Step 1: Write server.go**

```go
// server/server.go
package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"github.com/rs/zerolog"
	"github.com/yourusername/mobile-db/admin"
	"github.com/yourusername/mobile-db/compression/cbor"
	"github.com/yourusername/mobile-db/compression/zstd"
	"github.com/yourusername/mobile-db/core/crdt"
	coresync "github.com/yourusername/mobile-db/core/sync"
	"github.com/yourusername/mobile-db/server/auth"
	"github.com/yourusername/mobile-db/server/backup"
	"github.com/yourusername/mobile-db/server/handlers"
	"github.com/yourusername/mobile-db/server/middleware"
	"github.com/yourusername/mobile-db/storage/crsqlite"
	"github.com/yourusername/mobile-db/transport/http2"
)

// Run initializes all components and starts the HTTP server. Blocks until ctx is cancelled.
func Run(ctx context.Context, cfg *Config, log zerolog.Logger) error {
	// Storage
	metaPath := cfg.DataDir + "/meta.db"
	dataDir := cfg.DataDir + "/apps"
	encKey := string(cfg.DBEncryptionKey)

	store, err := crsqlite.NewBackend(metaPath, dataDir, encKey, cfg.CRSQLiteExtPath)
	if err != nil {
		return fmt.Errorf("storage init: %w", err)
	}
	defer store.Close()

	// Auth services
	metaDB, err := crsqlite.OpenMeta(metaPath, encKey, cfg.CRSQLiteExtPath)
	if err != nil {
		return fmt.Errorf("meta db: %w", err)
	}
	defer metaDB.Close()

	metaStore := crsqlite.NewAuthStore(metaDB)
	deviceSvc := auth.NewDeviceService(metaStore)
	jwtSvc := auth.NewJWTService(cfg.JWTSecret)
	refreshSvc := auth.NewRefreshService(metaStore, jwtSvc)
	authHandlers := auth.NewHandlers(deviceSvc, jwtSvc, refreshSvc)

	// Sync engine
	broadcaster := http2.NewBroadcaster()
	clock := crdt.NewClock("server")
	engine := coresync.NewEngine(store, broadcaster, clock, cfg.CompactionThreshold)

	// Codec + compressor
	codec := cbor.NewCodec()
	comp := zstd.NewCompressor()

	// Handlers
	metricsReg := handlers.NewMetricsRegistry()
	syncHandler := handlers.NewSyncHandler(engine, codec, comp)
	healthHandler := handlers.NewHealthHandler("1.0.0").WithDBCheck(func() bool {
		return store != nil
	})
	metricsHandler := handlers.NewMetricsHandler(metricsReg)

	// Admin console
	tmpl, err := admin.LoadTemplates()
	if err != nil {
		return fmt.Errorf("load admin templates: %w", err)
	}
	adminDataStore := crsqlite.NewAdminDataStore(store)
	adminRouter := admin.NewAdminRouter(admin.AdminRouterDeps{
		PasswordHash: cfg.AdminPasswordHash,
		AllowedIPs:   cfg.AdminAllowedIPs,
		DeviceStore:  crsqlite.NewAdminDeviceStore(metaDB),
		DataStore:    adminDataStore,
		SyncStore:    crsqlite.NewAdminSyncStore(store),
		Tmpl:         tmpl,
	})

	// Main router
	syncDeps := RouterDeps{
		AuthHandlers:  authHandlers,
		SyncHandler:   syncHandler,
		Broadcaster:   broadcaster,
		JWTSvc:        jwtSvc,
		DeviceSvc:     deviceSvc,
		AdminPassHash: cfg.AdminPasswordHash,
		AdminIPs:      cfg.AdminAllowedIPs,
	}
	mux := http.NewServeMux()
	mux.Handle("/admin/", adminRouter)
	mux.HandleFunc("/health", healthHandler.ServeHTTP)
	mux.Handle("/metrics", middleware.RequireAdminAuth(cfg.AdminPasswordHash, cfg.AdminAllowedIPs)(metricsHandler))
	mux.Handle("/", NewRouter(syncDeps))

	logged := middleware.RequestLogger(log)(mux)

	// Backup
	backupRunner := backup.NewRunner(backup.Config{
		SourcePaths: []string{metaPath},
		Destination: cfg.BackupDestination,
		Interval:    24 * time.Hour,
	})
	if cfg.BackupDestination != "" {
		go backupRunner.Start()
		defer backupRunner.Stop()
	}

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           logged,
		ReadHeaderTimeout: 10 * time.Second,
		TLSConfig:         &tls.Config{MinVersion: tls.VersionTLS13},
	}

	log.Info().Str("addr", cfg.ListenAddr).Msg("mobile-db listening")

	errCh := make(chan error, 1)
	go func() {
		// For local dev without TLS cert: use srv.ListenAndServe()
		// For production: srv.ListenAndServeTLS(certFile, keyFile)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutCtx)
	}
}
```

- [ ] **Step 2: Update main.go**

```go
// main.go
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"
	"github.com/yourusername/mobile-db/server"
)

func main() {
	log := zerolog.New(os.Stdout).With().Timestamp().Logger()

	cfg, err := server.LoadConfig()
	if err != nil {
		log.Fatal().Err(err).Msg("config")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := server.Run(ctx, cfg, log); err != nil {
		log.Fatal().Err(err).Msg("server stopped")
	}
}
```

- [ ] **Step 3: Build**

```bash
go build -o mobile-db .
```

Expected: binary created with no errors.

- [ ] **Step 4: Commit**

```bash
git add server/server.go main.go
git commit -m "feat: full server wiring — storage, auth, sync, admin, observability, backup"
```

---

### Task 6: AuthStore + AdminStore adapter on MetaDB

**Files:**
- Create: `storage/crsqlite/authstore.go`
- Create: `storage/crsqlite/adminstore.go`

These adapters connect the auth and admin packages to the concrete MetaDB / Backend.

- [ ] **Step 1: Write authstore.go**

```go
// storage/crsqlite/authstore.go
package crsqlite

import (
	"context"
	"database/sql"
	"time"

	"github.com/yourusername/mobile-db/server/auth"
)

// AuthStore implements auth.AuthStore + auth.RefreshStore backed by MetaDB.
type AuthStore struct{ db *MetaDB }

func NewAuthStore(db *MetaDB) *AuthStore { return &AuthStore{db: db} }

func (s *AuthStore) CreateDeviceKey(ctx context.Context, dk auth.DeviceKey) error {
	_, err := s.db.db.ExecContext(ctx,
		`INSERT INTO device_keys (device_key_id, device_id, app_id, key_hash, status, registered_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		dk.DeviceKeyID, dk.DeviceID, dk.AppID, dk.KeyHash, dk.Status, dk.RegisteredAt.UTC())
	return err
}

func (s *AuthStore) GetDeviceKey(ctx context.Context, deviceKeyID string) (*auth.DeviceKey, error) {
	var dk auth.DeviceKey
	var lastSeen sql.NullTime
	err := s.db.db.QueryRowContext(ctx,
		`SELECT device_key_id, device_id, app_id, key_hash, status, registered_at, last_seen_at
		 FROM device_keys WHERE device_key_id = ?`, deviceKeyID).
		Scan(&dk.DeviceKeyID, &dk.DeviceID, &dk.AppID, &dk.KeyHash, &dk.Status, &dk.RegisteredAt, &lastSeen)
	if err == sql.ErrNoRows {
		return nil, auth.ErrDeviceNotFound
	}
	if err != nil {
		return nil, err
	}
	if lastSeen.Valid {
		t := lastSeen.Time
		dk.LastSeenAt = &t
	}
	return &dk, nil
}

func (s *AuthStore) UpdateDeviceKey(ctx context.Context, dk auth.DeviceKey) error {
	now := time.Now().UTC()
	_, err := s.db.db.ExecContext(ctx,
		`UPDATE device_keys SET key_hash=?, status=?, last_seen_at=? WHERE device_key_id=?`,
		dk.KeyHash, dk.Status, now, dk.DeviceKeyID)
	return err
}

func (s *AuthStore) CreateRefreshToken(ctx context.Context, rt auth.RefreshToken) error {
	_, err := s.db.db.ExecContext(ctx,
		`INSERT INTO refresh_tokens (token_hash, app_id, user_id, expires_at, used) VALUES (?,?,?,?,0)`,
		rt.TokenHash, rt.AppID, rt.UserID, rt.ExpiresAt.UTC())
	return err
}

func (s *AuthStore) GetAndInvalidateRefreshToken(ctx context.Context, tokenHash string) (*auth.RefreshToken, error) {
	tx, err := s.db.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var rt auth.RefreshToken
	err = tx.QueryRowContext(ctx,
		`SELECT token_hash, app_id, user_id, expires_at, used FROM refresh_tokens WHERE token_hash=?`,
		tokenHash).Scan(&rt.TokenHash, &rt.AppID, &rt.UserID, &rt.ExpiresAt, &rt.Used)
	if err == sql.ErrNoRows || rt.Used {
		return nil, auth.ErrRefreshTokenInvalid
	}
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE refresh_tokens SET used=1 WHERE token_hash=?`, tokenHash); err != nil {
		return nil, err
	}
	return &rt, tx.Commit()
}

func (s *AuthStore) RevokeAllForUser(ctx context.Context, appID, userID string) error {
	_, err := s.db.db.ExecContext(ctx,
		`UPDATE refresh_tokens SET used=1 WHERE app_id=? AND user_id=?`, appID, userID)
	return err
}
```

- [ ] **Step 2: Write adminstore.go**

```go
// storage/crsqlite/adminstore.go
package crsqlite

import (
	"context"
	"time"

	"github.com/yourusername/mobile-db/admin"
	"github.com/yourusername/mobile-db/server/auth"
)

// AdminDeviceStore implements admin.DeviceAdminStore.
type AdminDeviceStore struct{ db *MetaDB }

func NewAdminDeviceStore(db *MetaDB) *AdminDeviceStore { return &AdminDeviceStore{db: db} }

func (s *AdminDeviceStore) ListDevices(ctx context.Context, appID string) ([]auth.DeviceKey, error) {
	q := `SELECT device_key_id, device_id, app_id, key_hash, status, registered_at FROM device_keys`
	args := []any{}
	if appID != "" {
		q += ` WHERE app_id=?`
		args = append(args, appID)
	}
	rows, err := s.db.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var devices []auth.DeviceKey
	for rows.Next() {
		var dk auth.DeviceKey
		rows.Scan(&dk.DeviceKeyID, &dk.DeviceID, &dk.AppID, &dk.KeyHash, &dk.Status, &dk.RegisteredAt)
		devices = append(devices, dk)
	}
	return devices, rows.Err()
}

func (s *AdminDeviceStore) RevokeDevice(ctx context.Context, deviceKeyID string) error {
	_, err := s.db.db.ExecContext(ctx,
		`UPDATE device_keys SET status='revoked' WHERE device_key_id=?`, deviceKeyID)
	return err
}

// AdminDataStore implements admin.DataAdminStore.
type AdminDataStore struct{ b *Backend }

func NewAdminDataStore(b *Backend) *AdminDataStore { return &AdminDataStore{b: b} }

func (s *AdminDataStore) ListDatasets(ctx context.Context, appID string) ([]string, error) {
	rows, err := s.b.meta.db.QueryContext(ctx,
		`SELECT dataset_id FROM datasets WHERE app_id=?`, appID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var datasets []string
	for rows.Next() {
		var id string
		rows.Scan(&id)
		datasets = append(datasets, id)
	}
	return datasets, rows.Err()
}

func (s *AdminDataStore) ListDocs(ctx context.Context, appID, datasetID string) ([]map[string]any, error) {
	d, err := s.b.appDB(appID)
	if err != nil {
		return nil, err
	}
	rows, err := d.db.QueryContext(ctx,
		"SELECT doc_id, data FROM snapshots WHERE dataset_id=?", datasetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var docs []map[string]any
	for rows.Next() {
		var docID, data string
		rows.Scan(&docID, &data)
		docs = append(docs, map[string]any{"doc_id": docID, "data": data})
	}
	return docs, rows.Err()
}

func (s *AdminDataStore) DeleteDoc(ctx context.Context, appID, datasetID, docID string) error {
	d, err := s.b.appDB(appID)
	if err != nil {
		return err
	}
	_, err = d.db.ExecContext(ctx,
		"DELETE FROM snapshots WHERE dataset_id=? AND doc_id=?", datasetID, docID)
	return err
}

// AdminSyncStore implements admin.SyncAdminStore.
type AdminSyncStore struct{ b *Backend }

func NewAdminSyncStore(b *Backend) *AdminSyncStore { return &AdminSyncStore{b: b} }

func (s *AdminSyncStore) ListSyncActivity(ctx context.Context) ([]admin.SyncEntry, error) {
	rows, err := s.b.meta.db.QueryContext(ctx,
		`SELECT app_id, dataset_id FROM datasets ORDER BY app_id, dataset_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entries []admin.SyncEntry
	for rows.Next() {
		var appID, datasetID string
		rows.Scan(&appID, &datasetID)
		count, _ := s.b.OpCount(ctx, appID, "", datasetID)
		entries = append(entries, admin.SyncEntry{
			AppID:     appID,
			DatasetID: datasetID,
			OpCount:   count,
			LastSync:  time.Now(), // TODO: track in a sync_log table in v2
		})
	}
	return entries, rows.Err()
}
```

- [ ] **Step 3: Build full binary**

```bash
go build -o mobile-db .
```

Expected: binary created.

- [ ] **Step 4: Run full test suite**

```bash
go test ./... -v
```

Expected: all PASS (storage tests require `CRSQLITE_EXT_PATH` set; skip otherwise).

- [ ] **Step 5: Commit**

```bash
git add storage/crsqlite/authstore.go storage/crsqlite/adminstore.go
git commit -m "feat: AuthStore and AdminStore adapters connecting auth/admin to crsqlite"
```

---

### Task 7: Smoke test — binary starts and /health responds

- [ ] **Step 1: Set env vars**

```bash
export ADMIN_PASSWORD_HASH=$(htpasswd -bnBC 12 "" "adminpass" | tr -d ':\n' | sed 's/$2y/$2a/')
export JWT_SECRET="$(openssl rand -hex 32)"
export DB_ENCRYPTION_KEY="$(openssl rand -hex 16)"
export CRSQLITE_EXT_PATH="./crsqlite.dylib"
export DATA_DIR="./data-test"
export LISTEN_ADDR=":9000"
```

- [ ] **Step 2: Run binary**

```bash
./mobile-db &
sleep 1
```

Expected: `mobile-db listening addr=:9000` logged to stdout.

- [ ] **Step 3: Hit /health**

```bash
curl -s http://localhost:9000/health | jq .
```

Expected:
```json
{
  "status": "ok",
  "uptime_seconds": 1,
  "db_status": "ok",
  "active_sse_connections": 0,
  "version": "1.0.0"
}
```

- [ ] **Step 4: Kill and clean up**

```bash
kill %1
rm -rf ./data-test ./mobile-db
```

- [ ] **Step 5: Final commit**

```bash
git add .
git commit -m "chore: smoke test verified — binary starts, /health responds"
```
