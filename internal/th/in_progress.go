package th

import "sync"

// Deprecated: use ConcurrencyMonitor instead
type InProgressCounter struct {
	mu      sync.Mutex
	current int
	max     int
}

func (c *InProgressCounter) add(delta int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.current += delta
	if c.max < c.current {
		c.max = c.current
	}
}

func (c *InProgressCounter) Inc() {
	c.add(1)
}

func (c *InProgressCounter) Dec() {
	c.add(-1)
}

func (c *InProgressCounter) Current() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.current
}

func (c *InProgressCounter) Max() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.max
}
