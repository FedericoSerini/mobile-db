package crdt_test

import (
	"context"
	"testing"

	"github.com/federicoserini/mobile-db/core"
	"github.com/federicoserini/mobile-db/core/crdt"
)

type schemaStub struct {
	core.NopStorage
	version int
}

func (s *schemaStub) SchemaVersion(_ context.Context, _, _ string) (int, error) {
	return s.version, nil
}

func TestSchemaVersionMatchNeedsNoSnapshot(t *testing.T) {
	store := &schemaStub{version: 3}
	needs, err := crdt.NeedsSnapshot(context.Background(), store, "app1", "ds1", 3)
	if err != nil {
		t.Fatal(err)
	}
	if needs {
		t.Fatal("equal version must not require snapshot")
	}
}

func TestSchemaVersionMismatchNeedsSnapshot(t *testing.T) {
	store := &schemaStub{version: 4}
	needs, err := crdt.NeedsSnapshot(context.Background(), store, "app1", "ds1", 3)
	if err != nil {
		t.Fatal(err)
	}
	if !needs {
		t.Fatal("server version ahead of client must require full snapshot")
	}
}

func TestSchemaVersionClientAheadNoSnapshot(t *testing.T) {
	store := &schemaStub{version: 2}
	needs, err := crdt.NeedsSnapshot(context.Background(), store, "app1", "ds1", 5)
	if err != nil {
		t.Fatal(err)
	}
	if needs {
		t.Fatal("client version ahead of server: no snapshot needed (server is authoritative)")
	}
}
