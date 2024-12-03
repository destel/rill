package th

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"
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

func SendTimeout[T any](ch chan<- T, timeout time.Duration, items ...T) bool {
	t := time.NewTimer(timeout)
	defer t.Stop()

	for _, item := range items {
		select {
		case <-t.C:
			return false
		case ch <- item:
		}
	}

	return true
}

func Sort[A ordered](s []A) {
	sort.Slice(s, func(i, j int) bool {
		return s[i] < s[j]
	})
}

func DoConcurrently(ff ...func()) {
	var wg sync.WaitGroup

	for _, f := range ff {
		f := f
		wg.Add(1)
		go func() {
			defer wg.Done()
			f()
		}()
	}

	wg.Wait()
}

func DoConcurrentlyN(n int, f func(i int)) {
	var wg sync.WaitGroup

	for i := 0; i < n; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			f(i)
		}()
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
