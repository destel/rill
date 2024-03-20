package echans

import (
	"math/rand"

	"github.com/destel/rill/chans"
	"github.com/destel/rill/internal/common"
)

// Merge combines multiple input channels into a single output channel. Items are emitted as soon as they're available,
// so the output order is not defined.
func Merge[A any](ins ...<-chan A) <-chan A {
	return chans.Merge(ins...)
}

// Split2 divides the input channel into two output channels based on the discriminator function f, using n goroutines for concurrency.
// The function f takes an item from the input and decides which output channel (out0 or out1) it should go to by returning 0 or 1, respectively.
// Return values other than 0 or 1 lead to the item being discarded.
// If an error is encountered, either from the function f itself or from upstream it is intentionally sent
// to one of the output channels in a non-deterministic manner.
// The output order is not guaranteed: results are written to the outputs as soon as they're ready.
// Use [OrderedSplit2] to preserve the input order.
func Split2[A any](in <-chan Try[A], n int, f func(A) (int, error)) (out0 <-chan Try[A], out1 <-chan Try[A]) {
	outs := common.MapAndSplit(in, 2, n, func(a Try[A]) (Try[A], int) {
		if a.Error != nil {
			return a, rand.Int() & 1
		}

		i, err := f(a.V)
		if err != nil {
			return Try[A]{Error: err}, rand.Int() & 1
		}

		return a, i
	})

	return outs[0], outs[1]
}

// OrderedSplit2 is similar to [Split2], but it guarantees that the order of the outputs matches the order of the input.
func OrderedSplit2[A any](in <-chan Try[A], n int, f func(A) (int, error)) (out0 <-chan Try[A], out1 <-chan Try[A]) {
	outs := common.OrderedMapAndSplit(in, 2, n, func(a Try[A]) (Try[A], int) {
		if a.Error != nil {
			return a, rand.Int() & 1
		}

		i, err := f(a.V)
		if err != nil {
			return Try[A]{Error: err}, rand.Int() & 1
		}

		return a, i
	})

	return outs[0], outs[1]
}
