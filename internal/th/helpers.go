package th

import (
	"fmt"
	"math/rand"
	"strings"
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

func Send[T any](ch chan<- T, items ...T) {
	for _, item := range items {
		ch <- item
	}
}

// SimulateWork sleeps for a random time in [min, max]. Call it inside a
// worker function (in a synctest bubble) to enforce concurrent execution
// instead of relying on scheduler luck.
//
// A random sleep imitates real IO-bound work, where each worker spends most of
// its time blocked on a network call, and every call takes a similar but not
// identical time.
//
// Since each worker sleeps at least min per iteration, one worker can't grab most
// of the items. The min and max values also limit how uneven the work can get. In the worst
// case, the fastest and slowest workers stay within (n-1)*max/min items of each
// other. Tests rely on this to check exact numbers, such as how many items still
// run after an early exit.
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

func DoConcurrentlyN(n int, f func(i int)) {
	var wg sync.WaitGroup

	for i := range n {
		wg.Go(func() {
			f(i)
		})
	}

	wg.Wait()
}

// Name generates a test name.
// Works the same way as fmt.Sprint, but adds spaces between all arguments.
func Name(args ...any) string {
	res := fmt.Sprintln(args...)
	return strings.TrimSpace(res)
}

func TestBothOrderings(t *testing.T, f func(t *testing.T, ord bool)) {
	t.Run("unordered", func(t *testing.T) {
		f(t, false)
	})

	t.Run("ordered", func(t *testing.T) {
		f(t, true)
	})
}

// RunSynctest runs a subtest in a synctest bubble.
// It panics if any unless all goroutines started from f exit cleanly.
func RunSynctest(t *testing.T, name string, f func(t *testing.T)) {
	t.Run(name, func(t *testing.T) {
		synctest.Test(t, f)
	})
}
