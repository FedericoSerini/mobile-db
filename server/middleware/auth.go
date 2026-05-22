package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/federicoserini/mobile-db/server/auth"
)

type contextKey string

const (
	CtxDeviceKeyID contextKey = "device_key_id"
	CtxAppID       contextKey = "app_id"
	CtxUserID      contextKey = "user_id"
)

type DeviceVerifier interface {
	Verify(ctx context.Context, deviceKeyID, secret string) error
}

// RequireDeviceKey validates X-Device-Key: <id>:<secret> header.
func RequireDeviceKey(svc DeviceVerifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("X-Device-Key")
			if header == "" {
				http.Error(w, "missing X-Device-Key", http.StatusUnauthorized)
				return
			}
			parts := strings.SplitN(header, ":", 2)
			if len(parts) != 2 {
				http.Error(w, "malformed X-Device-Key", http.StatusUnauthorized)
				return
			}
			if err := svc.Verify(r.Context(), parts[0], parts[1]); err != nil {
				http.Error(w, "invalid device key", http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), CtxDeviceKeyID, parts[0])
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireJWT validates Authorization: Bearer <token> header.
func RequireJWT(svc *auth.JWTService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if !strings.HasPrefix(header, "Bearer ") {
				http.Error(w, "missing Authorization header", http.StatusUnauthorized)
				return
			}
			claims, err := svc.Validate(strings.TrimPrefix(header, "Bearer "))
			if err != nil {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), CtxAppID, claims.AppID)
			ctx = context.WithValue(ctx, CtxUserID, claims.UserID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
