package core

import (
	"sync"
)

// Loop processes items from the input channel concurrently using n goroutines.
// If the done channel is not nil, it will be closed after all items are processed.
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

// OrderedLoop is similar to Loop, but it allows the caller to write results to a
// channel in the same order as the items were read from the input.
// If the done channel is not nil, it will be closed after all items are processed.
// A special "canWrite" channel is passed to the user's function f.
// A typical implementation of f performs the following steps:
//
//   - Do some processing (this part is executed concurrently).
//   - Read from the canWrite channel exactly once. This step is required. Otherwise, behavior is undefined.
//   - Write the result of the processing somewhere. This step is optional.
//
// This way, processing is done concurrently, but results are written in order.
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

	// High level idea: permission passing chain.
	// Each item is associated with a canWrite channel, and knows next item's canWrite channel.
	// After the item is processed and written, it signals the next item that it can also be written.

	// Workers pull from the input themselves. inputMu makes receiving an item and
	// chain linking atomic and serialized. Everything else happens concurrently.
	var inputMu DurableMutex

	// A channel that the next item will wait for to be written.
	// Initially filled, so the very first item can be written immediately.
	nextCanWrite := make(chan struct{}, 1)
	nextCanWrite <- struct{}{}

	var wg sync.WaitGroup
	for range n {
		wg.Go(func() {
			// We write to this channel when the current item is processed and written
			itemDone := make(chan struct{}, 1)

			for {
				inputMu.Lock()

				a, ok := <-in
				if !ok {
					inputMu.Unlock()
					return
				}

				canWrite := nextCanWrite // wait for this
				nextCanWrite = itemDone  // our itemDone becomes the next item's canWrite

				inputMu.Unlock()

				f(a, canWrite)
				itemDone <- struct{}{}

				// No need to allocate a new channel:
				// canWrite was drained by f, and is not used by anyone anymore, so we can reuse it
				itemDone = canWrite
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
