package core

import (
	"sync"
)

// Loop allows to process items from the input channel concurrently using n goroutines.
// If done channel is not nil, it will be closed after all items are processed.
func Loop[A, B any](in <-chan A, done chan<- B, n int, f func(A)) {
	if n == 1 {
		go func() {
			if done != nil {
				defer close(done)
			}

			for a := range in {
				f(a)
			}
		}()
		return
	}

	var wg sync.WaitGroup

	for range n {
		wg.Go(func() {
			for a := range in {
				f(a)
			}
		})
	}

	if done != nil {
		go func() {
			wg.Wait()
			close(done)
		}()
	}
}

type orderedValue[A any] struct {
	Value        A
	CanWrite     chan struct{}
	NextCanWrite chan struct{}
}

// OrderedLoop is similar to Loop, but it allows to write results to some channel in the same order as items were read from the input.
// If done channel is not nil, it will be closed after all items are processed.
// Special "canWrite" channel is passed to user's function f. Typical f function looks like this:
// - Do some processing (this part is executed concurrently).
// - Read from canWrite channel exactly once. This step is required. Otherwise, behavior is undefined.
// - Write result of the processing somewhere. This step is optional.
// This way processing is done concurrently, but results are written in order.
func OrderedLoop[A, B any](in <-chan A, done chan<- B, n int, f func(a A, canWrite <-chan struct{})) {
	if n == 1 {
		canWrite := make(chan struct{}, 1)
		close(canWrite)

		go func() {
			if done != nil {
				defer close(done)
			}

			for a := range in {
				f(a, canWrite)
			}
		}()
		return
	}

	// High level idea:
	// Each item holds its own canWrite channel and a reference to the next item's canWrite channel.
	// After item is processed and written, it sends a signal to the next item that it can also be written.

	// Pool of signal channels to avoid per-item allocations.
	// Size is O(n). No Reset: f drains its canWrite channel exactly once.
	pool := &Pool[chan struct{}]{
		New: func() chan struct{} { return make(chan struct{}, 1) },
	}

	orderedIn := make(chan orderedValue[A])

	go func() {
		defer close(orderedIn)

		var canWrite, nextCanWrite chan struct{}
		nextCanWrite = pool.Get()
		nextCanWrite <- struct{}{} // first item can be written immediately

		for a := range in {
			canWrite, nextCanWrite = nextCanWrite, pool.Get()
			orderedIn <- orderedValue[A]{a, canWrite, nextCanWrite}
		}
	}()

	var wg sync.WaitGroup
	for range n {
		wg.Go(func() {
			for a := range orderedIn {
				f(a.Value, a.CanWrite)

				pool.Put(a.CanWrite)
				a.NextCanWrite <- struct{}{}
			}
		})
	}

	if done != nil {
		go func() {
			wg.Wait()
			close(done)
		}()
	}
}
