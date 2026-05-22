package http2_test

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	transport "github.com/federicoserini/mobile-db/transport/http2"
)

func TestBroadcasterNotify(t *testing.T) {
	b := transport.NewBroadcaster()
	ch := b.Subscribe("app1", "user1")
	defer b.Unsubscribe("app1", "user1", ch)

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
	b := transport.NewBroadcaster()
	handler := transport.SSEHandler(b, "app1", "user1")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		time.Sleep(50 * time.Millisecond)
		b.Notify(context.Background(), "app1", "user1")
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/events", nil).WithContext(ctx)
	handler(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, "data: sync") {
		t.Fatalf("expected SSE data line, got: %s", body)
	}
}
