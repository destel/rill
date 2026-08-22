package rill

import (
	"time"
)

// Batch groups consecutive values of the stream into batches. In its
// simplest form, with timeout = -1 and no errors in the input, Batch
// accumulates values into a pending batch and emits it as soon as it
// reaches the target size.
//
// A positive timeout is the time each batch has to fill, starting from
// its first value. When it expires, the pending batch is emitted even
// if it is not full. This trades batch size for latency: batches can be
// smaller when the input is sparse, but no value is ever held longer
// than timeout.
//
// A zero timeout panics: the expected behavior would be to accumulate
// until reading from the input blocks, but in practice, with an
// unbuffered input, that often produces a flood of one-item batches.
// Use a small positive timeout instead.
//
// Input errors become batch boundaries: the pending batch, if not
// empty, is emitted first, and the error follows as a separate item.
//
// When the end of the input is reached, whatever has accumulated is
// emitted as a final batch. This function never emits empty batches,
// regardless of what triggered the emission.
//
// See the package documentation for the behaviors that all stages share.
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
//
// See the package documentation for the behaviors that all stages share.
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
