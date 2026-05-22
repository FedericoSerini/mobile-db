package auth

import (
	"encoding/json"
	"net/http"
	"strings"
)

type Handlers struct {
	deviceSvc  *DeviceService
	jwtSvc     *JWTService
	refreshSvc *RefreshService
}

func NewHandlers(deviceSvc *DeviceService, jwtSvc *JWTService, refreshSvc *RefreshService) *Handlers {
	return &Handlers{deviceSvc: deviceSvc, jwtSvc: jwtSvc, refreshSvc: refreshSvc}
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
func (h *Handlers) Token(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AppID        string `json:"app_id"`
		UserID       string `json:"user_id"`
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
	token, err := h.jwtSvc.Issue(req.AppID, req.UserID)
	if err != nil {
		http.Error(w, "token issuance failed", http.StatusInternalServerError)
		return
	}
	var refreshToken string
	if h.refreshSvc != nil {
		refreshToken, _ = h.refreshSvc.IssueRefresh(r.Context(), req.AppID, req.UserID)
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"access_token":  token,
		"refresh_token": refreshToken,
	})
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
