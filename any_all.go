package rill

import "errors"

// Any checks if there is an item in the input channel that satisfies the condition f.
// This function uses n goroutines for concurrency. It blocks execution until either:
//   - A matching item is found
//   - All items have been checked
//   - An error is encountered in the condition function f or from the upstream
//
// In case of early termination, Any ensures the input channel is drained to avoid goroutine leaks,
// making it safe for use in environments where cleanup is crucial. The function returns the first encountered error, or nil
//
// The function returns true if a match is found, false otherwise, or a first encountered error.
func Any[A any](in <-chan Try[A], n int, f func(A) (bool, error)) (bool, error) {
	errFound := errors.New("found")

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

	if err == nil {
		return false, nil
	}
	if errors.Is(err, errFound) {
		return true, nil
	}
	return false, err
}

// All checks if all items in the input channel satisfy the condition function f.
// This function uses n goroutines for concurrency and blocks execution until:
//   - A non-matching item is found,
//   - All items have been checked,
//   - An error is encountered in the condition function f or from the upstream.
//
// In case of early termination, All ensures the input channel is drained to avoid goroutine leaks,
// making it safe for use in environments where cleanup is crucial. The function returns the first encountered error, or nil
//
// Returns true if all items match the condition, false otherwise, or a first encountered error.
func All[A any](in <-chan Try[A], n int, f func(A) (bool, error)) (bool, error) {
	res, err := Any(in, n, func(a A) (bool, error) {
		ok, err := f(a)
		return !ok, err
	})
	return !res, err
}
