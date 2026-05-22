package crdt_test

import (
	"testing"

	"github.com/federicoserini/mobile-db/core"
	"github.com/federicoserini/mobile-db/core/crdt"
)

func TestMergePicksLatest(t *testing.T) {
	early := core.CRDTOp{
		OpID: "op1", DocID: "doc1", Field: "name", Value: "Alice",
		Timestamp: core.HLC{WallTime: 100, DeviceID: "dev1"},
	}
	late := core.CRDTOp{
		OpID: "op2", DocID: "doc1", Field: "name", Value: "Bob",
		Timestamp: core.HLC{WallTime: 200, DeviceID: "dev2"},
	}

	result := crdt.Merge([]core.CRDTOp{early, late})
	if result["doc1"]["name"] != "Bob" {
		t.Fatalf("want Bob, got %v", result["doc1"]["name"])
	}
}

func TestMergeIsIdempotent(t *testing.T) {
	op := core.CRDTOp{
		OpID: "op1", DocID: "doc1", Field: "x", Value: 42,
		Timestamp: core.HLC{WallTime: 1, DeviceID: "dev1"},
	}
	r1 := crdt.Merge([]core.CRDTOp{op})
	r2 := crdt.Merge([]core.CRDTOp{op, op})
	if r1["doc1"]["x"] != r2["doc1"]["x"] {
		t.Fatal("merge must be idempotent (duplicate ops produce same result)")
	}
}

func TestMergeIsCommutative(t *testing.T) {
	a := core.CRDTOp{OpID: "op1", DocID: "d", Field: "f", Value: "A",
		Timestamp: core.HLC{WallTime: 1, DeviceID: "x"}}
	b := core.CRDTOp{OpID: "op2", DocID: "d", Field: "f", Value: "B",
		Timestamp: core.HLC{WallTime: 2, DeviceID: "y"}}

	r1 := crdt.Merge([]core.CRDTOp{a, b})
	r2 := crdt.Merge([]core.CRDTOp{b, a})
	if r1["d"]["f"] != r2["d"]["f"] {
		t.Fatal("merge must be commutative (order of ops does not matter)")
	}
}

func TestMergeMultipleFields(t *testing.T) {
	ops := []core.CRDTOp{
		{OpID: "1", DocID: "doc1", Field: "name", Value: "Alice",
			Timestamp: core.HLC{WallTime: 1, DeviceID: "dev1"}},
		{OpID: "2", DocID: "doc1", Field: "age", Value: 30,
			Timestamp: core.HLC{WallTime: 2, DeviceID: "dev1"}},
	}
	result := crdt.Merge(ops)
	if result["doc1"]["name"] != "Alice" || result["doc1"]["age"] != 30 {
		t.Fatalf("unexpected result: %v", result)
	}
}
