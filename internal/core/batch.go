package core

import (
	"fmt"
	"time"
)

// Batch groups items from an input channel into batches based on a maximum size and a timeout.
// A batch is emitted when it reaches the maximum size, the timeout expires, or the input channel closes.
// This function never emits empty batches. The timeout countdown starts when the first item is added to a new batch.
// To emit batches only when full, set the timeout to -1. Zero timeout is not supported and will panic.
func Batch[A any](in <-chan A, n int, timeout time.Duration) <-chan []A {
	if in == nil {
		return nil
	}

	out := make(chan []A)

	switch {
	case timeout == 0:
		panic(fmt.Errorf("zero timeout is not supported yet"))

	case timeout < 0:
		// infinite timeout
		go func() {
			defer close(out)
			var batch []A
			for a := range in {
				batch = append(batch, a)
				if len(batch) >= n {
					out <- batch
					batch = make([]A, 0, n)
				}
			}
			if len(batch) > 0 {
				out <- batch
			}
		}()

	default:
		// finite timeout
		go func() {
			batch := make([]A, 0, n)
			t := time.NewTicker(1 * time.Hour)
			t.Stop()

			flush := func() {
				if len(batch) > 0 {
					out <- batch
					batch = make([]A, 0, n)
				}

				t.Stop()
				// consume a tick that might have been sent while we were flushing
				select {
				case <-t.C:
				default:
				}
			}

			for {
				select {
				case <-t.C:
					// timeout
					flush()

				case a, ok := <-in:
					if !ok {
						// end of input
						flush()
						close(out)
						return
					}

					// got new item
					batch = append(batch, a)

					if len(batch) == 1 {
						// we've just started collecting a new batch.
						// start the timer to flush the batch after the timeout.
						t.Reset(timeout)
					}

					if len(batch) >= n {
						// batch is full
						flush()
					}
				}

			}
		}()

	}

	return out
}

// Unbatch is the inverse of Batch. It takes a channel of batches and emits individual items.
func Unbatch[A any](in <-chan []A) <-chan A {
	if in == nil {
		return nil
	}

	out := make(chan A)

	go func() {
		defer close(out)
		for batch := range in {
			for _, a := range batch {
				out <- a
			}
		}
	}()

	return out
}
