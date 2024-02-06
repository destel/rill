package th

import (
	"sort"
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

// Infinite generate infinite sequence of natural numbers. It stops when stop channel is closed.
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
		f1 := f
		wg.Add(1)
		go func() {
			defer wg.Done()
			f1()
		}()
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
