package rill

import (
	"testing"
	"testing/synctest"

	"github.com/destel/rill/internal/th"
)

// Full behavior of these functions is tested in the internal/core package.
// Tests below only pin the wrapper wiring: right core function, args forwarded.

func TestDrain(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		in := th.FromRange(0, 10)
		Drain(in)
		th.ExpectDrainedChan(t, in)
	})
}

func TestDiscard(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		in := make(chan int)
		Discard(in)

		in <- 1
		in <- 2
		close(in)

		synctest.Wait()
		th.ExpectDrainedChan(t, in)
	})
}

func TestBuffer(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		in := make(chan int)
		out := Buffer(in, 2)

		in <- 1
		in <- 2
		close(in)

		Drain(out)
	})
}
