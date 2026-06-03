package http2

import (
	"context"
	"fmt"
	"net/http"
	"sync"
)

type Broadcaster struct {
	mu   sync.RWMutex
	subs map[string][]chan string
}

func NewBroadcaster() *Broadcaster {
	return &Broadcaster{subs: map[string][]chan string{}}
}

func subKey(appID, userID string) string { return appID + "/" + userID }

func (b *Broadcaster) Subscribe(appID, userID string) chan string {
	ch := make(chan string, 1)
	b.mu.Lock()
	k := subKey(appID, userID)
	b.subs[k] = append(b.subs[k], ch)
	b.mu.Unlock()
	return ch
}

func (b *Broadcaster) Unsubscribe(appID, userID string, ch chan string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	k := subKey(appID, userID)
	subs := b.subs[k]
	for i, s := range subs {
		if s == ch {
			b.subs[k] = append(subs[:i], subs[i+1:]...)
			return
		}
	}
}

func (b *Broadcaster) Notify(_ context.Context, appID, userID string) error {
	b.mu.RLock()
	subs := append([]chan string{}, b.subs[subKey(appID, userID)]...)
	b.mu.RUnlock()
	for _, ch := range subs {
		select {
		case ch <- "sync":
		default:
		}
	}
	return nil
}

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
		w.WriteHeader(http.StatusOK)
		flusher.Flush()

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
