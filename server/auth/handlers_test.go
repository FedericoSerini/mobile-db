package auth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/federicoserini/mobile-db/server/auth"
)

func TestRegisterHandler(t *testing.T) {
	svc := auth.NewDeviceService(newStubStore())
	h := auth.NewHandlers(svc, nil, nil)
	body, _ := json.Marshal(map[string]string{"app_id": "app1", "device_id": "dev1"})
	req := httptest.NewRequest("POST", "/devices/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Register(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d: %s", rr.Code, rr.Body)
	}
	var resp map[string]string
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["device_key_id"] == "" || resp["device_secret"] == "" {
		t.Fatalf("missing fields: %v", resp)
	}
}

func TestTokenHandler(t *testing.T) {
	store := newStubStore()
	devSvc := auth.NewDeviceService(store)
	jwtSvc := auth.NewJWTService([]byte("32-byte-secret-for-testing-1234!"))
	h := auth.NewHandlers(devSvc, jwtSvc, nil)
	reg, _ := devSvc.Register(context.Background(), "app1", "dev1")
	body, _ := json.Marshal(map[string]string{
		"app_id": "app1", "user_id": "u1",
		"device_key_id": reg.DeviceKeyID, "device_secret": reg.DeviceSecret,
	})
	req := httptest.NewRequest("POST", "/auth/token", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Token(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body)
	}
	var resp map[string]string
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["access_token"] == "" {
		t.Fatal("missing access_token")
	}
}
