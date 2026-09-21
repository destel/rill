package rill

import (
	"errors"
	"sync"
	"sync/atomic"
)

// ForEach calls f on the stream's values, like a concurrent for-range loop,
// and returns the first error it observes, or nil if there is none.
//
// The argument n bounds the number of concurrent calls to f.
//
// See the [rill] package documentation for the contract shared by all sinks.
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

	var errSeen atomic.Bool
	out := make(chan Try[struct{}], n)

	var wg sync.WaitGroup

	for range n {
		wg.Go(func() {
			for a := range in {
				if errSeen.Load() {
					return
				}

				err := a.Error
				if err == nil {
					err = f(a.Value)
				}

				if err != nil {
					errSeen.Store(true)
					out <- Try[struct{}]{Error: err}
					return
				}
			}
		})
	}

	go func() {
		wg.Wait()

		// out carries the settlement signal.
		// Drain the input before closing out.
		Drain(in)
		close(out)
	}()

	return Err(out, options...)
}

// Err returns the first error in the stream.
//
// See the [rill] package documentation for the contract shared by all sinks.
func Err[A any](in <-chan Try[A], options ...SinkOption) error {
	defer Discard(in, options...)

	for a := range in {
		if a.Error != nil {
			return a.Error
		}
	}

	return nil
}

// First consumes the first item of the stream and returns:
//   - (value, true, nil) if the item is a value
//   - (zero, false, err) if the item is an error
//   - (zero, false, nil) if the stream is empty
//
// See the [rill] package documentation for the contract shared by all sinks.
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

// Any reports whether the stream contains a value that matches f. It returns:
//   - (true, nil) if a match is observed first
//   - (false, err) if an error is observed first
//   - (false, nil) if neither is observed
//
// The argument n bounds the number of concurrent calls to f.
//
// See the [rill] package documentation for the contract shared by all sinks.
func Any[A any](in <-chan Try[A], n int, f func(A) (bool, error), options ...SinkOption) (bool, error) {
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
	}, options...)

	if err == errFound { //nolint:errorlint
		return true, nil
	}
	return false, err
}

// All reports whether every value in the stream matches f. It returns:
//   - (false, nil) if a mismatch is observed first
//   - (false, err) if an error is observed first
//   - (true, nil) if neither is observed
//
// The argument n bounds the number of concurrent calls to f.
//
// See the [rill] package documentation for the contract shared by all sinks.
func All[A any](in <-chan Try[A], n int, f func(A) (bool, error), options ...SinkOption) (bool, error) {
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
	}, options...)

	if err == errFound { //nolint:errorlint
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}
