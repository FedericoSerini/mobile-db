package middleware

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type rateBucket struct {
	count   int
	resetAt time.Time
}

var (
	rateMu      sync.Mutex
	rateBuckets = map[string]*rateBucket{}
)

const (
	maxAdminAttempts = 5
	rateLimitWindow  = 15 * time.Minute
)

// RequireAdminAuth enforces HTTP Basic Auth with bcrypt password.
// allowedIPs is optional; nil allows all IPs.
func RequireAdminAuth(passwordHash string, allowedIPs []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				ip = r.RemoteAddr
			}
			if len(allowedIPs) > 0 && !ipAllowed(ip, allowedIPs) {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			if rateLimited(ip) {
				http.Error(w, "too many attempts", http.StatusTooManyRequests)
				return
			}
			_, password, ok := r.BasicAuth()
			if !ok || bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)) != nil {
				recordAttempt(ip)
				w.Header().Set("WWW-Authenticate", `Basic realm="mobile-db admin"`)
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func ipAllowed(ip string, allowed []string) bool {
	for _, a := range allowed {
		if strings.TrimSpace(a) == ip {
			return true
		}
	}
	return false
}

func rateLimited(ip string) bool {
	rateMu.Lock()
	defer rateMu.Unlock()
	b, ok := rateBuckets[ip]
	if !ok {
		return false
	}
	if time.Now().After(b.resetAt) {
		delete(rateBuckets, ip)
		return false
	}
	return b.count >= maxAdminAttempts
}

func recordAttempt(ip string) {
	rateMu.Lock()
	defer rateMu.Unlock()
	b, ok := rateBuckets[ip]
	if !ok || time.Now().After(b.resetAt) {
		rateBuckets[ip] = &rateBucket{count: 1, resetAt: time.Now().Add(rateLimitWindow)}
		return
	}
	b.count++
}
