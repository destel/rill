package rill

import (
	"errors"
	"sync"

	"github.com/destel/rill/internal/core"
)

// ForEach applies a function f to each item in an input channel using n goroutines for parallel processing. The function
// blocks until all items are processed or an error is encountered, either from the function f itself or from upstream.
// In case of an error leading to early termination, ForEach ensures the input channel is drained to avoid goroutine leaks,
// making it safe for use in environments where cleanup is crucial. The function returns the first encountered error, or nil
// if all items were processed successfully.
// While this function does not guarantee the order of item processing due to its concurrent nature,
// using n = 1 results in sequential processing, as in a simple for-range loop.
func ForEach[A any](in <-chan Try[A], n int, f func(A) error) error {
	if n == 1 {
		for a := range in {
			err := a.Error
			if err == nil {
				err = f(a.Value)
			}

			if err != nil {
				DrainNB(in)
				return err
			}
		}

		return nil
	}

	var retErr error
	var once sync.Once

	in, earlyExit := core.Breakable(in)
	done := make(chan struct{})

	core.Loop(in, done, n, func(a Try[A]) {
		err := a.Error
		if err == nil {
			err = f(a.Value)
		}

		if err != nil {
			earlyExit()
			once.Do(func() {
				retErr = err
			})
		}
	})

	<-done
	return retErr
}

// onceFunc1 returns a single argument function that invokes f only once. The returned function may be called concurrently.
func onceFunc1[T any](f func(T)) func(T) {
	var once sync.Once
	return func(value T) {
		once.Do(func() {
			f(value)
			f = nil
		})
	}
}

// Err returns the first error encountered in the input channel.
// This function blocks until:
//   - An error is found.
//   - The end of the input channel is reached.
//
// If Err terminates early (before the input channel is fully consumed),
// it initiates background draining of the remaining items in the channel. This is done
// to prevent goroutine leaks by ensuring that all goroutines feeding the channel are allowed to complete.
// The input channel should not be used anymore after calling this function.
func Err[A any](in <-chan Try[A]) error {
	defer DrainNB(in)

	for a := range in {
		if a.Error != nil {
			return a.Error
		}
	}

	return nil
}

// First returns the first value or error encountered in the input channel.
// This function blocks until:
//   - A value is found. In this case, the found flag is set to true.
//   - The end of the input channel is reached. In this case, the found flag is set to false.
//   - An error is encountered in the input channel.
//
// If First terminates early (before the input channel is fully consumed),
// it initiates background draining of the remaining items in the channel. This is done
// to prevent goroutine leaks by ensuring that all goroutines feeding the channel are allowed to complete.
// The input channel should not be used anymore after calling this function.
func First[A any](in <-chan Try[A]) (value A, found bool, err error) {
	defer DrainNB(in)

	for a := range in {
		return a.Value, true, a.Error
	}

	found = false
	return
}

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
	errBreak := errors.New("break")
	res := false
	setRes := onceFunc1(func(a bool) {
		res = a
	})

	err := ForEach(in, n, func(a A) error {
		ok, err := f(a)
		if err != nil {
			return err
		}

		if ok {
			setRes(true)
			return errBreak

		}
		return nil
	})

	if err != nil && errors.Is(err, errBreak) {
		err = nil
	}
	return res, err
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
	// Idea: x && y && z is the same as !(!x || !y || !z)
	// So we can use Any with a negated condition to implement All
	res, err := Any(in, n, func(a A) (bool, error) {
		ok, err := f(a)
		return !ok, err // negate
	})
	return !res, err // negate
}
