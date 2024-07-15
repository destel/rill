package rill

import (
	"github.com/destel/rill/internal/core"
)

// ForEach applies a function f to each item in an input stream.
//
// This is a blocking unordered function that processes items concurrently using n goroutines.
// When n = 1, processing becomes sequential, making the function ordered and similar to a regular for-range loop.
//
// See the package documentation for more information on blocking unordered functions and error handling.
func ForEach[A any](in <-chan Try[A], n int, f func(A) error) error {
	var retErr error
	var once core.OnceWithWait
	setReturns := func(err error) {
		once.Do(func() {
			retErr = err
		})
	}

	go func() {
		core.ForEach(in, n, func(a Try[A]) {
			if once.WasCalled() {
				return // drain
			}

			err := a.Error
			if err == nil {
				err = f(a.Value)
			}
			if err != nil {
				setReturns(err)
			}
		})

		setReturns(nil)
	}()

	once.Wait()
	return retErr
}

// Err returns the first error encountered in the input stream or nil if there were no errors.
//
// This is a blocking ordered function that processes items sequentially.
// See the package documentation for more information on blocking ordered functions and error handling.
func Err[A any](in <-chan Try[A]) error {
	defer DrainNB(in)

	for a := range in {
		if a.Error != nil {
			return a.Error
		}
	}

	return nil
}

// First returns the first item or error encountered in the input stream, whichever comes first.
// The found return flag is set to false if the stream was empty, otherwise it is set to true.
//
// This is a blocking ordered function that processes items sequentially.
// See the package documentation for more information on blocking ordered functions and error handling.
func First[A any](in <-chan Try[A]) (value A, found bool, err error) {
	defer DrainNB(in)

	for a := range in {
		return a.Value, true, a.Error
	}

	found = false
	return
}

// Any checks if there is an item in the input stream that satisfies the condition f.
// This function returns true as soon as it finds such an item. Otherwise, it returns false.
//
// Any is a blocking unordered function that processes items concurrently using n goroutines.
// When n = 1, processing becomes sequential, making the function ordered.
//
// See the package documentation for more information on blocking unordered functions and error handling.
func Any[A any](in <-chan Try[A], n int, f func(A) (bool, error)) (bool, error) {
	var retFound bool
	var retErr error
	var once core.OnceWithWait
	setReturns := func(found bool, err error) {
		once.Do(func() {
			retFound = found
			retErr = err
		})
	}

	go func() {
		core.ForEach(in, n, func(a Try[A]) {
			if once.WasCalled() {
				return // drain
			}

			if err := a.Error; err != nil {
				setReturns(false, err)
				return
			}

			ok, err := f(a.Value)
			if err != nil {
				setReturns(false, err)
				return
			}
			if ok {
				setReturns(true, nil)
				return
			}
		})

		setReturns(false, nil)
	}()

	once.Wait()
	return retFound, retErr
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
	// Idea: x && y && z is the same as !(!x || !y || !z)
	// So we can use Any with a negated condition to implement All
	res, err := Any(in, n, func(a A) (bool, error) {
		ok, err := f(a)
		return !ok, err // negate
	})
	return !res, err // negate
}
