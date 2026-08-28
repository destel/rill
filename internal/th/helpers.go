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

// ExpectNoRace is a semantic name for a bare unsynchronized read.
// Tests sometimes need to do an unsynchronized access to a variable, to
// have the race detector confirm that all writes in other goroutines
// happen before this read. It's enough to do a no-op read:
//
//	_ = myVariable
//
// This works, but requires an explaining comment at every site.
// ExpectNoRace is also a no-op, but makes the call site clearer:
//
//	th.ExpectNoRace(myVariable)
//
//go:noinline
func ExpectNoRace[T any](value T) {
	// This function does nothing and has a noinline pragma to make sure
	// the compiler does not remove the variable access.
}

// DelayEach forwards items, sleeping for the given duration before each one.
// Under synctest this makes it impossible to consume the channel in zero fake
// time, hence one goroutine (main) can observe the intermediate state of another
// goroutine (drain) consuming the stream. This function is usually
// paired with [ExpectOpenChan].
//
// A sleep of 1ns - written as a bare 1 in the tests - is the minimum that
// gives observability: the sleep completes only when every goroutine in the
// bubble is durably blocked, so the stream cannot advance while any goroutine
// is runnable.
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
