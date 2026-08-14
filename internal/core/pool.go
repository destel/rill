package core

import "sync"

// Pool is a pool of reusable values. It grows on demand and never shrinks,
// it's caller's responsibility to ensure that the pool stays bounded.
//
// A Pool is safe for concurrent use, unless Unsynchronized is set. Its fields
// must not be changed once the pool is in use.
type Pool[T any] struct {
	// New creates a value when the pool is empty. It is required.
	New func() T

	// Reset prepares a value for reuse. It may be nil.
	Reset func(T)

	// Unsynchronized disables the pool's own locking, leaving the caller
	// responsible for serializing Get and Put. It is for callers that already
	// hold a lock covering both.
	Unsynchronized bool

	mu    sync.Mutex
	items []T
}

// Get returns a value from the pool, or a new one if the pool is empty.
func (p *Pool[T]) Get() T {
	if !p.Unsynchronized {
		p.mu.Lock()
	}

	n := len(p.items)
	if n == 0 {
		if !p.Unsynchronized {
			p.mu.Unlock()
		}

		return p.New()
	}

	res := p.items[n-1]

	// clear the slot: the backing array would keep the value alive otherwise
	var zero T
	p.items[n-1] = zero
	p.items = p.items[:n-1]

	if !p.Unsynchronized {
		p.mu.Unlock()
	}

	return res
}

// Put returns a value to the pool, transferring its ownership: the value must
// not be used afterward.
func (p *Pool[T]) Put(item T) {
	if p.Reset != nil {
		p.Reset(item)
	}

	if !p.Unsynchronized {
		p.mu.Lock()
	}

	p.items = append(p.items, item)

	if !p.Unsynchronized {
		p.mu.Unlock()
	}
}
