package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog"

	"github.com/federicoserini/mobile-db/admin"
	"github.com/federicoserini/mobile-db/compression/cbor"
	"github.com/federicoserini/mobile-db/compression/zstd"
	"github.com/federicoserini/mobile-db/core/crdt"
	coresync "github.com/federicoserini/mobile-db/core/sync"
	"github.com/federicoserini/mobile-db/server/auth"
	"github.com/federicoserini/mobile-db/server/backup"
	"github.com/federicoserini/mobile-db/server/handlers"
	"github.com/federicoserini/mobile-db/server/middleware"
	"github.com/federicoserini/mobile-db/storage/crsqlite"
	http2transport "github.com/federicoserini/mobile-db/transport/http2"
)

// Run initializes all components and starts the HTTP server.
// Blocks until ctx is cancelled.
func Run(ctx context.Context, cfg *Config, log zerolog.Logger) error {
	if err := os.MkdirAll(cfg.DataDir, 0700); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	dbPath := filepath.Join(cfg.DataDir, "mobile-db.db")
	encKey := string(cfg.DBEncryptionKey)

	store, err := crsqlite.NewBackend(dbPath, encKey, cfg.CRSQLiteExtPath)
	if err != nil {
		return fmt.Errorf("storage init: %w", err)
	}
	defer store.Close()

	metaDB := store.Meta()

	authStore := crsqlite.NewAuthStore(metaDB)
	deviceSvc := auth.NewDeviceService(authStore)
	oidcValidator := auth.NewOIDCValidator(cfg.KeycloakURL, cfg.KeycloakRealm, cfg.KeycloakClientID)
	authHandlers := auth.NewHandlers(deviceSvc, oidcValidator)

	broadcaster := http2transport.NewBroadcaster()
	clock := crdt.NewClock("server")
	engine := coresync.NewEngine(store, broadcaster, clock, cfg.CompactionThreshold)

	codec := cbor.NewCodec()
	comp := zstd.NewCompressor()

	metricsReg := handlers.NewMetricsRegistry()
	syncHandler := handlers.NewSyncHandler(engine, codec, comp)
	healthHandler := handlers.NewHealthHandler("1.0.0").WithDBCheck(func() bool {
		return store != nil
	})
	metricsHandler := handlers.NewMetricsHandler(metricsReg)

	tmpl, err := admin.LoadTemplates()
	if err != nil {
		return fmt.Errorf("load admin templates: %w", err)
	}

	adminRouter := admin.NewAdminRouter(admin.AdminRouterDeps{
		PasswordHash: cfg.AdminPasswordHash,
		AllowedIPs:   cfg.AdminAllowedIPs,
		DeviceStore:  crsqlite.NewAdminDeviceStore(metaDB),
		DataStore:    crsqlite.NewAdminDataStore(store),
		SyncStore:    crsqlite.NewAdminSyncStore(store),
		Tmpl:         tmpl,
	})

	mainMux := http.NewServeMux()
	mainMux.Handle("/admin/", adminRouter)
	mainMux.HandleFunc("/health", healthHandler.ServeHTTP)
	mainMux.Handle("/metrics", middleware.RequireAdminAuth(cfg.AdminPasswordHash, cfg.AdminAllowedIPs)(metricsHandler))
	mainMux.Handle("/", NewRouter(RouterDeps{
		AuthHandlers:  authHandlers,
		SyncHandler:   syncHandler,
		Broadcaster:   broadcaster,
		OIDCValidator: oidcValidator,
		DeviceSvc:     deviceSvc,
		AdminPassHash: cfg.AdminPasswordHash,
		AdminIPs:      cfg.AdminAllowedIPs,
	}))

	logged := middleware.RequestLogger(log)(mainMux)

	if cfg.BackupDestination != "" {
		backupInterval := 24 * time.Hour
		if cfg.BackupInterval != "" {
			if d, err := time.ParseDuration(cfg.BackupInterval); err == nil {
				backupInterval = d
			}
		}
		br := backup.NewRunner(backup.Config{
			SourcePaths: []string{dbPath},
			Destination: cfg.BackupDestination,
			Interval:    backupInterval,
		})
		go br.Start()
		defer br.Stop()
	}

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           logged,
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Info().Str("addr", cfg.ListenAddr).Msg("mobile-db listening")

	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutCtx)
	}
}
