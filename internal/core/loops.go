package core

import (
	"sync"
	"sync/atomic"
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

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for a := range in {
				f(a)
			}
			return
		}()
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

var canWritePool sync.Pool

func makeCanWriteChan() chan struct{} {
	ch := canWritePool.Get()
	if ch == nil {
		return make(chan struct{}, 1)
	}
	return ch.(chan struct{})
}

func releaseCanWriteChan(ch chan struct{}) {
	canWritePool.Put(ch)
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
		canWrite := makeCanWriteChan()
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

	orderedIn := make(chan orderedValue[A])

	go func() {
		defer close(orderedIn)

		var canWrite, nextCanWrite chan struct{}
		nextCanWrite = makeCanWriteChan()
		nextCanWrite <- struct{}{} // first item can be written immediately

		for a := range in {
			canWrite, nextCanWrite = nextCanWrite, makeCanWriteChan()
			orderedIn <- orderedValue[A]{a, canWrite, nextCanWrite}
		}
	}()

	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for a := range orderedIn {
				f(a.Value, a.CanWrite)

				releaseCanWriteChan(a.CanWrite)
				a.NextCanWrite <- struct{}{}
			}
		}()
	}

	if done != nil {
		go func() {
			wg.Wait()
			close(done)
		}()
	}
}

// Breakable creates a new channel from channel in, copying elements until doBreak function is called.
// doBreak halts copying and discards any remaining elements from in, closing the resulting channel.
// Use this to stop channel processing when needed.
func Breakable[A any](in <-chan A) (res <-chan A, doBreak func()) {
	if in == nil {
		return nil, func() {}
	}

	breakCalled := int64(0)
	breakFunc := func() {
		atomic.StoreInt64(&breakCalled, 1)
	}

	out := make(chan A)
	go func() {
		defer Drain(in) // discard unconsumed items
		defer close(out)

		for x := range in {
			if atomic.LoadInt64(&breakCalled) == 1 {
				break
			}
			out <- x
		}
	}()

	return out, breakFunc
}
