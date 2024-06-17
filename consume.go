package rill

import (
	"errors"
	"sync"

	"github.com/destel/rill/internal/core"
)

// ForEach applies a function f to each item in an input stream.
//
// This is a blocking unordered function that processes items concurrently using n goroutines.
// The case when n = 1 is optimized: it does not spawn additional goroutines and processes items sequentially,
// making the function ordered.
//
// See the package documentation for more information on blocking unordered functions and error handling.
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

// Err returns the first error encountered in the input stream or nil there were no errors.
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

// First returns the first item or error encountered in the input stream, whi
// The found return flag is set to false if the stream is empty, otherwise it is set to true.
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
// The case when n = 1 is optimized: it does not spawn additional goroutines and processes items sequentially,
// making the function ordered.
//
// See the package documentation for more information on blocking unordered functions and error handling.
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

// All checks if all items in the input stream that satisfy the condition f.
// This function returns false as soon as it finds an item that does not satisfy the condition. Otherwise, it returns true,
// including the case when the stream is empty.
//
// This is a blocking unordered function that processes items concurrently using n goroutines.
// The case when n = 1 is optimized: it does not spawn additional goroutines and processes items sequentially,
// making the function ordered.
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
