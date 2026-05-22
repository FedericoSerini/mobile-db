package crdt

import (
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

// Tick returns a new HLC guaranteed strictly greater than all previous Ticks.
func (c *Clock) Tick() core.HLC {
	c.mu.Lock()
	defer c.mu.Unlock()
	wall := time.Now().UnixNano()
	if wall <= c.last.WallTime {
		c.last.Logical++
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
		c.last.Logical = max2u(c.last.Logical, remote.Logical) + 1
	case maxWall == c.last.WallTime:
		c.last.Logical++
	case maxWall == remote.WallTime:
		c.last.Logical = remote.Logical + 1
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
