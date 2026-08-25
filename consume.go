package rill

import (
	"errors"
	"sync/atomic"
)

// ForEach applies a function f to each item in an input stream and returns the first error encountered.
//
// This is a blocking unordered function that processes items concurrently using n goroutines.
//
// When n = 1, ForEach processes items sequentially in stream order, similar to a regular
// for-range loop: f can safely read and modify shared state without synchronization,
// and all its effects are visible to the caller after ForEach returns.
//
// See the package documentation for more information on blocking unordered functions and error handling.
func ForEach[A any](in <-chan Try[A], n int, f func(A) error, options ...SinkOption) error {
	validateN(n)
	validateNilFunc(f == nil)

	// The n = 1 path is an internal contract, not just an optimization.
	// Other sinks (Any, Reduce) build their n = 1 behavior on it and rely on:
	//   - items processed sequentially, in stream order
	//   - f executed in the calling goroutine
	//   - return only after the loop exits, so state captured by f is safe
	//     to use after ForEach returns
	if n == 1 {
		defer Discard(in, options...)

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

	var done atomic.Bool
	defer done.Store(true)

	out := FilterMap(in, n, func(a A) (struct{}, bool, error) {
		if done.Load() {
			return struct{}{}, false, nil
		}
		return struct{}{}, false, f(a)
	})

	return Err(out, options...)
}

// Err returns the first error encountered in the input stream or nil if there were no errors.
//
// This is a blocking ordered function that processes items sequentially.
// See the package documentation for more information on blocking ordered functions and error handling.
func Err[A any](in <-chan Try[A], options ...SinkOption) error {
	defer Discard(in, options...)

	for a := range in {
		if a.Error != nil {
			return a.Error
		}
	}

	return nil
}

// First returns the first value or error encountered in the input stream.
// If the stream is empty or its first item is an error, found is false and
// value is the zero value of A.
//
// This is a blocking ordered function that processes items sequentially.
// See the package documentation for more information on blocking ordered functions and error handling.
func First[A any](in <-chan Try[A], options ...SinkOption) (value A, found bool, err error) {
	defer Discard(in, options...)

	var zero A
	a, ok := <-in
	if !ok || a.Error != nil {
		return zero, false, a.Error
	}
	return a.Value, true, nil
}

// errFound is a control-flow sentinel, compared by identity - the fs.SkipDir
// pattern. Shared by Any and All: both short-circuit when the search finds its
// target (a match, or a counterexample). It never escapes a sink, so the
// sharing cannot contaminate across calls.
var errFound = errors.New("found")

// Any reports whether the input stream contains an item that satisfies the condition f.
// This function returns true as soon as it finds such an item. Otherwise, it returns false.
//
// Any is a blocking unordered function that processes items concurrently using n goroutines.
// When n = 1, items are processed sequentially in stream order.
//
// See the package documentation for more information on blocking unordered functions and error handling.
func Any[A any](in <-chan Try[A], n int, f func(A) (bool, error)) (bool, error) {
	validateN(n)
	validateNilFunc(f == nil)

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

// All reports whether all items in the input stream satisfy the condition f.
// This function returns false as soon as it finds an item that does not satisfy the condition or encounters an error.
// Otherwise, it returns true.
//
// All is a blocking unordered function that processes items concurrently using n goroutines.
// When n = 1, items are processed sequentially in stream order.
//
// See the package documentation for more information on blocking unordered functions and error handling.
func All[A any](in <-chan Try[A], n int, f func(A) (bool, error)) (bool, error) {
	validateN(n)
	validateNilFunc(f == nil)

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
