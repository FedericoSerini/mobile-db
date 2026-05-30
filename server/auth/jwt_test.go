package auth_test

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/federicoserini/mobile-db/server/auth"
)

func newTestRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	return k
}

func encodeE(e int) string {
	b := []byte{byte(e >> 24), byte(e >> 16), byte(e >> 8), byte(e)}
	i := 0
	for i < 3 && b[i] == 0 {
		i++
	}
	return base64.RawURLEncoding.EncodeToString(b[i:])
}

func serveJWKS(t *testing.T, kid string, pub *rsa.PublicKey) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		doc := map[string]any{
			"keys": []map[string]any{{
				"kid": kid,
				"kty": "RSA",
				"alg": "RS256",
				"n":   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
				"e":   encodeE(pub.E),
			}},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(doc)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func issueTestToken(t *testing.T, key *rsa.PrivateKey, kid, issuer, appID, userID string, ttl time.Duration, aud ...string) string {
	t.Helper()
	claims := jwt.MapClaims{
		"sub": userID,
		"azp": appID,
		"iss": issuer,
		"exp": time.Now().Add(ttl).Unix(),
		"iat": time.Now().Unix(),
	}
	if len(aud) > 0 {
		claims["aud"] = aud
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = kid
	str, err := tok.SignedString(key)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return str
}

func TestOIDCValidatorValid(t *testing.T) {
	key := newTestRSAKey(t)
	kid := "key-1"
	srv := serveJWKS(t, kid, &key.PublicKey)

	v := auth.NewOIDCValidator(srv.URL, "test", "")
	issuer := fmt.Sprintf("%s/realms/test", srv.URL)
	token := issueTestToken(t, key, kid, issuer, "my-app", "user42", time.Hour)

	claims, err := v.Validate(token)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if claims.Subject != "user42" || claims.AppID != "my-app" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}

func TestOIDCValidatorExpired(t *testing.T) {
	key := newTestRSAKey(t)
	kid := "key-1"
	srv := serveJWKS(t, kid, &key.PublicKey)

	v := auth.NewOIDCValidator(srv.URL, "test", "")
	issuer := fmt.Sprintf("%s/realms/test", srv.URL)
	token := issueTestToken(t, key, kid, issuer, "app", "user", -time.Minute)

	_, err := v.Validate(token)
	if err == nil {
		t.Fatal("expired token must fail")
	}
}

func TestOIDCValidatorWrongIssuer(t *testing.T) {
	key := newTestRSAKey(t)
	kid := "key-1"
	srv := serveJWKS(t, kid, &key.PublicKey)

	v := auth.NewOIDCValidator(srv.URL, "test", "")
	token := issueTestToken(t, key, kid, "https://evil.example.com/realms/test", "app", "user", time.Hour)

	_, err := v.Validate(token)
	if err == nil {
		t.Fatal("wrong issuer must fail")
	}
}

func TestOIDCValidatorClientIDCheck(t *testing.T) {
	key := newTestRSAKey(t)
	kid := "key-1"
	srv := serveJWKS(t, kid, &key.PublicKey)

	v := auth.NewOIDCValidator(srv.URL, "test", "expected-client")
	issuer := fmt.Sprintf("%s/realms/test", srv.URL)
	token := issueTestToken(t, key, kid, issuer, "other-client", "user", time.Hour)

	_, err := v.Validate(token)
	if err == nil {
		t.Fatal("wrong clientID must fail")
	}
}

func TestOIDCValidatorAudienceCheck(t *testing.T) {
	key := newTestRSAKey(t)
	kid := "key-1"
	srv := serveJWKS(t, kid, &key.PublicKey)
	issuer := fmt.Sprintf("%s/realms/test", srv.URL)

	t.Run("token without aud rejected when clientID set", func(t *testing.T) {
		v := auth.NewOIDCValidator(srv.URL, "test", "my-client")
		token := issueTestToken(t, key, kid, issuer, "my-client", "user", time.Hour)
		_, err := v.Validate(token)
		if err == nil {
			t.Fatal("token without aud must be rejected when clientID is configured")
		}
	})

	t.Run("token with wrong aud rejected", func(t *testing.T) {
		v := auth.NewOIDCValidator(srv.URL, "test", "my-client")
		token := issueTestToken(t, key, kid, issuer, "my-client", "user", time.Hour, "other-client")
		_, err := v.Validate(token)
		if err == nil {
			t.Fatal("token with wrong aud must be rejected")
		}
	})

	t.Run("token with correct aud accepted", func(t *testing.T) {
		v := auth.NewOIDCValidator(srv.URL, "test", "my-client")
		token := issueTestToken(t, key, kid, issuer, "my-client", "user", time.Hour, "my-client")
		_, err := v.Validate(token)
		if err != nil {
			t.Fatalf("token with correct aud must pass: %v", err)
		}
	})

	t.Run("no aud check when clientID empty", func(t *testing.T) {
		v := auth.NewOIDCValidator(srv.URL, "test", "")
		token := issueTestToken(t, key, kid, issuer, "any-app", "user", time.Hour)
		_, err := v.Validate(token)
		if err != nil {
			t.Fatalf("no clientID set — aud must not be checked: %v", err)
		}
	})
}

func TestOIDCValidatorTrailingSlash(t *testing.T) {
	key := newTestRSAKey(t)
	kid := "key-1"
	srv := serveJWKS(t, kid, &key.PublicKey)

	// URL with trailing slash — must behave identically to without
	v := auth.NewOIDCValidator(srv.URL+"/", "test", "")
	issuer := fmt.Sprintf("%s/realms/test", srv.URL) // no double slash
	token := issueTestToken(t, key, kid, issuer, "my-app", "user42", time.Hour)

	claims, err := v.Validate(token)
	if err != nil {
		t.Fatalf("trailing slash must not break validation: %v", err)
	}
	if claims.Subject != "user42" {
		t.Fatalf("unexpected subject: %s", claims.Subject)
	}
}
