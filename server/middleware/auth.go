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

// TokenValidator is satisfied by *auth.OIDCValidator and test stubs.
type TokenValidator interface {
	Validate(tokenStr string) (*auth.OIDCClaims, error)
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

// RequireJWT validates Authorization: Bearer <keycloak_token> header.
// app_id and user_id are extracted from the Keycloak token claims.
func RequireJWT(v TokenValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if !strings.HasPrefix(header, "Bearer ") {
				http.Error(w, "missing Authorization header", http.StatusUnauthorized)
				return
			}
			claims, err := v.Validate(strings.TrimPrefix(header, "Bearer "))
			if err != nil {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}
			if claims.AppID == "" {
				http.Error(w, "token missing azp claim", http.StatusUnauthorized)
				return
			}
			if m := logMetaFromCtx(r.Context()); m != nil {
				m.appID = claims.AppID
			}
			ctx := context.WithValue(r.Context(), CtxAppID, claims.AppID)
			ctx = context.WithValue(ctx, CtxUserID, claims.Subject)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
