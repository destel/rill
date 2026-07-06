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

func RandomSleep(min, max time.Duration) {
	d := min + time.Duration(rand.Int63n(int64(max-min)))
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

// RunSynctestExpectBlock runs a subtest in a synctest bubble and expects all goroutines to durably block.
// The test will fail if all goroutines exit cleanly.
func RunSynctestExpectBlock(t *testing.T, name string, f func(t *testing.T)) {
	t.Run(name, func(t *testing.T) {
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("expected deadlock")
				return
			}
			if strings.Contains(fmt.Sprint(r), "deadlock") {
				return
			}
			panic(r) // re-panic if not a deadlock
		}()

		synctest.Test(t, f)
	})
}
