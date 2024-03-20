package rill

import (
	"time"

	"github.com/destel/rill/internal/common"
)

// Batch groups items from an input channel into batches based on a maximum size and a timeout.
// A batch is emitted when it reaches the maximum size, the timeout expires, or the input channel closes.
// To emit batches only when full, set the timeout to -1. This function never emits empty batches.
// The timeout countdown starts when the first item is added to a new batch.
func Batch[A any](in <-chan Try[A], n int, timeout time.Duration) <-chan Try[[]A] {
	values, errs := Unwrap(in)
	batches := common.Batch(values, n, timeout)
	return WrapAsync(batches, errs)
}

// Unbatch is the inverse of Batch. It takes a channel of batches and emits individual items.
func Unbatch[A any](in <-chan Try[[]A]) <-chan Try[A] {
	batches, errs := Unwrap(in)
	values := common.Unbatch(batches)
	return WrapAsync(values, errs)
}
