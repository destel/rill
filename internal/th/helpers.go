package th

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"testing/synctest"
	"time"
)

func FromSlice[A any](slice []A) <-chan A {
	out := make(chan A, len(slice))
	for _, a := range slice {
		out <- a
	}
	close(out)
	return out
}

func ToSlice[A any](in <-chan A) []A {
	var res []A
	for x := range in {
		res = append(res, x)
	}
	return res
}

func FromRange(start, end int) <-chan int {
	ch := make(chan int, end-start)
	for i := start; i < end; i++ {
		ch <- i
	}
	close(ch)
	return ch
}

func DontClose[A any](in <-chan A) <-chan A {
	out := make(chan A)
	go func() {
		for x := range in {
			out <- x
		}
		// don't close out
	}()
	return out
}

// DelayEach forwards items, sleeping for the given duration before each one.
// Useful to make a stream slow. Under synctest even a minimal 1ns delay acts
// as a freeze point: the stream cannot advance past this stage while any
// goroutine in the bubble is runnable, because fake time only advances when
// all goroutines are durably blocked.
func DelayEach[A any](in <-chan A, delay time.Duration) <-chan A {
	out := make(chan A)
	go func() {
		defer close(out)
		for x := range in {
			time.Sleep(delay)
			out <- x
		}
	}()
	return out
}

// SimulateWork sleeps for a random time in [min, max]. Call it inside a
// worker function (in a synctest bubble) to enforce concurrent execution
// instead of relying on scheduler luck.
//
// A random sleep imitates real IO-bound work, where each worker spends most of
// its time blocked on a network call, and every call takes a similar but not
// identical time.
//
// Since each worker sleeps at least min per iteration, no single worker can race
// ahead and grab most of the items. Workers move through the input at different,
// variable speeds, but they're guaranteed to stay within (n-1)*maxSleep/minSleep items of
// each other. Tests rely on this bound to check how many extra items were
// processed after an early return.
func SimulateWork(min, max time.Duration) {
	d := min + time.Duration(rand.Int63n(int64(max-min+1)))
	time.Sleep(d)
}

// WaitForInflightWork is just a sleep with a semantic name.
// It fast-forwards fake time far enough that any goroutines currently
// simulating work have a chance to complete.
// Mostly used with early-exit tests.
func WaitForInflightWork() {
	time.Sleep(1 * time.Hour)
}

func DoConcurrently(ff ...func()) {
	var wg sync.WaitGroup

	for _, f := range ff {
		wg.Go(func() {
			f()
		})
	}

	wg.Wait()
}

func TestBothOrderings(t *testing.T, f func(t *testing.T, ord bool)) {
	t.Run("unordered", func(t *testing.T) {
		f(t, false)
	})

	t.Run("ordered", func(t *testing.T) {
		f(t, true)
	})
}

func TestVariants[V any](t *testing.T, name string, variants []V, f func(t *testing.T, v V)) {
	for _, v := range variants {
		t.Run(fmt.Sprintf("%s=%v", name, v), func(t *testing.T) {
			f(t, v)
		})
	}
}

func TestLevels(t *testing.T, levels []int, f func(t *testing.T, n int)) {
	TestVariants(t, "n", levels, f)
}

// RunSynctest runs a subtest in a synctest bubble.
// It panics if any unless all goroutines started from f exit cleanly.
func RunSynctest(t *testing.T, name string, f func(t *testing.T)) {
	t.Run(name, func(t *testing.T) {
		synctest.Test(t, f)
	})
}
