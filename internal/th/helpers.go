package th

import (
	"sort"
	"sync"
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

func ErrChanToMessages(in <-chan error) <-chan string {
	out := make(chan string)

	go func() {
		defer close(out)
		for x := range in {
			out <- x.Error()
		}
	}()

	return out
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
