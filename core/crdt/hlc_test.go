package crdt_test

import (
	"testing"
	"time"

	"github.com/federicoserini/mobile-db/core"
	"github.com/federicoserini/mobile-db/core/crdt"
)

func TestHLCTickMonotonic(t *testing.T) {
	c := crdt.NewClock("dev1")
	a := c.Tick()
	b := c.Tick()
	if !crdt.Before(a, b) {
		t.Fatal("second Tick must be strictly after first")
	}
}

func TestHLCReceiveAdoptsFutureWall(t *testing.T) {
	c := crdt.NewClock("dev1")
	future := core.HLC{
		WallTime: time.Now().Add(5 * time.Second).UnixNano(),
		Logical:  0,
		DeviceID: "dev2",
	}
	updated := c.Receive(future)
	if updated.WallTime < future.WallTime {
		t.Fatal("Receive must adopt remote wall time when it is ahead of local")
	}
}

func TestHLCBeforeTotalOrder(t *testing.T) {
	cases := []struct {
		a, b core.HLC
		want bool
	}{
		{core.HLC{WallTime: 1}, core.HLC{WallTime: 2}, true},
		{core.HLC{WallTime: 2}, core.HLC{WallTime: 1}, false},
		{core.HLC{WallTime: 1, Logical: 0}, core.HLC{WallTime: 1, Logical: 1}, true},
		{core.HLC{WallTime: 1, Logical: 1, DeviceID: "a"}, core.HLC{WallTime: 1, Logical: 1, DeviceID: "b"}, true},
	}
	for _, tc := range cases {
		if crdt.Before(tc.a, tc.b) != tc.want {
			t.Fatalf("Before(%+v, %+v) = %v, want %v", tc.a, tc.b, !tc.want, tc.want)
		}
	}
}
