package rill

import (
	"time"

	"github.com/destel/rill/internal/core"
)

// Batch groups items from an input stream into batches based on a maximum size and a timeout.
// A batch is emitted when one of the following conditions is met:
//   - The batch reaches the size of n items
//   - The time since the first item was added to the batch exceeds the timeout
//   - The input channel is closed
//
// This function never emits empty batches. To disable the timeout and emit batches only based on the size,
// set the timeout to -1. Zero timeout is not supported and will result in a panic.
//
// Returns a stream of batches, each containing up to n items.
func Batch[A any](in <-chan Try[A], n int, timeout time.Duration) <-chan Try[[]A] {
	values, errs := ToChans(in)
	batches := core.Batch(values, n, timeout)
	return FromChans(batches, errs)
}

// Unbatch is the inverse of [Batch]. It takes a stream of batches (slices) and returns a stream of individual items.
func Unbatch[A any](in <-chan Try[[]A]) <-chan Try[A] {
	batches, errs := ToChans(in)
	values := core.Unbatch(batches)
	return FromChans(values, errs)
}
