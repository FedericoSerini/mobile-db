package auth

import (
	"encoding/json"
	"net/http"
	"strings"
)

type Handlers struct {
	deviceSvc *DeviceService
	oidcSvc   *OIDCValidator // nil when Keycloak not configured
}

func NewHandlers(deviceSvc *DeviceService, oidcSvc *OIDCValidator) *Handlers {
	return &Handlers{deviceSvc: deviceSvc, oidcSvc: oidcSvc}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

// POST /devices/register
func (h *Handlers) Register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AppID    string `json:"app_id"`
		DeviceID string `json:"device_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.AppID == "" || req.DeviceID == "" {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	result, err := h.deviceSvc.Register(r.Context(), req.AppID, req.DeviceID)
	if err != nil {
		http.Error(w, "registration failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{
		"device_key_id": result.DeviceKeyID,
		"device_secret": result.DeviceSecret,
	})
}

// POST /auth/token
// Verifies the device key from the request body.
// If Authorization: Bearer <keycloak_token> is present, validates it via Keycloak
// and returns {app_id, user_id} extracted from the token.
func (h *Handlers) Token(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceKeyID  string `json:"device_key_id"`
		DeviceSecret string `json:"device_secret"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := h.deviceSvc.Verify(r.Context(), req.DeviceKeyID, req.DeviceSecret); err != nil {
		http.Error(w, "invalid device key", http.StatusUnauthorized)
		return
	}

	header := r.Header.Get("Authorization")
	if strings.HasPrefix(header, "Bearer ") && h.oidcSvc != nil {
		claims, err := h.oidcSvc.Validate(strings.TrimPrefix(header, "Bearer "))
		if err != nil {
			http.Error(w, "invalid token: "+err.Error(), http.StatusUnauthorized)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{
			"app_id":  claims.AppID,
			"user_id": claims.Subject,
		})
		return
	}

	w.WriteHeader(http.StatusOK)
}

// POST /devices/rotate-key
func (h *Handlers) RotateKey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceKeyID string `json:"device_key_id"`
		OldSecret   string `json:"old_secret"`
		NewSecret   string `json:"new_secret"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := h.deviceSvc.RotateKey(r.Context(), req.DeviceKeyID, req.OldSecret, req.NewSecret); err != nil {
		http.Error(w, "rotation failed: "+err.Error(), http.StatusUnauthorized)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DELETE /devices/{device_key_id}
func (h *Handlers) RevokeDevice(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	idx := strings.LastIndex(path, "/")
	deviceKeyID := path[idx+1:]
	if deviceKeyID == "" {
		http.Error(w, "missing device_key_id", http.StatusBadRequest)
		return
	}
	if err := h.deviceSvc.Revoke(r.Context(), deviceKeyID); err != nil {
		http.Error(w, "revoke failed: "+err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
