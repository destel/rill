package rill

import (
	"math/rand"

	"github.com/destel/rill/internal/core"
)

// Merge performs a fan-in operation on the list of input streams, returning a single output stream.
// The resulting stream will contain all items from all inputs,
// and will be closed when all input streams are fully consumed.
//
// This is a non-blocking function that processes items from each input sequentially.
//
// See the package documentation for more information on non-blocking functions and error handling.
func Merge[A any](ins ...<-chan A) <-chan A {
	return core.Merge(ins...)
}

// Split2 divides the input stream into two output streams based on the predicate function f:
// The splitting behavior is determined by the boolean return value of f. When f returns true, the item is sent to the outTrue stream,
// otherwise it is sent to the outFalse stream. In case of any error, the item is sent to one of the output streams in a non-deterministic way.
//
// This is a non-blocking unordered function that processes items concurrently using n goroutines.
// An ordered version of this function, [OrderedSplit2], is also available.
//
// See the package documentation for more information on non-blocking unordered functions and error handling.
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
