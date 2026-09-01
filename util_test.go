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

	t.Run("nil w context", func(t *testing.T) {
		th.ExpectBlock(t, func(t *testing.T) {
			_, scope := WithContext(t.Context())
			defer scope.Cancel()

			Discard[int](nil, scope)
			scope.Wait()
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

	th.RunSynctest(t, "normal w context", func(t *testing.T) {
		ctx, scope := WithContext(t.Context())
		defer scope.Cancel()

		in := th.FromRange(0, 100)
		in = th.DelayEach(in, 1*time.Second)

		Discard(in, scope) // doesn't block

		time.Sleep(60 * time.Second)

		th.ExpectOpenChan(t, in)
		th.ExpectActiveContext(t, ctx)

		scope.Wait()

		th.ExpectDrainedChan(t, in)
		th.ExpectCanceledContext(t, ctx)
	})

	th.RunSynctest(t, "two scopes", func(t *testing.T) {
		in := th.FromRange(0, 100)
		in = th.DelayEach(in, 1*time.Second)

		ctx1, scope1 := WithContext(t.Context())
		defer scope1.Cancel()
		ctx2, scope2 := WithContext(t.Context())
		defer scope2.Cancel()

		Discard(in, scope1, scope2) // doesn't block

		th.ExpectActiveContext(t, ctx1)
		th.ExpectActiveContext(t, ctx2)

		scope1.Wait()

		th.ExpectDrainedChan(t, in)
		th.ExpectCanceledContext(t, ctx1)
		th.ExpectActiveContext(t, ctx2)

		scope2.Wait()
		th.ExpectCanceledContext(t, ctx2)
	})

	th.RunSynctest(t, "closed", func(t *testing.T) {
		in := make(chan int)
		close(in)
		Discard(in)
		th.ExpectDrainedChan(t, in)
	})

	th.RunSynctest(t, "closed w context", func(t *testing.T) {
		ctx, scope := WithContext(t.Context())
		defer scope.Cancel()

		in := make(chan int)
		close(in)

		Discard(in, scope)

		th.ExpectDrainedChan(t, in)
		th.ExpectActiveContext(t, ctx)

		scope.Wait() // should not leak

		th.ExpectCanceledContext(t, ctx)
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
