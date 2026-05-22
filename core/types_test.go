package core_test

import (
	"testing"

	"github.com/federicoserini/mobile-db/core"
)

func TestHLCZeroValue(t *testing.T) {
	var h core.HLC
	if h.WallTime != 0 || h.Logical != 0 {
		t.Fatal("zero HLC should have zero fields")
	}
}

func TestDatasetACLConstants(t *testing.T) {
	acls := []core.DatasetACL{core.ACLPrivate, core.ACLShared, core.ACLPublic}
	for _, a := range acls {
		if string(a) == "" {
			t.Fatalf("ACL constant must not be empty")
		}
	}
}
