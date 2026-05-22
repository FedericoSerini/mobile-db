package server

import (
	"net/http"

	"github.com/federicoserini/mobile-db/server/auth"
	"github.com/federicoserini/mobile-db/server/handlers"
	"github.com/federicoserini/mobile-db/server/middleware"
	"github.com/federicoserini/mobile-db/transport/http2"
)

type RouterDeps struct {
	AuthHandlers  *auth.Handlers
	SyncHandler   *handlers.SyncHandler
	Broadcaster   *http2.Broadcaster
	JWTSvc        *auth.JWTService
	DeviceSvc     *auth.DeviceService
	AdminPassHash string
	AdminIPs      []string
}

func NewRouter(d RouterDeps) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /devices/register", d.AuthHandlers.Register)
	mux.HandleFunc("POST /auth/token", d.AuthHandlers.Token)
	mux.HandleFunc("POST /devices/rotate-key", d.AuthHandlers.RotateKey)

	deviceAuth := middleware.RequireDeviceKey(d.DeviceSvc)
	jwtAuth := middleware.RequireJWT(d.JWTSvc)

	mux.Handle("DELETE /devices/", deviceAuth(http.HandlerFunc(d.AuthHandlers.RevokeDevice)))
	mux.Handle("POST /sync", deviceAuth(jwtAuth(d.SyncHandler)))
	mux.Handle("GET /events", jwtAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		appID, _ := r.Context().Value(middleware.CtxAppID).(string)
		userID, _ := r.Context().Value(middleware.CtxUserID).(string)
		http2.SSEHandler(d.Broadcaster, appID, userID)(w, r)
	})))

	return mux
}
