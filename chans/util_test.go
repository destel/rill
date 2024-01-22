package chans

import (
	"testing"
	"time"

	"github.com/destel/rill/internal/th"
)

func TestDrainNB(t *testing.T) {
	// run subtest with 10s timeout
	runSubtest := func(t *testing.T, name string, f func(t *testing.T)) {
		t.Run(name, func(t *testing.T) {
			done := make(chan struct{})
			go func() {
				defer close(done)
				f(t)
			}()

			select {
			case <-done:
			case <-time.After(10 * time.Second):
				t.Errorf("test hanged")
			}
		})
	}

	expectEmptyAndClosed := func(t *testing.T, in <-chan int) {
		t.Helper()

		select {
		case _, ok := <-in:
			if ok {
				t.Errorf("got unexpected item")
			}
		case <-time.After(1 * time.Second):
			t.Errorf("was not closed after 1s")
		}
	}

	runSubtest(t, "close before drain", func(t *testing.T) {
		in := make(chan int, 2)
		in <- 1
		in <- 2
		close(in)

		DrainNB(in)

		expectEmptyAndClosed(t, in)
	})

	runSubtest(t, "close after drain", func(t *testing.T) {
		in := make(chan int, 2)
		in <- 1
		in <- 2

		DrainNB(in)

		in <- 3
		in <- 4
		in <- 5
		close(in)

		expectEmptyAndClosed(t, in)
	})
}

func TestBuffer(t *testing.T) {
	trySend := func(ch chan<- int, x int) bool {
		select {
		case ch <- x:
			return true
		case <-time.After(1 * time.Second):
			return false
		}
	}

	in := make(chan int)
	inBuf := Buffer(in, 2)
	_ = inBuf

	th.ExpectValue(t, trySend(in, 1), true)
	th.ExpectValue(t, trySend(in, 2), true)
	th.ExpectValue(t, trySend(in, 3), false)
	th.ExpectValue(t, trySend(in, 4), false)
}

func TestFromToSlice(t *testing.T) {
	s := []string{"foo", "bar", "baz"}
	s1 := ToSlice(FromSlice(s))
	th.ExpectSlice(t, s1, s)
}
