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

func ExpectValueLTE[A number](t *testing.T, actual A, expected A) {
	t.Helper()
	if actual > expected {
		t.Errorf("expected %v <= %v", actual, expected)
	}
}

func ExpectValueGTE[A number](t *testing.T, actual A, expected A) {
	t.Helper()
	if actual < expected {
		t.Errorf("expected %v >= %v", actual, expected)
	}
}

func ExpectValueInDelta[A number](t *testing.T, actual A, expected A, delta A) {
	t.Helper()
	diff := actual - expected
	if diff < 0 {
		diff = -diff
	}

	if diff > delta {
		t.Errorf("expected %v in [%v-%v]", actual, expected-delta, expected+delta)
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

type number interface {
	~int | ~int64
}

type ordered interface {
	~int | ~int64 | ~string
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

func ExpectClosedChan[A any](t *testing.T, ch <-chan A) {
	t.Helper()
	select {
	case x, ok := <-ch:
		if ok {
			t.Errorf("expected channel to be closed, but got %v", x)
		}
	default:
		t.Errorf("expected channel to be closed, but it's blocked")
	}
}

func ExpectNeverClosedChan[A any](t *testing.T, ch <-chan A, waitFor time.Duration) {
	t.Helper()
	timeout := time.After(waitFor)
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				t.Errorf("expected channel to be never closed")
				return
			}
		case <-timeout:
			return
		}
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

func ExpectNotPanic(t *testing.T, f func()) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("unexpected panic: %v", r)
		}
	}()
	f()
}
