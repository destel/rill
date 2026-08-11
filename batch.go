package rill

import (
	"time"
)

// Batch takes a stream of items and returns a stream of batches based on a maximum size and a timeout.
//
// A batch is emitted when one of the following conditions is met:
//   - The batch reaches the maximum size
//   - The time since the first item was added to the batch exceeds the timeout
//   - An error is encountered in the input stream
//   - The input stream is closed
//
// Errors are never included in batches. Each error is forwarded to the output as a separate item,
// preserving the relative order of values and errors.
//
// This function never emits empty batches. To disable the timeout and emit batches only based on the size,
// set the timeout to -1. Setting the timeout to zero is not supported and will result in a panic
//
// This is a non-blocking ordered function that processes items sequentially.
//
// See the package documentation for more information on non-blocking ordered functions and error handling.
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
	t := time.NewTimer(1 * time.Hour)
	t.Stop()

	var batch []A

	flush := func() {
		t.Stop() // no need to drain t.C since Go 1.23

		if len(batch) > 0 {
			out <- Try[[]A]{Value: batch}
			batch = make([]A, 0, size)
		}
	}

	sendError := func(err error) {
		out <- Try[[]A]{Error: err}
	}

	// infinite timeout
	if timeout < 0 {
		go func() {
			defer close(out)
			defer flush()

			for x := range in {
				if x.Error != nil {
					flush()
					sendError(x.Error)
					continue
				}

				batch = append(batch, x.Value)
				if len(batch) >= size {
					flush()
				}
			}
		}()

		return out
	}

	// finite timeout
	go func() {
		defer close(out)
		defer flush()

		for {
			select {
			case <-t.C:
				flush()

			case x, ok := <-in:
				if !ok {
					return
				}

				if x.Error != nil {
					flush()
					sendError(x.Error)
					continue
				}

				batch = append(batch, x.Value)
				if len(batch) >= size {
					flush()
					continue
				}

				if len(batch) == 1 {
					// First item was just added to the batch - start the timer.
					// The invariant: no item sits in the batcher longer than timeout.
					t.Reset(timeout)
				}
			}
		}
	}()

	return out
}

// Unbatch is the inverse of [Batch]. It takes a stream of batches and returns a stream of individual items.
//
// This is a non-blocking ordered function that processes items sequentially.
// See the package documentation for more information on non-blocking ordered functions and error handling.
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
