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
// Both output streams must be consumed independently to avoid deadlocks.
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

// Tee returns two streams that are identical to the input stream (both errors and values).
// Both output streams must be consumed independently to avoid deadlocks.
//
// This is a non-blocking function that processes items in a single goroutine.
// See the package documentation for more information on non-blocking functions and error handling.
//
// If deep copying of values is needed, use [Map] on one or both outputs:
//
//	out1, out2 := rill.Tee(in)
//	out2 = rill.Map(out2, 1, func(x A) (A, error) {
//		return deepCopy(x), nil
//	})
func Tee[A any](in <-chan Try[A]) (<-chan Try[A], <-chan Try[A]) {
	if in == nil {
		return nil, nil
	}

	out1 := make(chan Try[A])
	out2 := make(chan Try[A])

	go func() {
		defer close(out1)
		defer close(out2)

		for x := range in {
			out1 <- x
			out2 <- x
		}
	}()

	return out1, out2
}
