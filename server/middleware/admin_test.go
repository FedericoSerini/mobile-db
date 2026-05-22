package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/federicoserini/mobile-db/server/middleware"
	"golang.org/x/crypto/bcrypt"
)

func hashPW(t *testing.T, pw string) string {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte(pw), 4)
	if err != nil {
		t.Fatal(err)
	}
	return string(h)
}

func TestAdminBasicAuthValid(t *testing.T) {
	h := middleware.RequireAdminAuth(hashPW(t, "pass"), nil)(http.HandlerFunc(okHandler))
	req := httptest.NewRequest("GET", "/admin/", nil)
	req.SetBasicAuth("admin", "pass")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}
}

func TestAdminBasicAuthInvalid(t *testing.T) {
	h := middleware.RequireAdminAuth(hashPW(t, "pass"), nil)(http.HandlerFunc(okHandler))
	req := httptest.NewRequest("GET", "/admin/", nil)
	req.SetBasicAuth("admin", "wrong")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rr.Code)
	}
}

func TestAdminIPAllowlist(t *testing.T) {
	h := middleware.RequireAdminAuth(hashPW(t, "pass"), []string{"192.168.1.1"})(http.HandlerFunc(okHandler))
	req := httptest.NewRequest("GET", "/admin/", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	req.SetBasicAuth("admin", "pass")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("want 403 for non-allowlisted IP, got %d", rr.Code)
	}
}
