package handlers_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/federicoserini/mobile-db/compression/cbor"
	"github.com/federicoserini/mobile-db/compression/zstd"
	"github.com/federicoserini/mobile-db/core"
	"github.com/federicoserini/mobile-db/core/crdt"
	coresync "github.com/federicoserini/mobile-db/core/sync"
	"github.com/federicoserini/mobile-db/server/handlers"
	"github.com/federicoserini/mobile-db/server/middleware"
)

type nopNotifier struct{}

func (n *nopNotifier) Notify(_ context.Context, _, _ string) error { return nil }

func buildSyncHandler() http.Handler {
	store := core.NopStorage{}
	engine := coresync.NewEngine(store, &nopNotifier{}, crdt.NewClock("server"), 1000)
	return handlers.NewSyncHandler(engine, cbor.NewCodec(), zstd.NewCompressor())
}

func encodeMsg(t *testing.T, msg core.SyncMessage) []byte {
	t.Helper()
	codec := cbor.NewCodec()
	comp := zstd.NewCompressor()
	raw, err := codec.Encode(msg)
	if err != nil {
		t.Fatal(err)
	}
	out, err := comp.Compress(raw)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestSyncHandlerReturns200(t *testing.T) {
	h := buildSyncHandler()
	msg := core.SyncMessage{AppID: "app1", UserID: "user1", DatasetID: "ds1"}
	body := encodeMsg(t, msg)

	req := httptest.NewRequest("POST", "/sync", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/cbor+zstd")
	ctx := context.WithValue(req.Context(), middleware.CtxAppID, "app1")
	ctx = context.WithValue(ctx, middleware.CtxUserID, "user1")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body)
	}
	comp := zstd.NewCompressor()
	decompressed, err := comp.Decompress(rr.Body.Bytes())
	if err != nil {
		t.Fatalf("decompress response: %v", err)
	}
	var resp core.SyncResponse
	if err := cbor.NewCodec().Decode(decompressed, &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}
