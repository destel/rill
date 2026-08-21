package rill

import (
	"errors"
	"sync/atomic"
)

// ForEach consumes the stream, calling f on each value. It immediately
// returns the first observed error. Otherwise, it returns nil after the
// input is fully consumed and every call to f has returned.
//
// The argument n bounds the number of concurrent calls to f. When n = 1,
// ForEach processes items sequentially in stream order, similar to a
// regular for-range loop: f can safely read and modify shared state without
// synchronization, and all its effects are visible to the caller after
// ForEach returns.
//
// See the package documentation for the behaviors that all sinks share.
func ForEach[A any](in <-chan Try[A], n int, f func(A) error) error {
	validateN(n)
	validateNilFunc(f == nil)

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

	var done atomic.Bool
	defer done.Store(true)

	out := FilterMap(in, n, func(a A) (struct{}, bool, error) {
		if done.Load() {
			return struct{}{}, false, nil
		}
		return struct{}{}, false, f(a)
	})

	return Err(out)
}

// Err consumes the stream and immediately returns the first error it
// contains. Otherwise, it returns nil after the input is fully
// consumed.
//
// See the package documentation for the behaviors that all sinks share.
func Err[A any](in <-chan Try[A]) error {
	defer Discard(in)

	for a := range in {
		if a.Error != nil {
			return a.Error
		}
	}

	return nil
}

// First returns the first item of the stream: (value, true, nil) if
// the item is a value, (zero, false, err) if it is an error, or
// (zero, false, nil) if the stream is empty.
//
// The rest of the stream is discarded. See the package documentation
// for the behaviors that all sinks share.
func First[A any](in <-chan Try[A]) (value A, found bool, err error) {
	defer Discard(in)

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

// Any reports whether the stream contains a value that matches the
// condition f. It consumes the stream, calling f on each value, and
// immediately returns (true, nil) or (false, err) on the first observed
// match or error, respectively. Otherwise, it returns (false, nil) after
// the input is fully consumed and every call to f has returned.
//
// The argument n bounds the number of concurrent calls to f.
//
// See the package documentation for the behaviors that all sinks share.
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

// All reports whether every value in the stream matches the condition f.
// It consumes the stream, calling f on each value, and immediately returns
// (false, nil) or (false, err) on the first observed mismatch or error,
// respectively. Otherwise, it returns (true, nil) after the input is fully
// consumed and every call to f has returned.
//
// The argument n bounds the number of concurrent calls to f.
//
// See the package documentation for the behaviors that all sinks share.
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
