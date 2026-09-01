package rill

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/destel/rill/internal/th"
)

func TestScope(t *testing.T) {
	th.RunSynctest(t, "one branch discarded", func(t *testing.T) {
		ctx, scope := WithContext(t.Context())
		defer scope.Cancel()

		var state int64

		in := FromChan(th.FromRange(0, 100), nil)
		in = th.DelayEach(in, 1)

		in1, in2 := Tee(in)

		out1 := Map(in1, 5, func(x int) (int, error) {
			atomic.AddInt64(&state, 1)
			th.SimulateWork(1*time.Second, 2*time.Second)
			return x * 2, nil
		})

		Discard(out1, scope)

		out2 := OrderedFilter(in2, 5, func(x int) (bool, error) {
			atomic.AddInt64(&state, 1)
			th.SimulateWork(1*time.Second, 2*time.Second)
			return x >= 50, nil
		})

		res, ok, err := First(out2, scope)

		th.ExpectNoError(t, err)
		th.ExpectValue(t, ok, true)
		th.ExpectValue(t, res, 50)
		th.ExpectActiveContext(t, ctx)

		scope.Wait()

		th.ExpectNoRace(state)
		th.ExpectCanceledContext(t, ctx)
	})

	th.RunSynctest(t, "two sinks", func(t *testing.T) {
		ctx, scope := WithContext(t.Context())
		defer scope.Cancel()

		var state int64

		in := FromChan(th.FromRange(0, 100), nil)
		in = th.DelayEach(in, 1)

		in1, in2 := Tee(in)

		out1 := OrderedFilter(in1, 5, func(x int) (bool, error) {
			atomic.AddInt64(&state, 1)
			th.SimulateWork(1*time.Second, 2*time.Second)
			return x >= 50, nil
		})

		// this branch is slower
		out2 := OrderedFilter(in2, 5, func(x int) (bool, error) {
			atomic.AddInt64(&state, 1)
			th.SimulateWork(100*time.Second, 200*time.Second)
			return x >= 15, nil
		})

		var res1, res2 int
		var ok1, ok2 bool
		var err1, err2 error

		var sinksReturned sync.WaitGroup
		sinksReturned.Go(func() { res1, ok1, err1 = First(out1, scope) })
		sinksReturned.Go(func() { res2, ok2, err2 = First(out2, scope) })
		sinksReturned.Wait()

		th.ExpectNoError(t, err1)
		th.ExpectValue(t, ok1, true)
		th.ExpectValue(t, res1, 50)
		th.ExpectNoError(t, err2)
		th.ExpectValue(t, ok2, true)
		th.ExpectValue(t, res2, 15)
		th.ExpectActiveContext(t, ctx)

		scope.Wait()

		th.ExpectNoRace(state)
		th.ExpectCanceledContext(t, ctx)
	})

	t.Run("wait called before sinks return", func(t *testing.T) {
		th.ExpectLeak(t, func(t *testing.T) {
			var panicsCount atomic.Int64
			catchSinkPanic := func() {
				if recover() != nil {
					panicsCount.Add(1)
				}
			}

			_, scope := WithContext(t.Context())
			defer scope.Cancel()

			in := FromChan(th.FromRange(0, 100), nil)
			in = th.DelayEach(in, 1*time.Second)

			in1, in2 := Tee(in)

			go func() {
				defer catchSinkPanic()
				_, _, _ = First(in1, scope)
			}()
			go func() {
				defer catchSinkPanic()
				_, _, _ = First(in2, scope)
			}()

			// Calling wait w/o waiting for sinks to return
			// Each sink taks 1s of fake time, so this is deterministic
			scope.Wait()

			// Eventually both goroutines return
			time.Sleep(24 * time.Hour)

			th.ExpectValue(t, panicsCount.Load(), 2)
		})
	})

	t.Run("cancel", func(t *testing.T) {
		ctx, scope := WithContext(t.Context())
		scope.Cancel()

		th.ExpectCanceledContext(t, ctx)
	})

	th.RunSynctest(t, "multiple waiters", func(t *testing.T) {
		ctx, scope := WithContext(t.Context())
		defer scope.Cancel()

		in := FromChan(th.FromRange(0, 100), nil)
		in = th.DelayEach(in, 1*time.Second)
		Discard(in, scope)

		var returnedCnt atomic.Int64
		for range 10 {
			go func() {
				scope.Wait()
				returnedCnt.Add(1)
			}()
		}

		time.Sleep(24 * time.Hour)

		th.ExpectValue(t, returnedCnt.Load(), 10)
		th.ExpectCanceledContext(t, ctx)
	})
}
