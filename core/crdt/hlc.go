package crdt

import (
	"math"
	"sync"
	"time"

	"github.com/federicoserini/mobile-db/core"
)

type Clock struct {
	mu       sync.Mutex
	last     core.HLC
	deviceID string
}

func NewClock(deviceID string) *Clock {
	return &Clock{deviceID: deviceID}
}

// advanceLogicalWall increments *logical and returns wallTime unchanged.
// If *logical is already at MaxUint32, resets it to 0 and returns wallTime+1.
// The returned value must replace the wall time to preserve total order.
func advanceLogicalWall(wallTime int64, logical *uint32) int64 {
	if *logical == math.MaxUint32 {
		*logical = 0
		return wallTime + 1
	}
	*logical++
	return wallTime
}

// Tick returns a new HLC guaranteed strictly greater than all previous Ticks.
func (c *Clock) Tick() core.HLC {
	c.mu.Lock()
	defer c.mu.Unlock()
	wall := time.Now().UnixNano()
	if wall <= c.last.WallTime {
		if c.last.Logical == math.MaxUint32 {
			c.last.WallTime++
			c.last.Logical = 0
		} else {
			c.last.Logical++
		}
	} else {
		c.last.WallTime = wall
		c.last.Logical = 0
	}
	c.last.DeviceID = c.deviceID
	return c.last
}

// Receive merges a remote HLC and returns the updated local HLC.
func (c *Clock) Receive(remote core.HLC) core.HLC {
	c.mu.Lock()
	defer c.mu.Unlock()
	wall := time.Now().UnixNano()
	maxWall := max3(wall, c.last.WallTime, remote.WallTime)
	switch {
	case maxWall == c.last.WallTime && maxWall == remote.WallTime:
		c.last.Logical = max2u(c.last.Logical, remote.Logical)
		maxWall = advanceLogicalWall(maxWall, &c.last.Logical)
	case maxWall == c.last.WallTime:
		maxWall = advanceLogicalWall(maxWall, &c.last.Logical)
	case maxWall == remote.WallTime:
		c.last.Logical = remote.Logical
		maxWall = advanceLogicalWall(maxWall, &c.last.Logical)
	default:
		c.last.Logical = 0
	}
	c.last.WallTime = maxWall
	c.last.DeviceID = c.deviceID
	return c.last
}

// Before returns true if a is strictly before b in total order.
func Before(a, b core.HLC) bool {
	if a.WallTime != b.WallTime {
		return a.WallTime < b.WallTime
	}
	if a.Logical != b.Logical {
		return a.Logical < b.Logical
	}
	return a.DeviceID < b.DeviceID
}

func max3(a, b, c int64) int64 {
	if a >= b && a >= c {
		return a
	}
	if b >= c {
		return b
	}
	return c
}

func max2u(a, b uint32) uint32 {
	if a > b {
		return a
	}
	return b
}
