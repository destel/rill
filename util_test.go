package rill

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/destel/rill/internal/th"
)

// Drain is a wrapper around the function from the core package. The full behavior test is there.
func TestDrain(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		in := th.FromRange(0, 10)
		Drain(in)
		th.ExpectDrainedChan(t, in)
	})
}

func TestDiscard(t *testing.T) {
	th.RunSynctest(t, "nil", func(t *testing.T) {
		Discard[int](nil)
	})

	t.Run("nil w settlement", func(t *testing.T) {
		th.ExpectBlock(t, func(t *testing.T) {
			settled, opt := Settlement()
			Discard[int](nil, opt)
			<-settled
		})
	})

	th.RunSynctest(t, "normal", func(t *testing.T) {
		in := th.FromRange(0, 100)
		in = th.DelayEach(in, 1*time.Second)
		Discard(in) // doesn't block

		time.Sleep(60 * time.Second)
		th.ExpectOpenChan(t, in)

		time.Sleep(60 * time.Second)
		th.ExpectDrainedChan(t, in)
	})

	th.RunSynctest(t, "normal w settlement", func(t *testing.T) {
		in := th.FromRange(0, 100)
		in = th.DelayEach(in, 1*time.Second)

		settled, opt := Settlement()
		Discard(in, opt) // doesn't block

		time.Sleep(60 * time.Second)
		th.ExpectOpenChan(t, in)

		<-settled
		th.ExpectDrainedChan(t, in)
	})

	th.RunSynctest(t, "two settlements", func(t *testing.T) {
		in := th.FromRange(0, 100)
		in = th.DelayEach(in, 1*time.Second)

		settled1, opt1 := Settlement()
		settled2, opt2 := Settlement()
		Discard(in, opt1, opt2) // doesn't block

		<-settled1
		th.ExpectDrainedChan(t, in)
		<-settled2
	})

	th.RunSynctest(t, "closed", func(t *testing.T) {
		in := make(chan int)
		close(in)
		Discard(in)
		th.ExpectDrainedChan(t, in)
	})

	th.RunSynctest(t, "closed w settlement", func(t *testing.T) {
		in := make(chan int)
		close(in)

		settled, opt := Settlement()
		Discard(in, opt)

		th.ExpectDrainedChan(t, in)

		<-settled // should not leak
	})
}

// Buffer is a wrapper around the function from the core package. The full behavior test is there.
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

func TestValidations(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		validateN(1)
		validateMinSize(5, 5)
		validateNilFunc(false)
	})

	t.Run("n too small", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("expected panic")
			}
		}()

		validateN(0)
	})

	t.Run("size too small", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("expected panic")
			}
		}()
		validateMinSize(5, 6)
	})

	t.Run("function is nil", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("expected panic")
			}
		}()
		validateNilFunc(true)
	})
}
