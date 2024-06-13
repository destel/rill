package th

import (
	"sync"
	"time"
)

// ConcurrencyMonitor measures the maximum concurrency level reached by goroutines.
// It enforces maximum possible concurrency by requiring each goroutine to call Inc() at the start and Dec() at the end of its execution.
// Goroutines calling Inc() are blocked until the concurrency level remains stable for a specified time window, ensuring that concurrency peaks are accurately captured.
// The highest level of concurrency observed can be retrieved using the Max() method.
type ConcurrencyMonitor struct {
	cond    *sync.Cond
	current int
	max     int

	target int
	window time.Duration

	lastChangeAt time.Time
	timer        *time.Timer
	timerFired   bool
}

func NewConcurrencyMonitor(window time.Duration) *ConcurrencyMonitor {
	c := &ConcurrencyMonitor{
		cond:   sync.NewCond(&sync.Mutex{}),
		window: window,
	}

	c.timer = time.AfterFunc(1*time.Hour, func() {
		c.cond.L.Lock()
		defer c.cond.L.Unlock()

		c.timerFired = true
		c.cond.Broadcast()
	})

	return c
}

func (c *ConcurrencyMonitor) Inc() {
	c.cond.L.Lock()
	defer c.cond.L.Unlock()

	c.lastChangeAt = time.Now()
	if !c.timerFired {
		c.timer.Reset(c.window)
	}

	c.current++
	if c.max < c.current {
		c.max = c.current
	}

	// block all goroutines unless "window" has passed since the last counter change
	for !c.timerFired && time.Since(c.lastChangeAt) < c.window {
		c.cond.Wait()
	}
}

func (c *ConcurrencyMonitor) Dec() {
	c.cond.L.Lock()
	defer c.cond.L.Unlock()

	c.lastChangeAt = time.Now()
	if !c.timerFired {
		c.timer.Reset(c.window)
	}

	c.current--
	c.cond.Broadcast()
}

func (c *ConcurrencyMonitor) Reset() int {
	c.cond.L.Lock()
	defer c.cond.L.Unlock()

	if c.timer != nil {
		c.timer.Stop()
	}

	c.current = 0
	c.max = 0
	c.lastChangeAt = time.Time{}
	c.timer.Reset(1 * time.Hour)
	c.timerFired = false
	return c.max
}

func (c *ConcurrencyMonitor) Max() int {
	c.cond.L.Lock()
	defer c.cond.L.Unlock()

	return c.max
}
