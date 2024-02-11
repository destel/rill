package th

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"
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

// InfiniteChan generates infinite sequence of natural numbers. It stops when stop channel is closed.
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
