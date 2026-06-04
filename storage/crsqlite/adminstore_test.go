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

func seedDevices(t *testing.T, db *sql.DB) {
	t.Helper()
	db.ExecContext(context.Background(), `
		INSERT INTO device_keys (device_key_id, device_id, app_id, key_hash, status)
		VALUES ('dk1','phone1','app1','h1','active'),
		       ('dk2','phone2','app1','h2','active'),
		       ('dk3','phone3','app1','h3','revoked')`)
}

func TestDeviceCounts(t *testing.T) {
	db := openMetaDB(t)
	defer db.Close()
	seedDevices(t, db)

	store := crsqlite.NewAdminDeviceStoreFromDB(db)
	all, active, revoked, err := store.DeviceCounts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if all != 3 || active != 2 || revoked != 1 {
		t.Fatalf("want 3/2/1 got %d/%d/%d", all, active, revoked)
	}
}

func TestListDevicesStatusFilter(t *testing.T) {
	db := openMetaDB(t)
	defer db.Close()
	seedDevices(t, db)

	store := crsqlite.NewAdminDeviceStoreFromDB(db)

	all, _ := store.ListDevices(context.Background(), "")
	if len(all) != 3 {
		t.Fatalf("want 3 devices, got %d", len(all))
	}

	active, _ := store.ListDevices(context.Background(), "active")
	if len(active) != 2 {
		t.Fatalf("want 2 active, got %d", len(active))
	}

	revoked, _ := store.ListDevices(context.Background(), "revoked")
	if len(revoked) != 1 {
		t.Fatalf("want 1 revoked, got %d", len(revoked))
	}
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
