package chans

import "sync"

func loop[A, B any](in <-chan A, toClose chan<- B, n int, f func(A)) {
	if n == 1 {
		go func() {
			if toClose != nil {
				defer close(toClose)
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

	if toClose != nil {
		go func() {
			wg.Wait()
			close(toClose)
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

// High level idea:
// Items can be processed in any order, but the output must be in the same order as the input.
// Once item is written to the output, it signals the next item that it can also be written.
// This is done using "canWrite" channel that each item has. Also, each item holds a reference
// to the next item's "canWrite" channel.
//
// Item's "canWrite" channel is passed to user's function "f". Typical "f" function looks like this:
// - Do some calculation
// - Read from "canWrite" channel exactly once. This step is required. Otherwise, behavior is undefined.
// - Write result of the calculation somewhere. This step is optional.
func orderedLoop[A, B any](in <-chan A, toClose chan<- B, n int, f func(a A, canWrite <-chan struct{})) {
	if n == 1 {
		canWrite := makeCanWriteChan()
		close(canWrite)

		go func() {
			if toClose != nil {
				defer close(toClose)
			}

			for a := range in {
				f(a, canWrite)
			}
		}()
		return
	}

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

	if toClose != nil {
		go func() {
			wg.Wait()
			close(toClose)
		}()
	}
}
