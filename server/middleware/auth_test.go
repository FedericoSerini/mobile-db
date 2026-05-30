package middleware_test

import (
	"context"
	"errors"
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

type stubTokenValidator struct {
	claims *auth.OIDCClaims
	err    error
}

func (s *stubTokenValidator) Validate(_ string) (*auth.OIDCClaims, error) {
	return s.claims, s.err
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
	v := &stubTokenValidator{err: errors.New("invalid")}
	h := middleware.RequireJWT(v)(http.HandlerFunc(okHandler))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/", nil))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rr.Code)
	}
}

func TestJWTMiddlewareValid(t *testing.T) {
	v := &stubTokenValidator{claims: &auth.OIDCClaims{AppID: "app1"}}
	h := middleware.RequireJWT(v)(http.HandlerFunc(okHandler))
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer any-token")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}
}

func TestJWTMiddlewareInvalidToken(t *testing.T) {
	v := &stubTokenValidator{err: errors.New("bad")}
	h := middleware.RequireJWT(v)(http.HandlerFunc(okHandler))
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer bad-token")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rr.Code)
	}
}
