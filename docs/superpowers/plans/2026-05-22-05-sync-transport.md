# Sync Engine + HTTP/2 Transport Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the CBOR+Zstd codec, SSE broadcaster, sync handler (POST /sync), and wire the full sync flow end-to-end.

**Architecture:** `compression/cbor/` and `compression/zstd/` implement `core.Codec` and `core.Compressor`. `transport/http2/` implements `core.SyncTransport` using net/http SSE. `core/sync/engine.go` contains the transport-agnostic sync logic that glues CRDT core + storage backend together. HTTP handler in `server/` calls the engine.

**Tech Stack:** github.com/fxamacker/cbor/v2, github.com/klauspost/compress/zstd, net/http (stdlib). Depends on Plans 01–04.

**Prerequisite:** Plans 01–04 complete and passing.

---

### Task 1: CBOR codec

**Files:**
- Create: `compression/cbor/codec.go`
- Create: `compression/cbor/codec_test.go`

- [ ] **Step 1: Write failing tests**

```go
// compression/cbor/codec_test.go
package cbor_test

import (
	"testing"

	"github.com/yourusername/mobile-db/compression/cbor"
	"github.com/yourusername/mobile-db/core"
)

func TestCBORRoundTrip(t *testing.T) {
	c := cbor.NewCodec()
	msg := core.SyncMessage{
		AppID:     "app1",
		UserID:    "user1",
		DatasetID: "ds1",
		Clock:     core.HLC{WallTime: 1000, Logical: 2, DeviceID: "dev1"},
		Ops: []core.CRDTOp{
			{OpID: "op1", DocID: "doc1", Field: "name", Value: "Alice",
				Timestamp: core.HLC{WallTime: 900, DeviceID: "dev1"}},
		},
	}

	data, err := c.Encode(msg)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	var decoded core.SyncMessage
	if err := c.Decode(data, &decoded); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if decoded.AppID != "app1" || len(decoded.Ops) != 1 {
		t.Fatalf("decoded mismatch: %+v", decoded)
	}
}

func TestCBORInterfaceCompliance(t *testing.T) {
	var _ core.Codec = cbor.NewCodec()
}
```

- [ ] **Step 2: Run to verify fails**

```bash
go test ./compression/cbor/ -v
```

Expected: compile error `no Go files`.

- [ ] **Step 3: Write codec.go**

```go
// compression/cbor/codec.go
package cbor

import (
	gocbor "github.com/fxamacker/cbor/v2"
)

// Codec implements core.Codec using CBOR encoding.
type Codec struct {
	enc gocbor.EncMode
	dec gocbor.DecMode
}

func NewCodec() *Codec {
	enc, _ := gocbor.EncOptions{}.EncMode()
	dec, _ := gocbor.DecOptions{}.DecMode()
	return &Codec{enc: enc, dec: dec}
}

func (c *Codec) Encode(v any) ([]byte, error) {
	return c.enc.Marshal(v)
}

func (c *Codec) Decode(data []byte, v any) error {
	return c.dec.Unmarshal(data, v)
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./compression/cbor/ -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add compression/cbor/codec.go compression/cbor/codec_test.go
git commit -m "feat: CBOR codec implementing core.Codec"
```

---

### Task 2: Zstd compressor

**Files:**
- Create: `compression/zstd/compressor.go`
- Create: `compression/zstd/compressor_test.go`

- [ ] **Step 1: Write failing tests**

```go
// compression/zstd/compressor_test.go
package zstd_test

import (
	"bytes"
	"testing"

	"github.com/yourusername/mobile-db/compression/zstd"
	"github.com/yourusername/mobile-db/core"
)

func TestZstdRoundTrip(t *testing.T) {
	c := zstd.NewCompressor()
	original := []byte(`{"key":"value","number":42}`)

	compressed, err := c.Compress(original)
	if err != nil {
		t.Fatalf("Compress: %v", err)
	}
	if bytes.Equal(compressed, original) {
		t.Fatal("compressed data must differ from original")
	}

	decompressed, err := c.Decompress(compressed)
	if err != nil {
		t.Fatalf("Decompress: %v", err)
	}
	if !bytes.Equal(decompressed, original) {
		t.Fatalf("decompressed mismatch: want %s, got %s", original, decompressed)
	}
}

func TestZstdInterfaceCompliance(t *testing.T) {
	var _ core.Compressor = zstd.NewCompressor()
}
```

- [ ] **Step 2: Run to verify fails**

```bash
go test ./compression/zstd/ -v
```

Expected: compile error `no Go files`.

- [ ] **Step 3: Write compressor.go**

```go
// compression/zstd/compressor.go
package zstd

import (
	"fmt"

	gozstd "github.com/klauspost/compress/zstd"
)

// Compressor implements core.Compressor using Zstandard.
type Compressor struct {
	enc *gozstd.Encoder
	dec *gozstd.Decoder
}

func NewCompressor() *Compressor {
	enc, _ := gozstd.NewWriter(nil)
	dec, _ := gozstd.NewReader(nil)
	return &Compressor{enc: enc, dec: dec}
}

func (c *Compressor) Compress(data []byte) ([]byte, error) {
	return c.enc.EncodeAll(data, nil), nil
}

func (c *Compressor) Decompress(data []byte) ([]byte, error) {
	out, err := c.dec.DecodeAll(data, nil)
	if err != nil {
		return nil, fmt.Errorf("zstd decompress: %w", err)
	}
	return out, nil
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./compression/... -v
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add compression/zstd/compressor.go compression/zstd/compressor_test.go
git commit -m "feat: Zstd compressor implementing core.Compressor"
```

---

### Task 3: SSE broadcaster (server-push notifications)

**Files:**
- Create: `transport/http2/sse.go`
- Create: `transport/http2/sse_test.go`

- [ ] **Step 1: Write failing tests**

```go
// transport/http2/sse_test.go
package http2_test

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yourusername/mobile-db/transport/http2"
)

func TestBroadcasterNotify(t *testing.T) {
	b := http2.NewBroadcaster()

	// Register a subscriber
	ch := b.Subscribe("app1", "user1")
	defer b.Unsubscribe("app1", "user1", ch)

	// Notify
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := b.Notify(ctx, "app1", "user1"); err != nil {
		t.Fatalf("Notify: %v", err)
	}

	select {
	case msg := <-ch:
		if msg != "sync" {
			t.Fatalf("unexpected message: %s", msg)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for notification")
	}
}

func TestSSEHandlerWritesEvents(t *testing.T) {
	b := http2.NewBroadcaster()
	handler := http2.SSEHandler(b, "app1", "user1")

	rr := httptest.NewRecorder()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		time.Sleep(50 * time.Millisecond)
		b.Notify(ctx, "app1", "user1")
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	req := httptest.NewRequest("GET", "/events", nil).WithContext(ctx)
	handler(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, "data: sync") {
		t.Fatalf("expected SSE data line, got: %s", body)
	}
}
```

- [ ] **Step 2: Run to verify fails**

```bash
go test ./transport/http2/ -v
```

Expected: compile error `no Go files`.

- [ ] **Step 3: Write sse.go**

```go
// transport/http2/sse.go
package http2

import (
	"context"
	"fmt"
	"net/http"
	"sync"
)

// Broadcaster manages SSE subscriptions and sends sync-available notifications.
type Broadcaster struct {
	mu   sync.RWMutex
	subs map[string][]chan string // key: "appID/userID"
}

func NewBroadcaster() *Broadcaster {
	return &Broadcaster{subs: map[string][]chan string{}}
}

func key(appID, userID string) string { return appID + "/" + userID }

// Subscribe returns a channel that receives "sync" when new ops are available.
func (b *Broadcaster) Subscribe(appID, userID string) chan string {
	ch := make(chan string, 1)
	b.mu.Lock()
	k := key(appID, userID)
	b.subs[k] = append(b.subs[k], ch)
	b.mu.Unlock()
	return ch
}

// Unsubscribe removes a subscriber channel.
func (b *Broadcaster) Unsubscribe(appID, userID string, ch chan string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	k := key(appID, userID)
	subs := b.subs[k]
	for i, s := range subs {
		if s == ch {
			b.subs[k] = append(subs[:i], subs[i+1:]...)
			break
		}
	}
}

// Notify sends a sync signal to all subscribers for appID/userID.
func (b *Broadcaster) Notify(_ context.Context, appID, userID string) error {
	b.mu.RLock()
	subs := append([]chan string{}, b.subs[key(appID, userID)]...)
	b.mu.RUnlock()
	for _, ch := range subs {
		select {
		case ch <- "sync":
		default: // drop if subscriber is slow; it will pick up delta on next poll
		}
	}
	return nil
}

// SSEHandler returns an http.HandlerFunc that streams SSE events to the client.
func SSEHandler(b *Broadcaster, appID, userID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		ch := b.Subscribe(appID, userID)
		defer b.Unsubscribe(appID, userID, ch)

		for {
			select {
			case msg := <-ch:
				fmt.Fprintf(w, "data: %s\n\n", msg)
				flusher.Flush()
			case <-r.Context().Done():
				return
			}
		}
	}
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./transport/http2/ -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add transport/http2/sse.go transport/http2/sse_test.go
git commit -m "feat: SSE broadcaster for real-time sync notifications"
```

---

### Task 4: Sync engine (transport-agnostic)

**Files:**
- Create: `core/sync/engine.go`
- Create: `core/sync/engine_test.go`

- [ ] **Step 1: Write failing tests**

```go
// core/sync/engine_test.go
package sync_test

import (
	"context"
	"testing"

	"github.com/yourusername/mobile-db/core"
	coresync "github.com/yourusername/mobile-db/core/sync"
	"github.com/yourusername/mobile-db/core/crdt"
)

type captureStorage struct {
	core.NopStorage
	merged   []core.CRDTOp
	opCount  int
	compacted bool
}

func (s *captureStorage) MergeOps(_ context.Context, _, _, _ string, ops []core.CRDTOp) error {
	s.merged = append(s.merged, ops...)
	s.opCount += len(ops)
	return nil
}

func (s *captureStorage) GetDelta(_ context.Context, _, _, _ string, _ core.HLC) ([]core.CRDTOp, core.HLC, error) {
	return nil, core.HLC{WallTime: 999}, nil
}

func (s *captureStorage) OpCount(_ context.Context, _, _, _ string) (int, error) {
	return s.opCount, nil
}

func (s *captureStorage) Compact(_ context.Context, _, _, _ string) error {
	s.compacted = true
	return nil
}

type captureBroadcaster struct {
	notified bool
}

func (b *captureBroadcaster) Notify(_ context.Context, _, _ string) error {
	b.notified = true
	return nil
}

func TestEngineSyncMergesAndBroadcasts(t *testing.T) {
	store := &captureStorage{}
	bc := &captureBroadcaster{}
	clock := crdt.NewClock("server")
	eng := coresync.NewEngine(store, bc, clock, 1000)

	msg := core.SyncMessage{
		AppID: "app1", UserID: "user1", DatasetID: "ds1",
		Clock: core.HLC{},
		Ops: []core.CRDTOp{
			{OpID: "op1", DocID: "doc1", Field: "x", Value: "v",
				Timestamp: core.HLC{WallTime: 1, DeviceID: "dev1"}},
		},
	}

	resp, err := eng.Sync(context.Background(), msg)
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if len(store.merged) != 1 {
		t.Fatal("engine must call MergeOps with incoming ops")
	}
	if !bc.notified {
		t.Fatal("engine must broadcast after merge")
	}
	if resp.NewClock.WallTime == 0 {
		t.Fatal("response must include a non-zero clock")
	}
}
```

- [ ] **Step 2: Run to verify fails**

```bash
go test ./core/sync/ -v
```

Expected: compile error `no Go files`.

- [ ] **Step 3: Write engine.go**

```go
// core/sync/engine.go
package sync

import (
	"context"

	"github.com/yourusername/mobile-db/core"
	"github.com/yourusername/mobile-db/core/crdt"
)

// Notifier broadcasts sync-available signals to connected clients.
type Notifier interface {
	Notify(ctx context.Context, appID, userID string) error
}

// Engine is the transport-agnostic sync logic.
type Engine struct {
	store     core.StorageBackend
	notifier  Notifier
	clock     *crdt.Clock
	threshold int
}

func NewEngine(store core.StorageBackend, notifier Notifier, clock *crdt.Clock, compactionThreshold int) *Engine {
	return &Engine{store: store, notifier: notifier, clock: clock, threshold: compactionThreshold}
}

// Sync processes an incoming SyncMessage and returns the delta for the client.
func (e *Engine) Sync(ctx context.Context, msg core.SyncMessage) (*core.SyncResponse, error) {
	// Advance server clock
	serverClock := e.clock.Receive(msg.Clock)

	// Check schema version — if mismatch, return full snapshot instead of delta
	needsSnap, err := crdt.NeedsSnapshot(ctx, e.store, msg.AppID, msg.DatasetID, 0)
	if err != nil {
		return nil, err
	}

	var snapshot any
	if needsSnap {
		snap, _, err := e.store.GetSnapshot(ctx, msg.AppID, msg.UserID, msg.DatasetID)
		if err != nil {
			return nil, err
		}
		snapshot = snap
	}

	// Merge incoming ops
	if len(msg.Ops) > 0 {
		if err := e.store.MergeOps(ctx, msg.AppID, msg.UserID, msg.DatasetID, msg.Ops); err != nil {
			return nil, err
		}
		// Notify other devices for the same user
		_ = e.notifier.Notify(ctx, msg.AppID, msg.UserID)
		// Trigger compaction if needed
		_ = crdt.MaybeCompact(ctx, e.store, msg.AppID, msg.UserID, msg.DatasetID, e.threshold)
	}

	// Compute delta: ops the client has not seen
	delta, newClock, err := e.store.GetDelta(ctx, msg.AppID, msg.UserID, msg.DatasetID, msg.Clock)
	if err != nil {
		return nil, err
	}

	// Use server clock if storage clock is zero
	if newClock.WallTime == 0 {
		newClock = serverClock
	}

	schemaVer, _ := e.store.SchemaVersion(ctx, msg.AppID, msg.DatasetID)

	return &core.SyncResponse{
		NewClock:      newClock,
		Ops:           delta,
		Snapshot:      snapshot,
		SchemaVersion: schemaVer,
	}, nil
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./core/sync/ -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add core/sync/engine.go core/sync/engine_test.go
git commit -m "feat: transport-agnostic sync engine (merge, broadcast, compaction, delta)"
```

---

### Task 5: POST /sync HTTP handler

**Files:**
- Create: `server/handlers/sync.go`
- Create: `server/handlers/sync_test.go`

- [ ] **Step 1: Write failing tests**

```go
// server/handlers/sync_test.go
package handlers_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	gocbor "github.com/fxamacker/cbor/v2"
	"github.com/yourusername/mobile-db/compression/cbor"
	"github.com/yourusername/mobile-db/compression/zstd"
	"github.com/yourusername/mobile-db/core"
	coresync "github.com/yourusername/mobile-db/core/sync"
	"github.com/yourusername/mobile-db/core/crdt"
	"github.com/yourusername/mobile-db/server/handlers"
	"github.com/yourusername/mobile-db/server/middleware"
)

type nopNotifier struct{}

func (n *nopNotifier) Notify(_ context.Context, _, _ string) error { return nil }

func buildSyncHandler() http.Handler {
	store := core.NopStorage{}
	notifier := &nopNotifier{}
	clock := crdt.NewClock("server")
	engine := coresync.NewEngine(store, notifier, clock, 1000)
	codec := cbor.NewCodec()
	comp := zstd.NewCompressor()
	h := handlers.NewSyncHandler(engine, codec, comp)
	return h
}

func encodeRequest(t *testing.T, msg core.SyncMessage) []byte {
	t.Helper()
	raw, err := gocbor.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	// Compress with zstd
	comp := zstd.NewCompressor()
	out, _ := comp.Compress(raw)
	return out
}

func TestSyncHandlerReturns200(t *testing.T) {
	handler := buildSyncHandler()

	msg := core.SyncMessage{AppID: "app1", UserID: "user1", DatasetID: "ds1"}
	body := encodeRequest(t, msg)

	req := httptest.NewRequest("POST", "/sync", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/cbor+zstd")
	// Inject auth context (normally set by middleware)
	ctx := context.WithValue(req.Context(), middleware.CtxAppID, "app1")
	ctx = context.WithValue(ctx, middleware.CtxUserID, "user1")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body)
	}
	// Response should be CBOR+Zstd decodable
	comp := zstd.NewCompressor()
	decompressed, err := comp.Decompress(rr.Body.Bytes())
	if err != nil {
		t.Fatalf("decompress response: %v", err)
	}
	var resp core.SyncResponse
	if err := gocbor.Unmarshal(decompressed, &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}
```

- [ ] **Step 2: Run to verify fails**

```bash
go test ./server/handlers/ -run TestSyncHandler -v
```

Expected: compile error `no Go files`.

- [ ] **Step 3: Write sync.go handler**

```go
// server/handlers/sync.go
package handlers

import (
	"net/http"

	"github.com/yourusername/mobile-db/core"
	coresync "github.com/yourusername/mobile-db/core/sync"
	"github.com/yourusername/mobile-db/server/middleware"
)

// SyncHandler handles POST /sync.
type SyncHandler struct {
	engine     *coresync.Engine
	codec      core.Codec
	compressor core.Compressor
}

func NewSyncHandler(engine *coresync.Engine, codec core.Codec, comp core.Compressor) *SyncHandler {
	return &SyncHandler{engine: engine, codec: codec, compressor: comp}
}

func (h *SyncHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	compressed, err := readBody(r, 4<<20) // 4MB limit
	if err != nil {
		http.Error(w, "read body: "+err.Error(), http.StatusBadRequest)
		return
	}
	raw, err := h.compressor.Decompress(compressed)
	if err != nil {
		http.Error(w, "decompress: "+err.Error(), http.StatusBadRequest)
		return
	}

	var msg core.SyncMessage
	if err := h.codec.Decode(raw, &msg); err != nil {
		http.Error(w, "decode: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Trust context values set by auth middleware
	if appID, ok := r.Context().Value(middleware.CtxAppID).(string); ok {
		msg.AppID = appID
	}
	if userID, ok := r.Context().Value(middleware.CtxUserID).(string); ok {
		msg.UserID = userID
	}

	resp, err := h.engine.Sync(r.Context(), msg)
	if err != nil {
		http.Error(w, "sync: "+err.Error(), http.StatusInternalServerError)
		return
	}

	encoded, err := h.codec.Encode(resp)
	if err != nil {
		http.Error(w, "encode response: "+err.Error(), http.StatusInternalServerError)
		return
	}
	out, err := h.compressor.Compress(encoded)
	if err != nil {
		http.Error(w, "compress response: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/cbor+zstd")
	w.WriteHeader(http.StatusOK)
	w.Write(out)
}

func readBody(r *http.Request, limit int64) ([]byte, error) {
	buf := make([]byte, 0, 4096)
	lr := &limitedReader{r: r.Body, n: limit}
	chunk := make([]byte, 4096)
	for {
		n, err := lr.Read(chunk)
		buf = append(buf, chunk[:n]...)
		if err != nil {
			break
		}
	}
	return buf, nil
}

type limitedReader struct {
	r interface{ Read([]byte) (int, error) }
	n int64
}

func (l *limitedReader) Read(p []byte) (int, error) {
	if l.n <= 0 {
		return 0, http.ErrHandlerTimeout
	}
	if int64(len(p)) > l.n {
		p = p[:l.n]
	}
	n, err := l.r.Read(p)
	l.n -= int64(n)
	return n, err
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./server/handlers/ ./core/sync/ ./compression/... ./transport/... -v
```

Expected: all PASS.

- [ ] **Step 5: Run full suite**

```bash
go test ./... -v
```

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add server/handlers/sync.go server/handlers/sync_test.go
git commit -m "feat: POST /sync handler with CBOR+Zstd encode/decode"
```

---

### Task 6: Wire router

**Files:**
- Create: `server/router.go`

- [ ] **Step 1: Write router**

```go
// server/router.go
package server

import (
	"net/http"

	"github.com/yourusername/mobile-db/server/auth"
	"github.com/yourusername/mobile-db/server/handlers"
	"github.com/yourusername/mobile-db/server/middleware"
	"github.com/yourusername/mobile-db/transport/http2"
)

type RouterDeps struct {
	AuthHandlers  *auth.Handlers
	SyncHandler   *handlers.SyncHandler
	Broadcaster   *http2.Broadcaster
	JWTSvc        *auth.JWTService
	DeviceSvc     *auth.DeviceService
	AdminPassHash string
	AdminIPs      []string
}

func NewRouter(d RouterDeps) http.Handler {
	mux := http.NewServeMux()

	// Public auth endpoints
	mux.HandleFunc("POST /devices/register", d.AuthHandlers.Register)
	mux.HandleFunc("POST /auth/token", d.AuthHandlers.Token)
	mux.HandleFunc("POST /devices/rotate-key", d.AuthHandlers.RotateKey)

	// Device-key protected
	deviceAuth := middleware.RequireDeviceKey(d.DeviceSvc)
	jwtAuth := middleware.RequireJWT(d.JWTSvc)

	// DELETE /devices/{id} — device key required
	mux.Handle("DELETE /devices/", deviceAuth(http.HandlerFunc(d.AuthHandlers.RevokeDevice)))

	// POST /sync — device key + JWT required
	syncChain := deviceAuth(jwtAuth(d.SyncHandler))
	mux.Handle("POST /sync", syncChain)

	// GET /events — JWT required (SSE)
	mux.Handle("GET /events", jwtAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		appID, _ := r.Context().Value(middleware.CtxAppID).(string)
		userID, _ := r.Context().Value(middleware.CtxUserID).(string)
		http2.SSEHandler(d.Broadcaster, appID, userID)(w, r)
	})))

	return mux
}
```

- [ ] **Step 2: Build**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add server/router.go
git commit -m "feat: HTTP router wiring all sync and auth endpoints"
```
