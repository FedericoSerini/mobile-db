package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/federicoserini/mobile-db/server/auth"
	"github.com/federicoserini/mobile-db/server/middleware"
)

type stubDeviceSvc struct{ valid bool }

func (s *stubDeviceSvc) Verify(_ context.Context, _, _ string) error {
	if s.valid {
		return nil
	}
	return auth.ErrInvalidSecret
}

func okHandler(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }

func TestDeviceKeyMissingHeader(t *testing.T) {
	h := middleware.RequireDeviceKey(&stubDeviceSvc{valid: true})(http.HandlerFunc(okHandler))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/", nil))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rr.Code)
	}
}

func TestDeviceKeyValidHeader(t *testing.T) {
	h := middleware.RequireDeviceKey(&stubDeviceSvc{valid: true})(http.HandlerFunc(okHandler))
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Device-Key", "key-id:secret")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}
}

func TestJWTMiddlewareMissing(t *testing.T) {
	svc := auth.NewJWTService([]byte("32-byte-secret-for-testing-1234!"))
	h := middleware.RequireJWT(svc)(http.HandlerFunc(okHandler))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/", nil))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rr.Code)
	}
}

func TestJWTMiddlewareValid(t *testing.T) {
	svc := auth.NewJWTService([]byte("32-byte-secret-for-testing-1234!"))
	h := middleware.RequireJWT(svc)(http.HandlerFunc(okHandler))
	token, _ := svc.Issue("app1", "user1")
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}
}
