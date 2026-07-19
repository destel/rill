package rill

import (
	"errors"
	"sync/atomic"
)

// ForEach applies a function f to each item in an input stream.
//
// This is a blocking unordered function that processes items concurrently using n goroutines.
// When n = 1, processing becomes sequential, making the function ordered and similar to a regular for-range loop.
//
// See the package documentation for more information on blocking unordered functions and error handling.
func ForEach[A any](in <-chan Try[A], n int, f func(A) error) error {
	// The n = 1 path is an internal contract, not just an optimization.
	// Other sinks (Any, Reduce) build their n = 1 behavior on it and rely on:
	//   - items processed sequentially, in stream order
	//   - f executed in the calling goroutine
	//   - return only after the loop exits, so state captured by f is safe
	//     to use after ForEach returns
	if n == 1 {
		defer Discard(in)

		for a := range in {
			err := a.Error
			if err == nil {
				err = f(a.Value)
			}
			if err != nil {
				return err
			}
		}
		return nil
	}

	var suppressCallbacks atomic.Bool
	defer suppressCallbacks.Store(true)

	out := FilterMap(in, n, func(a A) (struct{}, bool, error) {
		if suppressCallbacks.Load() {
			return struct{}{}, false, nil
		}
		return struct{}{}, false, f(a)
	})

	return Err(out)
}

// Err returns the first error encountered in the input stream or nil if there were no errors.
//
// This is a blocking ordered function that processes items sequentially.
// See the package documentation for more information on blocking ordered functions and error handling.
func Err[A any](in <-chan Try[A]) error {
	defer Discard(in)

	for a := range in {
		if a.Error != nil {
			return a.Error
		}
	}

	return nil
}

// First returns the first item or error encountered in the input stream, whichever comes first.
// The found return flag reports whether a value was found: it is set to false
// if the stream was empty or if an error was encountered first.
//
// This is a blocking ordered function that processes items sequentially.
// See the package documentation for more information on blocking ordered functions and error handling.
func First[A any](in <-chan Try[A]) (value A, found bool, err error) {
	defer Discard(in)

	for a := range in {
		return a.Value, a.Error == nil, a.Error
	}

	var zero A
	return zero, false, nil
}

// errFound is a control-flow sentinel, compared by identity - the fs.SkipDir
// pattern. Shared by Any and All: both short-circuit when the search finds its
// target (a match, or a counterexample). It never escapes a sink, so the
// sharing cannot contaminate across calls.
var errFound = errors.New("found")

// Any checks if there is an item in the input stream that satisfies the condition f.
// This function returns true as soon as it finds such an item. Otherwise, it returns false.
//
// Any is a blocking unordered function that processes items concurrently using n goroutines.
// When n = 1, processing becomes sequential, making the function ordered.
//
// See the package documentation for more information on blocking unordered functions and error handling.
func Any[A any](in <-chan Try[A], n int, f func(A) (bool, error)) (bool, error) {
	err := ForEach(in, n, func(a A) error {
		ok, err := f(a)
		if err != nil {
			return err
		}
		if ok {
			return errFound
		}
		return nil
	})

	if err == errFound { //nolint:errorlint
		return true, nil
	}
	return false, err
}

// All checks if all items in the input stream satisfy the condition f.
// This function returns false as soon as it finds an item that does not satisfy the condition. Otherwise, it returns true,
// including the case when the stream was empty.
//
// This is a blocking unordered function that processes items concurrently using n goroutines.
// When n = 1, processing becomes sequential, making the function ordered.
//
// See the package documentation for more information on blocking unordered functions and error handling.
func All[A any](in <-chan Try[A], n int, f func(A) (bool, error)) (bool, error) {
	err := ForEach(in, n, func(a A) error {
		ok, err := f(a)
		if err != nil {
			return err
		}
		if !ok {
			return errFound
		}
		return nil
	})

	if err == errFound { //nolint:errorlint
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}
