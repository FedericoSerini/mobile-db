package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/rs/zerolog"
)

type responseWriter struct {
	http.ResponseWriter
	code int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.code = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

type logMeta struct{ appID string }

type ctxLogMetaKey struct{}

// logMetaFromCtx returns the mutable log metadata injected by RequestLogger.
// Inner middleware (e.g. RequireJWT) uses this to write fields back to the logger.
func logMetaFromCtx(ctx context.Context) *logMeta {
	m, _ := ctx.Value(ctxLogMetaKey{}).(*logMeta)
	return m
}

// RequestLogger logs every request as structured JSON.
func RequestLogger(log zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			meta := &logMeta{}
			r = r.WithContext(context.WithValue(r.Context(), ctxLogMetaKey{}, meta))
			rw := &responseWriter{ResponseWriter: w, code: http.StatusOK}
			next.ServeHTTP(rw, r)
			log.Info().
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Int("status", rw.code).
				Int64("duration_ms", time.Since(start).Milliseconds()).
				Str("remote_addr", r.RemoteAddr).
				Str("app_id", meta.appID).
				Msg("request")
		})
	}
}
