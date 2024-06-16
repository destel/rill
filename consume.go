package rill

import (
	"errors"
	"sync"

	"github.com/destel/rill/internal/core"
)

// ForEach applies a function f to each item in an input stream and returns the first encountered error, if any.
// It blocks until one of the following conditions is met:
//   - An error is encountered in the stream - ForEach returns the error.
//   - Function f returns an error - ForEach returns the error.
//   - The end of the stream is reached - ForEach returns nil.
//
// ForEach uses n goroutines to call the user-provided function f concurrently, which means that
// items can be processed out of order. The case when n = 1 is optimized: it does not spawn
// additional goroutines and processes items sequentially.
//
// If ForEach terminates early (before reaching the end of the input stream), it initiates
// background draining of the remaining items. This is done to prevent goroutine
// leaks by ensuring that all goroutines feeding the stream are allowed to complete.
// The input stream should not be used anymore after calling this function.
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

// Err returns the first error encountered in the input stream or nil There are no errors.
// It blocks until one of the following conditions is met:
//   - An error is encountered - Err returns the error.
//   - The end of the stream is reached - Err returns nil.
//
// If Err terminates early (before reaching the end of the input stream), it initiates
// background draining of the remaining items. This is done to prevent goroutine
// leaks by ensuring that all goroutines feeding the stream are allowed to complete.
// The input stream should not be used anymore after calling this function.
func Err[A any](in <-chan Try[A]) error {
	defer DrainNB(in)

	for a := range in {
		if a.Error != nil {
			return a.Error
		}
	}

	return nil
}

// First returns the first item encountered in the input stream.
// It blocks until one of the following conditions is met:
//   - An error is encountered - First returns the error.
//   - A value is encountered - First returns the value with the found flag set to true.
//   - The end of the stream is reached - First sets the found flag to false.
//
// If First terminates early (before reaching the end of the input stream), it initiates
// background draining of the remaining items. This is done to prevent goroutine
// leaks by ensuring that all goroutines feeding the stream are allowed to complete.
// The input stream should not be used anymore after calling this function.
func First[A any](in <-chan Try[A]) (value A, found bool, err error) {
	defer DrainNB(in)

	for a := range in {
		return a.Value, true, a.Error
	}

	found = false
	return
}

// Any checks if there is an item in the input stream that satisfies the condition f.
// It blocks until one of the following conditions is met:
//   - An error is encountered in the stream - Any returns the error.
//   - Function f returns an error - Any returns the error.
//   - Function f returns true - Any returns true.
//   - The end of the stream is reached - Any returns false.
//
// Any uses n goroutines to call the user-provided function f concurrently.
// The case when n = 1 is optimized: it does not spawn additional goroutines.
//
// If Any terminates early (before reaching the end of the input stream), it initiates
// background draining of the remaining items. This is done to prevent goroutine
// leaks by ensuring that all goroutines feeding the stream are allowed to complete.
// The input stream should not be used anymore after calling this function.
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

// All checks if all items in the input stream satisfy the condition function f.
// It blocks until one of the following conditions is met:
//   - An error is encountered in the stream - All returns the error.
//   - Function f returns an error - All returns the error.
//   - Function f returns false - All returns false.
//   - The end of the stream is reached - All returns true
//
// All uses n goroutines to call the user-provided function f concurrently.
// The case when n = 1 is optimized: it does not spawn additional goroutines.
//
// If All terminates early (before reaching the end of the input stream), it initiates
// background draining of the remaining items. This is done to prevent goroutine
// leaks by ensuring that all goroutines feeding the stream are allowed to complete.
// The input stream should not be used anymore after calling this function.
func All[A any](in <-chan Try[A], n int, f func(A) (bool, error)) (bool, error) {
	// Idea: x && y && z is the same as !(!x || !y || !z)
	// So we can use Any with a negated condition to implement All
	res, err := Any(in, n, func(a A) (bool, error) {
		ok, err := f(a)
		return !ok, err // negate
	})
	return !res, err // negate
}
