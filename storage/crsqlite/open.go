package crsqlite

import (
	"context"
	"database/sql"
	"fmt"
	"sync"

	sqlite3 "github.com/mattn/go-sqlite3"
)

var (
	driverMu   sync.Mutex
	driverPath string
	driverName = "sqlite3_crsqlite"
)

func ensureDriver(extPath string) error {
	driverMu.Lock()
	defer driverMu.Unlock()
	if driverPath == extPath {
		return nil // already registered
	}
	if driverPath != "" {
		return fmt.Errorf("cr-sqlite driver already registered with path %q; cannot re-register with %q", driverPath, extPath)
	}
	sql.Register(driverName, &sqlite3.SQLiteDriver{
		ConnectHook: func(conn *sqlite3.SQLiteConn) error {
			return conn.LoadExtension(extPath, "sqlite3_crsqlite_init")
		},
	})
	driverPath = extPath
	return nil
}

// Handle wraps a *sql.DB opened with the cr-sqlite extension loaded.
type Handle struct {
	db *sql.DB
}

// Open opens (or creates) a SQLite database with cr-sqlite extension loaded.
// key param is reserved for future SQLCipher use; currently unused.
func Open(path, key, extPath string) (*Handle, error) {
	if err := ensureDriver(extPath); err != nil {
		return nil, err
	}
	dsn := fmt.Sprintf("file:%s?_journal_mode=WAL&_foreign_keys=ON", path)
	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	db.SetMaxOpenConns(1)
	if err := db.PingContext(context.Background()); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return &Handle{db: db}, nil
}

// DB returns the underlying *sql.DB.
func (h *Handle) DB() *sql.DB { return h.db }

// Close closes the underlying database connection.
func (h *Handle) Close() error { return h.db.Close() }
