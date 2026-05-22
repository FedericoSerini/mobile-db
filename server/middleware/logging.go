package middleware

import (
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

// RequestLogger logs every request as structured JSON.
func RequestLogger(log zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rw := &responseWriter{ResponseWriter: w, code: http.StatusOK}
			next.ServeHTTP(rw, r)
			log.Info().
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Int("status", rw.code).
				Int64("duration_ms", time.Since(start).Milliseconds()).
				Str("remote_addr", r.RemoteAddr).
				Msg("request")
		})
	}
}
