package crsqlite

import (
	"context"
	"database/sql"
	"fmt"
)

// MetaDB wraps a *sql.DB that holds the metadata tables (device_keys,
// datasets, refresh_tokens). It is typically obtained via Backend.Meta()
// so that the same connection is reused without a second open/close.
type MetaDB struct {
	db    *sql.DB
	owner bool // true if this MetaDB owns the connection and should close it
}

// OpenMeta opens (or creates) a standalone MetaDB at path.
// Use this only when you need an independent connection; prefer
// Backend.Meta() to share the Backend's existing connection.
func OpenMeta(path, key, extPath string) (*MetaDB, error) {
	h, err := Open(path, key, extPath)
	if err != nil {
		return nil, fmt.Errorf("open meta db: %w", err)
	}
	ctx := context.Background()
	if err := EnsureMeta(ctx, h.DB()); err != nil {
		h.Close()
		return nil, fmt.Errorf("ensure meta schema: %w", err)
	}
	return &MetaDB{db: h.DB(), owner: true}, nil
}

// DB returns the underlying *sql.DB.
func (m *MetaDB) DB() *sql.DB { return m.db }

// Close closes the underlying connection only if this MetaDB owns it.
// MetaDB instances obtained via Backend.Meta() are non-owning and this
// is a no-op to prevent double-close.
func (m *MetaDB) Close() error {
	if m.owner {
		return m.db.Close()
	}
	return nil
}
