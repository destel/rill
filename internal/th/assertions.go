// Package th provides basic test helpers.
package th

import (
	"sort"
	"testing"
	"time"
)

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

func isSortedChan[A ordered](ch <-chan A) bool {
	prev, ok := <-ch
	if !ok {
		return true
	}

	sorted := true
	for x := range ch {
		sorted = sorted && prev <= x
		prev = x
	}

	return sorted
}

func ExpectSorted[T ordered](t *testing.T, arr []T) {
	t.Helper()
	isSorted := sort.SliceIsSorted(arr, func(i, j int) bool {
		return arr[i] <= arr[j]
	})
	if !isSorted {
		t.Errorf("expected sorted slice")
	}
}

func ExpectUnsorted[T ordered](t *testing.T, arr []T) {
	t.Helper()
	isSorted := sort.SliceIsSorted(arr, func(i, j int) bool {
		return arr[i] <= arr[j]
	})
	if isSorted {
		t.Errorf("expected unsorted slice")
	}
}

func ExpectClosedChan[A any](t *testing.T, ch <-chan A, waitFor time.Duration) {
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

func ExpectError(t *testing.T, err error, message string) {
	t.Helper()
	if err == nil {
		t.Errorf("expected error '%s', got nil", message)
		return
	}

	if err.Error() != message {
		t.Errorf("expected error '%s', got '%s'", message, err.Error())
	}
}

func ExpectNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Errorf("unexpected error '%v'", err)
	}
}

func ExpectNotHang(t *testing.T, waitFor time.Duration, f func()) {
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
