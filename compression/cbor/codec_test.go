package cbor_test

import (
	"testing"

	"github.com/federicoserini/mobile-db/compression/cbor"
	"github.com/federicoserini/mobile-db/core"
)

func TestCBORRoundTrip(t *testing.T) {
	c := cbor.NewCodec()
	msg := core.SyncMessage{
		AppID: "app1", UserID: "user1", DatasetID: "ds1",
		Clock: core.HLC{WallTime: 1000, Logical: 2, DeviceID: "dev1"},
		Ops: []core.CRDTOp{
			{OpID: "op1", DocID: "doc1", Field: "name", Value: "Alice",
				Timestamp: core.HLC{WallTime: 900, DeviceID: "dev1"}},
		},
	}
	data, err := c.Encode(msg)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	var decoded core.SyncMessage
	if err := c.Decode(data, &decoded); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if decoded.AppID != "app1" || len(decoded.Ops) != 1 {
		t.Fatalf("decoded mismatch: %+v", decoded)
	}
}

func TestCBORInterfaceCompliance(t *testing.T) {
	var _ interface {
		Encode(any) ([]byte, error)
		Decode([]byte, any) error
	} = cbor.NewCodec()
}
