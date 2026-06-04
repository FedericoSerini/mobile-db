package handlers

import (
	"context"
	"io"
	"net/http"

	"github.com/federicoserini/mobile-db/core"
	coresync "github.com/federicoserini/mobile-db/core/sync"
	"github.com/federicoserini/mobile-db/server/middleware"
)

// SyncEventWriter is satisfied by crsqlite.AdminEventStore.
type SyncEventWriter interface {
	WriteSyncEvent(ctx context.Context, appID, datasetID, userID, deviceKeyID string, opCount int) error
}

type SyncHandler struct {
	engine     *coresync.Engine
	codec      core.Codec
	compressor core.Compressor
	events     SyncEventWriter // nil = no-op
}

func NewSyncHandler(engine *coresync.Engine, codec core.Codec, comp core.Compressor) *SyncHandler {
	return &SyncHandler{engine: engine, codec: codec, compressor: comp}
}

func (h *SyncHandler) WithEventWriter(w SyncEventWriter) *SyncHandler {
	h.events = w
	return h
}

func (h *SyncHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	compressed, err := io.ReadAll(io.LimitReader(r.Body, 4<<20))
	defer r.Body.Close()
	if err != nil {
		http.Error(w, "read body: "+err.Error(), http.StatusBadRequest)
		return
	}
	if h.compressor == nil {
		http.Error(w, "server misconfigured: nil compressor", http.StatusInternalServerError)
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

	// Overwrite with values from auth middleware context (trusted)
	if appID, ok := r.Context().Value(middleware.CtxAppID).(string); ok && appID != "" {
		msg.AppID = appID
	}
	if userID, ok := r.Context().Value(middleware.CtxUserID).(string); ok && userID != "" {
		msg.UserID = userID
	}

	resp, err := h.engine.Sync(r.Context(), msg)
	if err != nil {
		http.Error(w, "sync: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Record sync event after successful merge (best-effort, never fail the request)
	if h.events != nil && len(msg.Ops) > 0 {
		dkID, _ := r.Context().Value(middleware.CtxDeviceKeyID).(string)
		_ = h.events.WriteSyncEvent(r.Context(), msg.AppID, msg.DatasetID, msg.UserID, dkID, len(msg.Ops))
	}

	encoded, err := h.codec.Encode(resp)
	if err != nil {
		http.Error(w, "encode: "+err.Error(), http.StatusInternalServerError)
		return
	}
	out, err := h.compressor.Compress(encoded)
	if err != nil {
		http.Error(w, "compress: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/cbor+zstd")
	w.WriteHeader(http.StatusOK)
	w.Write(out)
}
