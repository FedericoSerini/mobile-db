package auth_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
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

func serveJWKSWithStatus(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		fmt.Fprint(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestOIDCValidatorJWKSHTTPError(t *testing.T) {
	srv := serveJWKSWithStatus(t, http.StatusNotFound, `{"error":"Realm does not exist"}`)
	v := auth.NewOIDCValidator(srv.URL, "test", "")
	_, err := v.Validate("dummy")
	if err == nil {
		t.Fatal("non-200 JWKS response must return error")
	}
}

func TestOIDCValidatorEmptyJWKSNotCached(t *testing.T) {
	key := newTestRSAKey(t)
	kid := "key-1"
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			fmt.Fprint(w, `{"keys":[]}`)
			return
		}
		doc := map[string]any{
			"keys": []map[string]any{{
				"kid": kid,
				"kty": "RSA",
				"n":   base64.RawURLEncoding.EncodeToString(key.PublicKey.N.Bytes()),
				"e":   encodeE(key.PublicKey.E),
			}},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(doc)
	}))
	t.Cleanup(srv.Close)

	v := auth.NewOIDCValidator(srv.URL, "test", "")
	_, err := v.Validate("dummy")
	if err == nil {
		t.Fatal("empty JWKS must return error")
	}
	// Second call must re-fetch (not use empty cache) and succeed
	issuer := fmt.Sprintf("%s/realms/test", srv.URL)
	token := issueTestToken(t, key, kid, issuer, "app", "user", time.Hour)
	_, err = v.Validate(token)
	if err != nil {
		t.Fatalf("after re-fetch with real keys, Validate must succeed: %v", err)
	}
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

func newTestECKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate EC key: %v", err)
	}
	return k
}

func serveECJWKS(t *testing.T, kid string, pub *ecdsa.PublicKey) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		doc := map[string]any{
			"keys": []map[string]any{{
				"kid": kid,
				"kty": "EC",
				"crv": "P-256",
				"x":   base64.RawURLEncoding.EncodeToString(pub.X.Bytes()),
				"y":   base64.RawURLEncoding.EncodeToString(pub.Y.Bytes()),
			}},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(doc)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func issueECTestToken(t *testing.T, key *ecdsa.PrivateKey, kid, issuer, appID, userID string, ttl time.Duration) string {
	t.Helper()
	claims := jwt.MapClaims{
		"sub": userID,
		"azp": appID,
		"iss": issuer,
		"exp": time.Now().Add(ttl).Unix(),
		"iat": time.Now().Unix(),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	tok.Header["kid"] = kid
	str, err := tok.SignedString(key)
	if err != nil {
		t.Fatalf("sign EC token: %v", err)
	}
	return str
}

func TestOIDCValidatorES256(t *testing.T) {
	key := newTestECKey(t)
	kid := "ec-key-1"
	srv := serveECJWKS(t, kid, &key.PublicKey)

	v := auth.NewOIDCValidator(srv.URL, "test", "")
	issuer := fmt.Sprintf("%s/realms/test", srv.URL)
	token := issueECTestToken(t, key, kid, issuer, "my-app", "user42", time.Hour)

	claims, err := v.Validate(token)
	if err != nil {
		t.Fatalf("ES256 token must validate: %v", err)
	}
	if claims.Subject != "user42" {
		t.Fatalf("unexpected subject: %s", claims.Subject)
	}
}

func TestOIDCValidatorSingleFlight(t *testing.T) {
	key := newTestRSAKey(t)
	kid := "key-1"
	var fetchCount int
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		fetchCount++
		mu.Unlock()
		doc := map[string]any{
			"keys": []map[string]any{{
				"kid": kid,
				"kty": "RSA",
				"n":   base64.RawURLEncoding.EncodeToString(key.PublicKey.N.Bytes()),
				"e":   encodeE(key.PublicKey.E),
			}},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(doc)
	}))
	t.Cleanup(srv.Close)

	v := auth.NewOIDCValidator(srv.URL, "test", "")
	issuer := fmt.Sprintf("%s/realms/test", srv.URL)
	token := issueTestToken(t, key, kid, issuer, "app", "user", time.Hour)

	var wg sync.WaitGroup
	const concurrency = 20
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v.Validate(token) //nolint:errcheck
		}()
	}
	wg.Wait()

	mu.Lock()
	count := fetchCount
	mu.Unlock()
	if count > 2 {
		t.Fatalf("expected ≤2 JWKS fetches for %d concurrent Validates, got %d", concurrency, count)
	}
}
