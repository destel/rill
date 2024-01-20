package chans

import (
	"sync"
	"testing"
)

func fromRange(start, end int) <-chan int {
	ch := make(chan int)
	go func() {
		defer close(ch)
		for i := start; i < end; i++ {
			ch <- i
		}
	}()
	return ch
}

func expectValue[A comparable](t *testing.T, expected A, actual A) {
	t.Helper()
	if expected != actual {
		t.Errorf("expected %v, got %v", expected, actual)
	}
}

func expectSlice[A comparable](t *testing.T, expected []A, actual []A) {
	t.Helper()
	if len(expected) != len(actual) {
		t.Errorf("expected %v, got %v", expected, actual)
		return
	}

	for i := range expected {
		if expected[i] != actual[i] {
			t.Errorf("expected %v, got %v", expected, actual)
			return
		}
	}
}

type inProgressCounter struct {
	mu      sync.Mutex
	current int
	max     int
}

func (c *inProgressCounter) Inc() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.current++
	if c.max < c.current {
		c.max = c.current
	}
}

func (c *inProgressCounter) Dec() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.current--
}

func (c *inProgressCounter) Current() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.current
}

func (c *inProgressCounter) Max() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.max
}
