package echans

import (
	"time"

	"github.com/destel/rill/chans"
)

func Batch[A any](in <-chan Try[A], n int, timeout time.Duration) <-chan Try[[]A] {
	values, errs := Unwrap(in)
	batches := chans.Batch(values, n, timeout)
	return WrapAsync(batches, errs)
}

func Unbatch[A any](in <-chan Try[[]A]) <-chan Try[A] {
	batches, errs := Unwrap(in)
	values := chans.Unbatch(batches)
	return WrapAsync(values, errs)
}
