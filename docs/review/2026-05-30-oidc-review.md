# Code Review — OIDC/Keycloak Integration
_Date: 2026-05-30_
_Scope: Keycloak auth refactor (working-tree diff against last commit)_

## Status

| # | Severity | Status | Finding |
|---|---|---|---|
| 1 | 🔴 Security | FIXED | `aud` claim never validated |
| 2 | 🔴 Security/Config | FIXED | Trailing slash in `KEYCLOAK_URL` breaks all auth |
| 3 | 🔴 Security | FIXED | Missing `azp` → empty `app_id` → empty-namespace data access |
| 4 | 🟠 Availability | FIXED | No HTTP status check → Keycloak 404 wipes live key cache |
| 5 | 🟠 Availability | FIXED | Empty JWKS overwrites live keys → 10-min lockout |
| 6 | 🟡 DoS | FIXED | Unknown `kid` → unbounded `fetchJWKS` per request |
| 7 | 🟡 Reliability | FIXED | TOCTOU: N concurrent goroutines → N simultaneous JWKS fetches |
| 8 | 🟡 Reliability | FIXED | EC keys (ES256) silently rejected → total auth failure if realm uses EC |

---

## Findings

### 1. 🔴 `aud` claim never validated
**File:** `server/auth/jwt.go:73`
**Status:** OPEN

`jwt.ParseWithClaims` is called without `jwt.WithAudience()`, so the `aud` field is parsed but never checked. `KEYCLOAK_CLIENT_ID` only validates `azp` (who initiated the flow), not `aud` (who the token is intended for).

**Failure scenario:** User authenticates to any other Keycloak client in the same realm and receives a token with `aud=['spa-client']`. Token passes issuer and signature checks and grants full sync access to mobile-db.

**Fix:**
```go
// In Validate(), add audience check when clientID is configured:
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
```
Or pass `jwt.WithAudience(v.clientID)` to `jwt.NewParser`.

---

### 2. 🔴 Trailing slash in `KEYCLOAK_URL` breaks all authentication
**File:** `server/auth/jwt.go:53`
**Status:** OPEN

`NewOIDCValidator` builds the issuer with naive string concatenation. `KEYCLOAK_URL=https://host/` (trailing slash) produces `issuer=https://host//realms/realm`. Keycloak issues tokens with `iss=https://host/realms/realm`. Strict equality at line 74 fails for every token. LoadConfig does not normalize or reject trailing slashes.

**Failure scenario:** Operator copy-pastes URL with trailing slash. Every `/sync` request returns 401 with error `unexpected issuer "https://host//realms/homelab"`. No diagnostic points to the trailing slash.

**Fix:**
```go
func NewOIDCValidator(keycloakURL, realm, clientID string) *OIDCValidator {
    base := strings.TrimRight(keycloakURL, "/")
    return &OIDCValidator{
        issuer:  fmt.Sprintf("%s/realms/%s", base, realm),
        jwksURL: fmt.Sprintf("%s/realms/%s/protocol/openid-connect/certs", base, realm),
        // ...
    }
}
```

---

### 3. 🔴 Missing `azp` claim → empty `app_id` reaches sync engine
**File:** `server/auth/jwt.go:21`
**Status:** OPEN

The `azp` claim is optional per OIDC spec (absent when `aud` and `azp` would be identical). A token without `azp` unmarshals to `OIDCClaims{AppID: ""}`. `RequireJWT` writes `""` into `CtxAppID` with no guard. The sync handler only sets `msg.AppID` from context when `appID != ""`, so a client-supplied empty `app_id` passes through to storage unchecked.

**Failure scenario:** Token without `azp` → `app_id=""` → sync engine creates table `op_log___<userID>_<datasetID>` → any user with such a token can read/write the empty-app-namespace partition.

**Fix:**
```go
// In RequireJWT, after Validate():
if claims.AppID == "" {
    http.Error(w, "token missing azp claim", http.StatusUnauthorized)
    return
}
```

---

### 4. 🟠 No HTTP status check in `fetchJWKS` → Keycloak error JSON wipes live key cache
**File:** `server/auth/jwt.go:119`
**Status:** OPEN

`fetchJWKS` does not check `resp.StatusCode` before JSON decoding. A Keycloak 404 response like `{"error":"Realm does not exist"}` decodes without error, produces zero keys, and `v.keys` is replaced with an empty map. `fetchedAt` is updated, so the empty map is cached for the full 10-minute TTL.

**Failure scenario:** `KEYCLOAK_REALM` is misconfigured → Keycloak 404 → empty key map cached → all users locked out for 10 minutes with `unknown kid` errors.

**Fix:**
```go
resp, err := v.httpClient.Get(v.jwksURL)
if err != nil {
    return err
}
defer resp.Body.Close()
if resp.StatusCode != http.StatusOK {
    return fmt.Errorf("jwks endpoint returned HTTP %d", resp.StatusCode)
}
```

---

### 5. 🟠 Empty JWKS response unconditionally overwrites live key cache
**File:** `server/auth/jwt.go:140`
**Status:** OPEN

`fetchJWKS` always writes `v.keys = keys` even when `keys` is empty (zero RSA keys parsed, all keys failed `jwkToRSA`, or all keys are EC). Returns `nil` (success) and advances `fetchedAt`, caching the empty state for 10 minutes.

**Failure scenario:** Keycloak serves a JWKS with only EC keys during key rotation → zero RSA keys parsed → empty map cached → all validations fail with `unknown kid` for up to 10 min.

**Fix:**
```go
if len(keys) == 0 {
    return errors.New("jwks: zero usable keys in response")
}
v.mu.Lock()
v.keys = keys
v.fetchedAt = time.Now()
v.mu.Unlock()
```

---

### 6. 🟡 Unknown `kid` triggers unbounded `fetchJWKS` per request (DoS vector)
**File:** `server/auth/jwt.go:97`
**Status:** OPEN

`keyFunc` calls `fetchJWKS()` on every cache-miss with no rate-limiting. `refreshIfStale` is gated by a 10-minute TTL; this path has no gate. An attacker sending tokens with rotating/random `kid` headers triggers one HTTP GET to Keycloak per request.

**Failure scenario:** 500 req/s with unknown kids → 500 JWKS fetches/s to Keycloak.

**Fix:** Skip the in-keyFunc re-fetch if a refresh occurred within a short back-off window:
```go
// In keyFunc, before calling fetchJWKS:
v.mu.RLock()
recentFetch := time.Since(v.fetchedAt) < 5*time.Second
v.mu.RUnlock()
if recentFetch {
    return nil, fmt.Errorf("unknown kid %q", kid)
}
if err := v.fetchJWKS(); err != nil { ... }
```

---

### 7. 🟡 TOCTOU in `refreshIfStale` → thundering herd of JWKS fetches
**File:** `server/auth/jwt.go:109`
**Status:** OPEN

`refreshIfStale` reads `fetchedAt` under `RLock`, releases it, then calls `fetchJWKS`. No exclusion between the check and the call. N concurrent goroutines all see stale=true and all call `fetchJWKS` concurrently.

**Failure scenario:** Server starts (fetchedAt is zero) and receives 50 concurrent sync requests → 50 simultaneous HTTP GETs to Keycloak's `/certs` endpoint. Repeats every 10 minutes at TTL expiry.

**Fix:** Use `golang.org/x/sync/singleflight`:
```go
var fetchGroup singleflight.Group

func (v *OIDCValidator) refreshIfStale() error {
    v.mu.RLock()
    stale := time.Since(v.fetchedAt) > jwksCacheTTL
    v.mu.RUnlock()
    if !stale {
        return nil
    }
    _, err, _ := fetchGroup.Do("jwks", func() (any, error) {
        return nil, v.fetchJWKS()
    })
    return err
}
```

---

### 8. 🟡 EC keys (ES256) silently rejected → complete auth failure if realm uses EC
**File:** `server/auth/jwt.go:84`
**Status:** OPEN

`keyFunc` hard-rejects any non-RSA signing method. `fetchJWKS` skips all non-RSA keys (`kty != "RSA"`). The `jwkKey` struct has no EC fields (`Crv`, `X`, `Y`). If a Keycloak realm is configured with EC keys, the JWKS is fetched successfully but produces an empty key map, and all tokens are rejected with `unknown kid`.

**Failure scenario:** Keycloak realm reconfigured to ES256 → fetchJWKS returns nil/success with empty keys → every token validation fails → all auth broken with no diagnostic.

**Fix:** Extend `jwkKey` to include EC fields and handle `*jwt.SigningMethodECDSA` in `keyFunc`, or document clearly that only RS256 is supported and add a startup check/log warning.

---

## Notes

- Findings 4 + 5 overlap: both result in an empty key cache. Fix #4 (status check) prevents the most common trigger; fix #5 (empty-keys guard) is a defense-in-depth second layer. Both should be applied.
- Finding 7 (TOCTOU) and finding 6 (unbounded keyFunc fetch) compound: worst case is `2×N` fetches per burst. The `singleflight` fix for #7 also reduces the blast radius of #6.
- Finding 1 (aud) and finding 3 (empty azp) are independent security gaps that should both be fixed before any production traffic is routed through this server.
