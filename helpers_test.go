package rill

import (
	"fmt"
)

// Item[A] is a comparable variation of Try[A] that's used for testing.
// Stores errors as messages
type Item[A any] struct {
	Value A
	Error string
}

func (item Item[A]) String() string {
	if item.Error != "" {
		return item.Error
	}
	return fmt.Sprintf("%v", item.Value)
}

func appendVal[A any](s []Item[A], value ...A) []Item[A] {
	for _, v := range value {
		s = append(s, Item[A]{Value: v})
	}
	return s
}

func appendErr[A any](s []Item[A], error ...error) []Item[A] {
	for _, e := range error {
		s = append(s, Item[A]{Error: e.Error()})
	}
	return s
}

func appendTry[A any](s []Item[A], item Try[A]) []Item[A] {
	var errMessage string
	if item.Error != nil {
		errMessage = item.Error.Error()
	}
	return append(s, Item[A]{Value: item.Value, Error: errMessage})
}

func toItemSlice[A any](in <-chan Try[A]) []Item[A] {
	var s []Item[A]
	for x := range in {
		s = appendTry(s, x)
	}
	return s
}

// Converts a stream in a slice of values and a slice of error messages.
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

func replaceWithError[A comparable](in <-chan Try[A], value A, err error) <-chan Try[A] {
	out := make(chan Try[A])

	go func() {
		defer close(out)

		var zero A

		for x := range in {
			if x.Error == nil && x.Value == value {
				x.Value = zero
				x.Error = err
			}
			out <- x
		}
	}()

	return out
}
