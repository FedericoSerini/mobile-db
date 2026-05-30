package auth

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/sync/singleflight"
)

// OIDCClaims are extracted from a Keycloak access token.
// AppID maps to the "azp" (authorized party) claim — the Keycloak client ID.
// UserID is in RegisteredClaims.Subject ("sub").
type OIDCClaims struct {
	AppID string `json:"azp"`
	jwt.RegisteredClaims
}

type jwkKey struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	// RSA
	N string `json:"n"`
	E string `json:"e"`
	// EC
	Crv string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

type jwksDoc struct {
	Keys []jwkKey `json:"keys"`
}

const jwksCacheTTL = 10 * time.Minute

// OIDCValidator validates Keycloak-issued RS256 JWTs using the realm JWKS endpoint.
// Keys are cached for jwksCacheTTL; a cache miss triggers one immediate re-fetch.
type OIDCValidator struct {
	issuer   string
	jwksURL  string
	clientID string // optional azp check

	mu         sync.RWMutex
	keys       map[string]crypto.PublicKey
	fetchedAt  time.Time
	httpClient *http.Client
	fetchGroup singleflight.Group
}

func NewOIDCValidator(keycloakURL, realm, clientID string) *OIDCValidator {
	base := strings.TrimRight(keycloakURL, "/")
	return &OIDCValidator{
		issuer:     fmt.Sprintf("%s/realms/%s", base, realm),
		jwksURL:    fmt.Sprintf("%s/realms/%s/protocol/openid-connect/certs", base, realm),
		clientID:   clientID,
		keys:       map[string]crypto.PublicKey{},
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (v *OIDCValidator) Validate(tokenStr string) (*OIDCClaims, error) {
	if err := v.refreshIfStale(); err != nil {
		return nil, fmt.Errorf("jwks: %w", err)
	}

	token, err := jwt.ParseWithClaims(tokenStr, &OIDCClaims{}, v.keyFunc)
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*OIDCClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}
	if claims.Issuer != v.issuer {
		return nil, fmt.Errorf("unexpected issuer %q", claims.Issuer)
	}
	if v.clientID != "" {
		found := false
		for _, a := range claims.Audience {
			if a == v.clientID {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("token audience does not include %q", v.clientID)
		}
	}
	if v.clientID != "" && claims.AppID != v.clientID {
		return nil, fmt.Errorf("token not issued for client %q", v.clientID)
	}
	return claims, nil
}

func (v *OIDCValidator) keyFunc(t *jwt.Token) (any, error) {
	switch t.Method.(type) {
	case *jwt.SigningMethodRSA, *jwt.SigningMethodECDSA:
		// supported
	default:
		return nil, fmt.Errorf("unexpected signing method %v", t.Header["alg"])
	}
	kid, _ := t.Header["kid"].(string)

	v.mu.RLock()
	key, ok := v.keys[kid]
	v.mu.RUnlock()
	if ok {
		return key, nil
	}

	// Unknown kid: skip re-fetch if we refreshed within the last 5 seconds.
	v.mu.RLock()
	recentFetch := time.Since(v.fetchedAt) < 5*time.Second
	v.mu.RUnlock()
	if recentFetch {
		return nil, fmt.Errorf("unknown kid %q", kid)
	}
	if err := v.fetchJWKS(); err != nil {
		return nil, fmt.Errorf("jwks refresh: %w", err)
	}
	v.mu.RLock()
	key, ok = v.keys[kid]
	v.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown kid %q", kid)
	}
	return key, nil
}

func (v *OIDCValidator) refreshIfStale() error {
	v.mu.RLock()
	stale := time.Since(v.fetchedAt) > jwksCacheTTL
	v.mu.RUnlock()
	if !stale {
		return nil
	}
	_, err, _ := v.fetchGroup.Do("jwks", func() (any, error) {
		return nil, v.fetchJWKS()
	})
	return err
}

func (v *OIDCValidator) fetchJWKS() error {
	resp, err := v.httpClient.Get(v.jwksURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("jwks endpoint returned HTTP %d", resp.StatusCode)
	}

	var doc jwksDoc
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return fmt.Errorf("decode jwks: %w", err)
	}

	keys := make(map[string]crypto.PublicKey, len(doc.Keys))
	for _, k := range doc.Keys {
		if k.Kid == "" {
			continue
		}
		switch k.Kty {
		case "RSA":
			pub, err := jwkToRSA(k)
			if err != nil {
				continue
			}
			keys[k.Kid] = pub
		case "EC":
			pub, err := jwkToEC(k)
			if err != nil {
				continue
			}
			keys[k.Kid] = pub
		}
	}

	if len(keys) == 0 {
		return errors.New("jwks: zero usable keys in response")
	}

	v.mu.Lock()
	v.keys = keys
	v.fetchedAt = time.Now()
	v.mu.Unlock()
	return nil
}

func jwkToRSA(k jwkKey) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil {
		return nil, fmt.Errorf("decode n: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil {
		return nil, fmt.Errorf("decode e: %w", err)
	}
	e := 0
	for _, b := range eBytes {
		e = e<<8 | int(b)
	}
	return &rsa.PublicKey{N: new(big.Int).SetBytes(nBytes), E: e}, nil
}

func jwkToEC(k jwkKey) (*ecdsa.PublicKey, error) {
	xBytes, err := base64.RawURLEncoding.DecodeString(k.X)
	if err != nil {
		return nil, fmt.Errorf("decode x: %w", err)
	}
	yBytes, err := base64.RawURLEncoding.DecodeString(k.Y)
	if err != nil {
		return nil, fmt.Errorf("decode y: %w", err)
	}
	var curve elliptic.Curve
	switch k.Crv {
	case "P-256":
		curve = elliptic.P256()
	case "P-384":
		curve = elliptic.P384()
	case "P-521":
		curve = elliptic.P521()
	default:
		return nil, fmt.Errorf("unsupported EC curve %q", k.Crv)
	}
	return &ecdsa.PublicKey{
		Curve: curve,
		X:     new(big.Int).SetBytes(xBytes),
		Y:     new(big.Int).SetBytes(yBytes),
	}, nil
}
