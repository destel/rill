package rill

import (
	"time"
)

// Batch groups consecutive values of the stream into batches. With
// timeout = -1, it accumulates values until the batch reaches size,
// then emits it. When the input closes, any remaining values are
// emitted as a final batch.
//
// Input errors create batch boundaries: any pending batch is emitted
// first, followed by the error as a separate item. This function never
// emits empty batches.
//
// A positive timeout adds another trigger: each batch has that much
// time to fill, starting from its first value. When the timeout expires,
// the pending batch is emitted even if it is not full. This trades batch
// size for latency: sparse input produces smaller batches, but no value
// is ever held longer than timeout. Backpressure can still delay delivery
// beyond the timeout.
//
// A zero timeout panics. Use a small positive timeout instead.
func Batch[A any](in <-chan Try[A], size int, timeout time.Duration) <-chan Try[[]A] {
	validateMinSize(size, 1)
	if timeout == 0 {
		// Zero timeout reads as "batch greedily until reading from the input blocks". With an unbuffered
		// input channel, "reading blocks" is a scheduler accident, not an end-of-burst signal,
		// so this degenerates into floods of 1-item batches. A small positive timeout is the
		// reliable way to get the intended behavior.
		panic("rill: zero timeout is not supported")
	}

	if in == nil {
		return nil
	}

	out := make(chan Try[[]A])

	go func() {
		defer close(out)

		t := time.NewTimer(1 * time.Hour)
		t.Stop()

		var batch []A

		flush := func() {
			t.Stop() // no need to drain t.C since Go 1.23

			if len(batch) > 0 {
				out <- Try[[]A]{Value: batch}
				batch = nil
			}
		}

		sendError := func(err error) {
			flush()
			out <- Try[[]A]{Error: err}
		}

		send := func(x A) {
			if batch == nil {
				batch = make([]A, 0, size)
			}
			batch = append(batch, x)
			if len(batch) >= size {
				flush()
			}
		}

		defer flush()

		// infinite timeout
		if timeout < 0 {
			for x := range in {
				if x.Error != nil {
					sendError(x.Error)
					continue
				}

				send(x.Value)
			}
			return
		}

		// finite timeout
		for {
			select {
			case <-t.C:
				flush()

			case x, ok := <-in:
				if !ok {
					return
				}

				if x.Error != nil {
					sendError(x.Error)
					continue
				}

				send(x.Value)

				if len(batch) == 1 {
					// x became the first item in a new batch - start the timer.
					t.Reset(timeout)
				}
			}
		}
	}()

	return out
}

// Unbatch flattens a stream of slices into a stream of their values.
// This function is the inverse of [Batch].
func Unbatch[A any](in <-chan Try[[]A]) <-chan Try[A] {
	if in == nil {
		return nil
	}

	out := make(chan Try[A])

	go func() {
		defer close(out)
		for x := range in {
			if x.Error != nil {
				out <- Try[A]{Error: x.Error}
				continue
			}

			for _, a := range x.Value {
				out <- Try[A]{Value: a}
			}
		}
	}()

	return out
}
