package crsqlite_test

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/federicoserini/mobile-db/storage/crsqlite"
)

func openMetaDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := crsqlite.ApplyMetaDDL(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestWriteSyncEvent(t *testing.T) {
	db := openMetaDB(t)
	defer db.Close()

	store := crsqlite.NewAdminEventStoreFromDB(db)
	err := store.WriteSyncEvent(context.Background(), "app1", "notes", "user1", "dkey1", 7)
	if err != nil {
		t.Fatalf("WriteSyncEvent: %v", err)
	}

	var count int
	db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM sync_events WHERE app_id='app1' AND op_count=7`).Scan(&count)
	if count != 1 {
		t.Fatalf("want 1 row, got %d", count)
	}
}
