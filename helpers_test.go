package rill

import (
	"fmt"
	"testing"
	"testing/synctest"

	"github.com/destel/rill/internal/th"
)

type ItemSlice[A any] = []Try[A]

func appendVal[A any](s ItemSlice[A], value ...A) ItemSlice[A] {
	for _, v := range value {
		s = append(s, Try[A]{Value: v})
	}
	return s
}

func appendErr[A any](s ItemSlice[A], error ...error) ItemSlice[A] {
	for _, e := range error {
		s = append(s, Try[A]{Error: e})
	}
	return s
}

func appendStream[A any](s ItemSlice[A], stream Stream[A]) ItemSlice[A] {
	for x := range stream {
		s = append(s, x)
	}
	return s
}

func toItemSlice[A any](stream Stream[A]) ItemSlice[A] {
	return appendStream(nil, stream)
}

func ExpectItemsMatch[A comparable](t *testing.T, actual, expected ItemSlice[A]) {
	t.Helper()
	type ComparableItem struct {
		Value A
		Error string
	}

	toComparable := func(s ItemSlice[A]) []ComparableItem {
		res := make([]ComparableItem, len(s))
		for i, item := range s {

			if item.Error != nil {
				res[i].Error = item.Error.Error()
			} else {
				res[i].Value = item.Value
			}
		}
		return res
	}

	th.ExpectElementsMatch(t, toComparable(actual), toComparable(expected))
}

func toSliceAndErrors[A any](in <-chan Try[A]) ([]A, []string) {
	var values []A
	var errors []string

	for x := range in {
		if x.Error != nil {
			errors = append(errors, x.Error.Error())
			continue
		}

		values = append(values, x.Value)
	}

	return values, errors
}

// Converts errors and values to strings and collects them into a single slice.
// Uses the provided [fmt.Printf] format to stringify values.
func toUnifiedStringSlice[A any](in <-chan Try[A], format string) []string {
	var res []string
	for x := range in {
		if x.Error != nil {
			res = append(res, x.Error.Error())
			continue
		}
		res = append(res, fmt.Sprintf(format, x.Value))
	}
	return res
}

// Similar to toSliceAndErrors, but reads until all goroutines are durably blocked
// The third return is true only if the channel was fully consumed.
func toSliceAndErrorsNB[A any](in <-chan Try[A]) ([]A, []string, bool) {
	var values []A
	var errors []string

	for {
		// Wait for all goroutines to be durably blocked.
		// This makes the non-blocking read below deterministic.
		synctest.Wait()

		select {
		case x, ok := <-in:
			if !ok {
				return values, errors, true
			}

			if x.Error != nil {
				errors = append(errors, x.Error.Error())
				continue
			}

			values = append(values, x.Value)
		default:
			return values, errors, false
		}
	}
}

func replaceWithError[A comparable](in <-chan Try[A], value A, err error) <-chan Try[A] {
	out := make(chan Try[A])

	go func() {
		defer close(out)

		for x := range in {
			if x.Error == nil && x.Value == value {
				x.Error = err
			}
			out <- x
		}
	}()

	return out
}
