// Package th provides basic test helpers.
package th

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"testing"
	"testing/synctest"
	"time"
)

func ExpectValue[A comparable](t *testing.T, actual A, expected A) {
	t.Helper()
	if expected != actual {
		t.Errorf("expected %v, got %v", expected, actual)
	}
}

func ExpectBetween[A cmp.Ordered](t *testing.T, actual A, min A, max A) {
	t.Helper()
	if actual < min || actual > max {
		t.Errorf("expected %v to be between %v and %v", actual, min, max)
	}
}

func ExpectSlice[A comparable](t *testing.T, actual []A, expected []A) {
	t.Helper()
	if len(expected) != len(actual) {
		t.Errorf("length mismatch %d!=%d: expected %v, actual %v", len(expected), len(actual), expected, actual)
		return
	}

	for i := range expected {
		if expected[i] != actual[i] {
			t.Errorf("expected %v, actual %v, mismatch at pos %d: %v != %v", expected, actual, i, expected[i], actual[i])
			return
		}
	}
}

func ExpectElementsMatch[A comparable](t *testing.T, actual, expected []A) {
	t.Helper()
	if len(actual) != len(expected) {
		t.Errorf("length mismatch %d!=%d: expected %v, actual %v", len(expected), len(actual), expected, actual)
		return
	}

	expectedMap := make(map[A]int, len(expected))
	for _, v := range expected {
		expectedMap[v]++
	}
	for _, v := range actual {
		expectedMap[v]--
	}
	for el, count := range expectedMap {
		if count != 0 {
			t.Errorf("element %v mismatch: expected %v, actual %v", el, expected, actual)
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

// ExpectBlock runs f in a synctest bubble and asserts that the main goroutine
// is durably blocked.
func ExpectBlock(t *testing.T, f func(t *testing.T)) {
	t.Helper()
	outcome := checkBlock(t, f)

	if outcome == 0 {
		t.Errorf("expected the main goroutine to block, but everything completed")
		return
	}
	if outcome == 1 {
		t.Errorf("expected the main goroutine to block, but it returned and only background goroutines remained blocked (did you mean ExpectLeak?)")
		return
	}
}

// ExpectLeak runs f in a synctest bubble and asserts that at least one background goroutine
// is durably blocked, while the main goroutine completes cleanly.
func ExpectLeak(t *testing.T, f func(t *testing.T)) {
	t.Helper()
	outcome := checkBlock(t, f)

	if outcome == 0 {
		t.Errorf("expected at least one background goroutine to leak, but everything completed")
		return
	}
	if outcome == 2 {
		t.Errorf("expected only background goroutines to block, but the main goroutine blocked too (did you mean ExpectBlock?)")
		return
	}
}

// outcomes:
// 0 - nothing blocked
// 1 - some goroutines blocked, but not main
// 2 - main blocked and possibly some goroutines also blocked
func checkBlock(t *testing.T, f func(t *testing.T)) (outcome int) {
	mainBlocked := true

	defer func() {
		r := recover()

		if r == nil {
			outcome = 0
			return
		}

		if !strings.HasPrefix(fmt.Sprint(r), "deadlock:") {
			panic(r) // re-panic if not a deadlock message
		}

		if mainBlocked {
			outcome = 2
		} else {
			outcome = 1
		}
	}()

	synctest.Test(t, func(t *testing.T) {
		f(t)
		mainBlocked = false
	})

	return
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

func ExpectNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Errorf("unexpected error '%v'", err)
	}
}
