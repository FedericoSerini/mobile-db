package auth_test

import (
	"testing"
	"time"

	"github.com/federicoserini/mobile-db/server/auth"
)

func TestIssueAndValidateJWT(t *testing.T) {
	svc := auth.NewJWTService([]byte("32-byte-secret-for-testing-1234!"))
	token, err := svc.Issue("app1", "user42")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	claims, err := svc.Validate(token)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if claims.AppID != "app1" || claims.UserID != "user42" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}

func TestJWTExpiry(t *testing.T) {
	svc := auth.NewJWTService([]byte("32-byte-secret-for-testing-1234!"))
	token, _ := svc.IssueWithTTL("app1", "user1", -time.Minute)
	_, err := svc.Validate(token)
	if err == nil {
		t.Fatal("expired token must fail validation")
	}
}

func TestJWTWrongSecret(t *testing.T) {
	svc1 := auth.NewJWTService([]byte("secret-one-32-bytes-xxxxxxxxxxx!"))
	svc2 := auth.NewJWTService([]byte("secret-two-32-bytes-xxxxxxxxxxx!"))
	token, _ := svc1.Issue("app1", "user1")
	_, err := svc2.Validate(token)
	if err == nil {
		t.Fatal("token signed with different secret must fail")
	}
}
