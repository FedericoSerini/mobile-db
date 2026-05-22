package crdt_test

import (
	"math"
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

func TestReceiveRemoteMaxLogicalDoesNotWrap(t *testing.T) {
	c := crdt.NewClock("dev1")
	remote := core.HLC{
		WallTime: time.Now().Add(time.Hour).UnixNano(),
		Logical:  math.MaxUint32,
		DeviceID: "attacker",
	}
	result := c.Receive(remote)
	if !crdt.Before(remote, result) {
		t.Fatalf("result must be strictly after remote; remote=%+v result=%+v", remote, result)
	}
	if result.WallTime != remote.WallTime+1 || result.Logical != 0 {
		t.Fatalf("overflow: want WallTime=%d Logical=0, got %+v", remote.WallTime+1, result)
	}
}

func TestReceiveBothMaxLogical(t *testing.T) {
	c := crdt.NewClock("dev1")
	futureWall := time.Now().Add(time.Hour).UnixNano()
	// Seed local to {futureWall, MaxUint32} by receiving MaxUint32-1 remote.
	// advanceLogicalWall increments MaxUint32-1 to MaxUint32 without overflow.
	c.Receive(core.HLC{WallTime: futureWall, Logical: math.MaxUint32 - 1, DeviceID: "dev2"})
	// Now both local and remote are at futureWall with MaxUint32 — hits dual-max branch.
	remote := core.HLC{WallTime: futureWall, Logical: math.MaxUint32, DeviceID: "dev3"}
	result := c.Receive(remote)
	if result.Logical != 0 || result.WallTime != futureWall+1 {
		t.Fatalf("dual overflow: want WallTime=%d Logical=0, got %+v", futureWall+1, result)
	}
}

func TestTickLogicalOverflowAdvancesWall(t *testing.T) {
	c := crdt.NewClock("dev1")
	futureWall := time.Now().Add(time.Hour).UnixNano()
	atEdge := c.Receive(core.HLC{WallTime: futureWall, Logical: math.MaxUint32, DeviceID: "dev2"})
	next := c.Receive(core.HLC{WallTime: futureWall, Logical: math.MaxUint32, DeviceID: "dev2"})
	if !crdt.Before(atEdge, next) {
		t.Fatal("second overflow-triggered result must be strictly after first")
	}
}

func TestOverflowMaintainsTotalOrder(t *testing.T) {
	c := crdt.NewClock("dev1")
	futureWall := time.Now().Add(time.Hour).UnixNano()
	before := core.HLC{WallTime: futureWall, Logical: math.MaxUint32, DeviceID: "z"}
	after := c.Receive(before)
	if !crdt.Before(before, after) {
		t.Fatalf("post-overflow HLC must be strictly after MaxUint32 HLC; before=%+v after=%+v", before, after)
	}
}
