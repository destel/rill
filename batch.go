package rill

import (
	"time"

	"github.com/destel/rill/internal/core"
)

// Batch take a stream of items and returns a stream of batches based on a maximum size and a timeout.
//
// A batch is emitted when one of the following conditions is met:
//   - The batch reaches the maximum size
//   - The time since the first item was added to the batch exceeds the timeout
//   - The input stream is closed
//
// This function never emits empty batches. To disable the timeout and emit batches only based on the size,
// set the timeout to -1. Setting the timeout to zero is not supported and will result in a panic
//
// This is a non-blocking ordered function that processes items sequentially.
//
// See the package documentation for more information on non-blocking ordered functions and error handling.
func Batch[A any](in <-chan Try[A], size int, timeout time.Duration) <-chan Try[[]A] {
	values, errs := ToChans(in)
	batches := core.Batch(values, size, timeout)
	return FromChans(batches, errs)
}

// Unbatch is the inverse of [Batch]. It takes a stream of batches and returns a stream of individual items.
//
// This is a non-blocking ordered function that processes items sequentially.
// See the package documentation for more information on non-blocking ordered functions and error handling.
func Unbatch[A any](in <-chan Try[[]A]) <-chan Try[A] {
	batches, errs := ToChans(in)
	values := core.Unbatch(batches)
	return FromChans(values, errs)
}
