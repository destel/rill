package rill

import (
	"context"
	"sync/atomic"
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
	t.Run("nil", func(t *testing.T) {
		th.ExpectLeak(t, func(t *testing.T) {
			Discard[int](nil)
		})
	})

	t.Run("nil w context", func(t *testing.T) {
		th.ExpectBlock(t, func(t *testing.T) {
			_, opt := WithContext(t.Context())
			Discard[int](nil, opt)
		})
	})

	th.RunSynctest(t, "normal", func(t *testing.T) {
		in := th.FromRange(0, 100)
		in = th.DelayEach(in, 1*time.Second)

		stopwatch := th.StartStopwatch()
		Discard(in)
		stopwatch.Stop()

		th.ExpectOpenChan(t, in)
		th.ExpectValue(t, stopwatch.Elapsed(), 0)

		time.Sleep(50 * time.Second)
		th.ExpectOpenChan(t, in)

		time.Sleep(50 * time.Second)
		th.ExpectDrainedChan(t, in)
	})

	th.RunSynctest(t, "normal w context", func(t *testing.T) {
		ctx, opt := WithContext(t.Context())

		var stopwatch th.Stopwatch
		context.AfterFunc(ctx, stopwatch.Stop)

		in := th.FromRange(0, 100)
		in = th.DelayEach(in, 1*time.Second)

		stopwatch.Start()
		Discard(in, opt)

		th.ExpectDrainedChan(t, in)
		th.ExpectCanceledContext(t, ctx)
		th.ExpectValue(t, stopwatch.Elapsed(), 0)
	})

	th.RunSynctest(t, "two contexts", func(t *testing.T) {
		ctx1, opt1 := WithContext(t.Context())
		ctx2, opt2 := WithContext(t.Context())

		in := th.FromRange(0, 100)
		in = th.DelayEach(in, 1*time.Second)

		Discard(in, opt1, opt2)

		th.ExpectDrainedChan(t, in)
		th.ExpectCanceledContext(t, ctx1)
		th.ExpectCanceledContext(t, ctx2)
	})

	th.RunSynctest(t, "closed w context", func(t *testing.T) {
		ctx, opt := WithContext(t.Context())

		in := make(chan int)
		close(in)

		Discard(in, opt)

		th.ExpectCanceledContext(t, ctx)
	})

	th.RunSynctest(t, "hooks", func(t *testing.T) {
		var appliedCnt, outcomeKnownCnt, settledCnt atomic.Int32

		opt := sinkOptionFunc(func(options *sinkOptions) {
			appliedCnt.Add(1)
			options.onOutcomeKnown = append(options.onOutcomeKnown, func() {
				outcomeKnownCnt.Add(1)
			})
			options.onSettled = append(options.onSettled, func() {
				settledCnt.Add(1)
			})
		})

		// Closed input, instant settlement
		in1 := make(chan int)
		close(in1)

		Discard(in1, opt)
		th.ExpectValue(t, appliedCnt.Load(), 1)
		th.ExpectValue(t, outcomeKnownCnt.Load(), 1)
		th.ExpectValue(t, settledCnt.Load(), 1)

		time.Sleep(10 * time.Second)
		synctest.Wait()
		th.ExpectValue(t, settledCnt.Load(), 1)

		// The same option twice on another sink: each occurrence is applied
		// Settlement after 10s
		in2 := th.FromRange(0, 10)
		in2 = th.DelayEach(in2, 1*time.Second)

		Discard(in2, opt, opt)
		th.ExpectValue(t, appliedCnt.Load(), 3)
		th.ExpectValue(t, outcomeKnownCnt.Load(), 3)
		th.ExpectValue(t, settledCnt.Load(), 1)

		time.Sleep(10 * time.Second)
		synctest.Wait()
		th.ExpectValue(t, settledCnt.Load(), 3)
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
