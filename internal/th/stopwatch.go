package th

import (
	"sync"
	"time"
)

const never = 10000 * time.Hour

// Stopwatch measures the time elapsed from the first call to Start to the first call to Stop.
type Stopwatch struct {
	startedAt time.Time
	stoppedAt time.Time
	mu        sync.Mutex
}

func (sw *Stopwatch) Start() {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	if sw.startedAt.IsZero() {
		sw.startedAt = time.Now()
	}
}

func (sw *Stopwatch) Stop() {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	if sw.stoppedAt.IsZero() {
		sw.stoppedAt = time.Now()
	}
}

func (sw *Stopwatch) Elapsed() time.Duration {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	if sw.stoppedAt.IsZero() || sw.startedAt.IsZero() {
		return never
	}
	return sw.stoppedAt.Sub(sw.startedAt)
}
