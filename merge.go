package rill

import (
	"math/rand"

	"github.com/destel/rill/internal/core"
)

// Merge performs a fan-in operation on the list of input channels, returning a single output channel.
// The resulting channel will contain all items from all input channels,
// and will be closed when all input channels are closed.
func Merge[A any](ins ...<-chan A) <-chan A {
	return core.Merge(ins...)
}

// Split2 divides the input stream into two output streams based on the predicate function f:
// The splitting behavior is:
//   - An error is encountered in the stream - Split2 forwards it to one of the outputs (non-deterministically).
//   - Function f returns an error - Split2 sends it to one of the outputs (non-deterministically).
//   - Function f returns true - Split2 sends the item to outTrue.
//   - Function f returns false - Split2 sends the item to outFalse.
//
// Split2 uses n goroutines to call the user-provided function f concurrently, which means that
// items can be written to the output channels out of order. To preserve the input order, use the [OrderedSplit2] function.
func Split2[A any](in <-chan Try[A], n int, f func(A) (bool, error)) (outTrue <-chan Try[A], outFalse <-chan Try[A]) {
	outs := core.MapAndSplit(in, 2, n, func(a Try[A]) (Try[A], int) {
		if a.Error != nil {
			return a, rand.Int() & 1
		}

		putToTrue, err := f(a.Value)
		switch {
		case err != nil:
			return Try[A]{Error: err}, rand.Int() & 1
		case putToTrue:
			return a, 0
		default:
			return a, 1
		}
	})

	return outs[0], outs[1]
}

// OrderedSplit2 is the ordered version of [Split2].
// It has the same behavior, but orders the items in the output channels will match the order of the input channel.
func OrderedSplit2[A any](in <-chan Try[A], n int, f func(A) (bool, error)) (outTrue <-chan Try[A], outFalse <-chan Try[A]) {
	outs := core.OrderedMapAndSplit(in, 2, n, func(a Try[A]) (Try[A], int) {
		if a.Error != nil {
			return a, rand.Int() & 1
		}

		putToTrue, err := f(a.Value)
		switch {
		case err != nil:
			return Try[A]{Error: err}, rand.Int() & 1
		case putToTrue:
			return a, 0
		default:
			return a, 1
		}
	})

	return outs[0], outs[1]
}
