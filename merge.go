package rill

import (
	"github.com/destel/rill/internal/core"
)

// Merge performs a fan-in operation on the list of input channels, returning a single output channel.
// The resulting channel will contain all items from all inputs,
// and will be closed when all inputs are fully consumed.
//
// This is a non-blocking function that processes items from each input sequentially.
//
// See the package documentation for more information on non-blocking functions and error handling.
func Merge[A any](ins ...<-chan A) <-chan A {
	return core.Merge(ins...)
}

// Split2 divides the input stream into two output streams based on the predicate function f:
// The splitting behavior is determined by the boolean return value of f. When f returns true, the item is sent to the outTrue stream,
// otherwise it is sent to the outFalse stream. In case of any error, the item is sent to both output streams.
//
// This is a non-blocking unordered function that processes items concurrently using n goroutines.
// An ordered version of this function, [OrderedSplit2], is also available.
//
// See the package documentation for more information on non-blocking unordered functions and error handling.
func Split2[A any](in <-chan Try[A], n int, f func(A) (bool, error)) (outTrue <-chan Try[A], outFalse <-chan Try[A]) {
	if in == nil {
		return nil, nil
	}

	resOutTrue := make(chan Try[A])
	resOutFalse := make(chan Try[A])
	done := make(chan struct{})

	core.Loop(in, done, n, func(a Try[A]) {
		if a.Error != nil {
			resOutTrue <- a
			resOutFalse <- a
			return
		}

		dir, err := f(a.Value)
		switch {
		case err != nil:
			resOutTrue <- Try[A]{Error: err}
			resOutFalse <- Try[A]{Error: err}
		case dir:
			resOutTrue <- a
		default:
			resOutFalse <- a
		}
	})

	go func() {
		<-done
		close(resOutTrue)
		close(resOutFalse)
	}()

	return resOutTrue, resOutFalse
}

// OrderedSplit2 is the ordered version of [Split2].
func OrderedSplit2[A any](in <-chan Try[A], n int, f func(A) (bool, error)) (outTrue <-chan Try[A], outFalse <-chan Try[A]) {
	if in == nil {
		return nil, nil
	}

	resOutTrue := make(chan Try[A])
	resOutFalse := make(chan Try[A])
	done := make(chan struct{})

	core.OrderedLoop(in, done, n, func(a Try[A], canWrite <-chan struct{}) {
		if a.Error != nil {
			<-canWrite
			resOutTrue <- a
			resOutFalse <- a
			return
		}

		dir, err := f(a.Value)
		<-canWrite
		switch {
		case err != nil:
			resOutTrue <- Try[A]{Error: err}
			resOutFalse <- Try[A]{Error: err}
		case dir:
			resOutTrue <- a
		default:
			resOutFalse <- a
		}
	})

	go func() {
		<-done
		close(resOutTrue)
		close(resOutFalse)
	}()

	return resOutTrue, resOutFalse
}
