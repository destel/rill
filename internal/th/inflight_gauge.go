package th

import (
	"sync"
)

// InFlightGauge measures the peak number of goroutines in flight inside an
// instrumented region: each goroutine calls Enter when it arrives and Exit when
// it leaves, and Max reports the high-water mark of how many were in flight at
// once.
type InFlightGauge struct {
	mu      sync.Mutex
	current int
	max     int
}

func (g *InFlightGauge) Enter() {
	g.mu.Lock()
	g.current++
	g.max = max(g.max, g.current)
	g.mu.Unlock()
}

func (g *InFlightGauge) Exit() {
	g.mu.Lock()
	g.current--
	g.mu.Unlock()
}

func (g *InFlightGauge) Max() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.max
}
