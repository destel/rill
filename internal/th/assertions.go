// Package th provides basic test helpers.
package th

import (
	"cmp"
	"slices"
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
			t.Errorf("expected %v, got %v, mismatch at pos %d: %v != %v", expected, actual, i, expected[i], actual[i])
			return
		}
	}
}

func ExpectMap[K, V comparable](t *testing.T, actual map[K]V, expected map[K]V) {
	t.Helper()
	if len(expected) != len(actual) {
		t.Errorf("expected %v, got %v", expected, actual)
		return
	}

	for k, v := range expected {
		actualV, ok := actual[k]
		if !ok {
			t.Errorf("expected %v, got %v", expected, actual)
			return
		}

		if v != actualV {
			t.Errorf("expected %v, got %v", expected, actual)
			return
		}
	}
}

type number interface {
	~int | ~int64
}

func ExpectSorted[T cmp.Ordered](t *testing.T, arr []T) {
	t.Helper()
	if !slices.IsSorted(arr) {
		t.Errorf("expected sorted slice")
	}
}

func ExpectUnsorted[T cmp.Ordered](t *testing.T, arr []T) {
	t.Helper()
	if slices.IsSorted(arr) {
		t.Errorf("expected unsorted slice")
	}
}

func ExpectDrainedChan[A any](t *testing.T, ch <-chan A) {
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

func ExpectError(t *testing.T, err error, expectedMessage string) {
	t.Helper()
	if err == nil {
		t.Errorf("expected error '%s', got nil", expectedMessage)
		return
	}

	if err.Error() != expectedMessage {
		t.Errorf("expected error '%s', got '%s'", expectedMessage, err.Error())
	}
}

func ExpectErrorSlice(t *testing.T, errs []error, expectedMessages []string) {
	t.Helper()
	if len(errs) != len(expectedMessages) {
		t.Errorf("expected %d errors, got %d", len(expectedMessages), len(errs))
		return
	}

	for i := range errs {
		if errs[i].Error() != expectedMessages[i] {
			t.Errorf("expected %v, got %v, mismatch at pos %d: %v != %v", expectedMessages, errs, i, expectedMessages[i], errs[i].Error())
			return
		}
	}
}

func ExpectNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Errorf("unexpected error '%v'", err)
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
