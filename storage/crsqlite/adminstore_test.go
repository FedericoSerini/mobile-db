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

func openSnapDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.ExecContext(context.Background(), `
		CREATE TABLE IF NOT EXISTS snapshots (
			dataset_id  TEXT NOT NULL,
			user_id     TEXT NOT NULL,
			doc_id      TEXT NOT NULL,
			data        TEXT NOT NULL,
			wall_time   INTEGER NOT NULL,
			logical     INTEGER NOT NULL,
			device_id   TEXT NOT NULL,
			created_at  DATETIME NOT NULL DEFAULT (datetime('now')),
			PRIMARY KEY (dataset_id, user_id, doc_id)
		);
		CREATE TABLE IF NOT EXISTS datasets (
			app_id         TEXT NOT NULL,
			dataset_id     TEXT NOT NULL,
			acl            TEXT NOT NULL DEFAULT 'private',
			schema_version INTEGER NOT NULL DEFAULT 1,
			PRIMARY KEY (app_id, dataset_id)
		);`)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func seedSnapshot(t *testing.T, db *sql.DB, datasetID, userID, docID, data, deviceID string) {
	t.Helper()
	_, err := db.ExecContext(context.Background(),
		`INSERT OR REPLACE INTO snapshots (dataset_id, user_id, doc_id, data, wall_time, logical, device_id)
		 VALUES (?,?,?,?,0,0,?)`,
		datasetID, userID, docID, data, deviceID)
	if err != nil {
		t.Fatal(err)
	}
}

func TestListDocsAllColumns(t *testing.T) {
	db := openSnapDB(t)
	defer db.Close()
	seedSnapshot(t, db, "ds1", "user-42", "doc1", `{"x":1}`, "dev-iphone")

	store := crsqlite.NewAdminDataStoreFromDB(db)
	docs, err := store.ListDocs(context.Background(), "app1", "ds1", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 1 {
		t.Fatalf("want 1 doc, got %d", len(docs))
	}
	d := docs[0]
	for _, key := range []string{"doc_id", "user_id", "data", "device_id", "created_at"} {
		if _, ok := d[key]; !ok {
			t.Errorf("missing key %q in doc map", key)
		}
	}
	if d["user_id"] != "user-42" {
		t.Errorf("want user_id=user-42, got %v", d["user_id"])
	}
}

func TestListDocsUserIDFilter(t *testing.T) {
	db := openSnapDB(t)
	defer db.Close()
	seedSnapshot(t, db, "ds1", "user-42", "doc1", `{}`, "dev1")
	seedSnapshot(t, db, "ds1", "user-99", "doc2", `{}`, "dev2")

	store := crsqlite.NewAdminDataStoreFromDB(db)
	docs, _ := store.ListDocs(context.Background(), "app1", "ds1", "user-42")
	if len(docs) != 1 {
		t.Fatalf("want 1 doc filtered, got %d", len(docs))
	}
}

func TestGetDoc(t *testing.T) {
	db := openSnapDB(t)
	defer db.Close()
	seedSnapshot(t, db, "ds1", "user-42", "doc1", `{"name":"Alice"}`, "dev1")

	store := crsqlite.NewAdminDataStoreFromDB(db)
	doc, err := store.GetDoc(context.Background(), "ds1", "user-42", "doc1")
	if err != nil {
		t.Fatal(err)
	}
	if doc["data"] != `{"name":"Alice"}` {
		t.Errorf("want data={\"name\":\"Alice\"}, got %v", doc["data"])
	}
}

func TestDeleteDocRequiresUserID(t *testing.T) {
	db := openSnapDB(t)
	defer db.Close()
	seedSnapshot(t, db, "ds1", "user-42", "doc1", `{}`, "dev1")
	seedSnapshot(t, db, "ds1", "user-99", "doc1", `{}`, "dev2")

	store := crsqlite.NewAdminDataStoreFromDB(db)
	if err := store.DeleteDoc(context.Background(), "ds1", "user-42", "doc1"); err != nil {
		t.Fatal(err)
	}
	// user-99's doc1 must still exist
	doc, err := store.GetDoc(context.Background(), "ds1", "user-99", "doc1")
	if err != nil || doc == nil {
		t.Fatal("DeleteDoc must be scoped to user_id; user-99 doc deleted wrongly")
	}
}
