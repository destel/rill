// Package th provides basic test helpers.
package th

import (
	"sort"
	"testing"
	"time"
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

// Infinite generate infinite sequence of natural numbers. It stops when stop channel is closed.
func InfiniteChan(stop <-chan struct{}) <-chan int {
	ch := make(chan int)
	go func() {
		defer close(ch)
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			case ch <- i:
			}
		}
	}()
	return ch
}

func Send[T any](ch chan<- T, items ...T) {
	for _, item := range items {
		ch <- item
	}
}

func ExpectValue[A comparable](t *testing.T, actual A, expected A) {
	t.Helper()
	if expected != actual {
		t.Errorf("expected %v, got %v", expected, actual)
	}
}

func ExpectSlice[A comparable](t *testing.T, actual []A, expected []A) {
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

type ordered interface {
	~int | ~string
}

func IsSorted[A ordered](s []A) bool {
	return sort.SliceIsSorted(s, func(i, j int) bool {
		return s[i] < s[j]
	})
}

func Sort[A ordered](s []A) {
	sort.Slice(s, func(i, j int) bool {
		return s[i] < s[j]
	})
}

func ExpectSorted[A ordered](t *testing.T, s []A) {
	t.Helper()
	if !IsSorted(s) {
		t.Errorf("expected sorted slice")
	}
}

func ExpectUnsorted[A ordered](t *testing.T, s []A) {
	t.Helper()
	if IsSorted(s) {
		t.Errorf("expected unsorted slice")
	}
}

func ExpectClosed[A any](t *testing.T, ch <-chan A, waitFor time.Duration) {
	t.Helper()
	select {
	case x, ok := <-ch:
		if ok {
			t.Errorf("expected channel to be closed, but got %v", x)
		}
	case <-time.After(waitFor):
		t.Errorf("channel was not closed after %v", waitFor)
	}
}

func ExpectError(t *testing.T, actual error, expected error) {
	t.Helper()
	if actual == nil {
		t.Errorf("expected error '%v', got nil", expected)
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

func NotHang(t *testing.T, waitFor time.Duration, f func()) {
	t.Helper()
	done := make(chan struct{})

	go func() {
		defer close(done)
		f()
	}()

	select {
	case <-done:
	case <-time.After(waitFor):
		t.Errorf("test hanged")
	}
}
