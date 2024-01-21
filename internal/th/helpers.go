// Package th provides basic test helpers.
package th

import (
	"testing"
)

func FromRange(start, end int) <-chan int {
	ch := make(chan int)
	go func() {
		defer close(ch)
		for i := start; i < end; i++ {
			ch <- i
		}
	}()
	return ch
}

func Send[T any](ch chan<- T, items ...T) {
	for _, item := range items {
		ch <- item
	}
}

func ExpectValue[A comparable](t *testing.T, expected A, actual A) {
	t.Helper()
	if expected != actual {
		t.Errorf("expected %v, got %v", expected, actual)
	}
}

func ExpectSlice[A comparable](t *testing.T, expected []A, actual []A) {
	t.Helper()
	if len(expected) != len(actual) {
		t.Errorf("expected %v, got %v", expected, actual)
		return
	}

	for i := range expected {
		if expected[i] != actual[i] {
			t.Errorf("expected %v, got %v", expected, actual)
			return
		}
	}
}

func ExpectError(t *testing.T, expected error, actual error) {
	t.Helper()
	if actual == nil {
		t.Errorf("expected error '%v'", expected)
		return
	}

	if expected.Error() != actual.Error() {
		t.Errorf("expected error '%v', got '%v'", expected, actual)
	}
}

func ExpectNoError(t *testing.T, actual error) {
	t.Helper()
	if actual != nil {
		t.Errorf("unexpected error '%v'", actual)
	}
}
