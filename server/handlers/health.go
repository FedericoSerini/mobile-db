package handlers

import (
	"encoding/json"
	"net/http"
	"sync/atomic"
	"time"
)

var activeSSEConns atomic.Int64

func SSEConnect()    { activeSSEConns.Add(1) }
func SSEDisconnect() { activeSSEConns.Add(-1) }

type healthHandler struct {
	startTime time.Time
	version   string
	dbOK      func() bool
}

func NewHealthHandler(version string) *healthHandler {
	return &healthHandler{startTime: time.Now(), version: version, dbOK: func() bool { return true }}
}

func (h *healthHandler) WithDBCheck(fn func() bool) *healthHandler {
	h.dbOK = fn
	return h
}

func (h *healthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	dbStatus := "ok"
	if !h.dbOK() {
		dbStatus = "error"
	}
	resp := map[string]any{
		"status":                 "ok",
		"uptime_seconds":         int64(time.Since(h.startTime).Seconds()),
		"db_status":              dbStatus,
		"active_sse_connections": activeSSEConns.Load(),
		"version":                h.version,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
