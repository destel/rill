package th

import (
	"sync"
	"time"
)

// ConcurrencyMonitor measures the peak number of goroutines simultaneously
// inside an instrumented region: each calls Enter at the start and Exit at the
// end. Enter sleeps for Hold so concurrent callers pile up before any leaves,
// and Max reports the peak. Only meaningful inside a testing/synctest bubble,
// where the fake clock makes the pile-up deterministic.
type ConcurrencyMonitor struct {
	// Hold is how long Enter parks the caller. Zero defaults to 1s; the exact
	// value is irrelevant under synctest. Set before first use.
	Hold time.Duration

	mu      sync.Mutex
	current int
	max     int
}

func (c *ConcurrencyMonitor) Enter() {
	c.mu.Lock()
	c.current++
	if c.current > c.max {
		c.max = c.current
	}
	c.mu.Unlock()

	hold := c.Hold
	if hold <= 0 {
		hold = 1 * time.Second // default
	}

	time.Sleep(hold) // must stay outside the lock
}

func (c *ConcurrencyMonitor) Exit() {
	c.mu.Lock()
	c.current--
	c.mu.Unlock()
}

func (c *ConcurrencyMonitor) Max() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.max
}
