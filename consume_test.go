package rill

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/destel/rill/internal/th"
)

func TestErr(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		th.ExpectBlock(t, func(t *testing.T) {
			_ = Err[int](nil)
		})
	})

	th.RunSynctest(t, "empty", func(t *testing.T) {
		in := FromSlice([]int{}, nil)
		err := Err(in)

		th.ExpectNoError(t, err)
		th.ExpectDrainedChan(t, in)
	})

	th.RunSynctest(t, "no errors", func(t *testing.T) {
		in := FromChan(th.FromRange(1, 21), nil)
		err := Err(in)

		th.ExpectNoError(t, err)
		th.ExpectDrainedChan(t, in)
	})

	th.RunSynctest(t, "error", func(t *testing.T) {
		in := FromChan(th.FromRange(1, 21), nil)
		in = replaceWithError(in, 10, fmt.Errorf("err010"))
		in = replaceWithError(in, 15, fmt.Errorf("err015"))
		in = th.DelayEach(in, 1*time.Second)

		stopwatch := th.StartStopwatch()
		err := Err(in)
		stopwatch.Stop()

		th.ExpectError(t, err, "err010")
		th.ExpectOpenChan(t, in)
		th.ExpectValue(t, stopwatch.Elapsed(), 10*time.Second)

		time.Sleep(24 * time.Hour) // eventually drained

		th.ExpectDrainedChan(t, in)
	})

	t.Run("unclosed", func(t *testing.T) {
		th.ExpectLeak(t, func(t *testing.T) {
			in := FromChan(th.FromRange(1, 21), nil)
			in = replaceWithError(in, 10, fmt.Errorf("err010"))
			in = th.DontClose(in)

			err := Err(in)

			th.ExpectError(t, err, "err010")
		})
	})

	th.RunSynctest(t, "context", func(t *testing.T) {
		ctx, scope := WithContext(t.Context())

		in := FromChan(th.FromRange(1, 21), nil)

		err := Err(in, scope)

		th.ExpectNoError(t, err)
		th.ExpectDrainedChan(t, in)
		th.ExpectCanceledContext(t, ctx)
	})

	th.RunSynctest(t, "context (early cancellation)", func(t *testing.T) {
		ctx, scope := WithContext(t.Context())

		var stopwatch th.Stopwatch
		context.AfterFunc(ctx, stopwatch.Stop)

		in := FromChan(th.FromRange(1, 21), nil)
		in = replaceWithError(in, 10, fmt.Errorf("err010"))
		in = th.DelayEach(in, 1*time.Second)

		stopwatch.Start()
		err := Err(in, scope)

		th.ExpectError(t, err, "err010")
		th.ExpectDrainedChan(t, in)
		th.ExpectCanceledContext(t, ctx)
		th.ExpectValue(t, stopwatch.Elapsed(), 10*time.Second)
	})
}

func TestFirst(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		th.ExpectBlock(t, func(t *testing.T) {
			_, _, _ = First[int](nil)
		})
	})

	th.RunSynctest(t, "empty", func(t *testing.T) {
		in := FromSlice([]int{}, nil)

		x, ok, err := First(in)

		th.ExpectNoError(t, err)
		th.ExpectValue(t, ok, false)
		th.ExpectValue(t, x, 0)

		th.ExpectDrainedChan(t, in)
	})

	th.RunSynctest(t, "value is first", func(t *testing.T) {
		in := FromChan(th.FromRange(1, 21), nil)
		in = replaceWithError(in, 10, fmt.Errorf("err010"))
		in = th.DelayEach(in, 1*time.Second)

		stopwatch := th.StartStopwatch()
		x, ok, err := First(in)
		stopwatch.Stop()

		th.ExpectNoError(t, err)
		th.ExpectValue(t, ok, true)
		th.ExpectValue(t, x, 1)
		th.ExpectOpenChan(t, in)
		th.ExpectValue(t, stopwatch.Elapsed(), 1*time.Second)

		time.Sleep(24 * time.Hour) // eventually drained

		th.ExpectDrainedChan(t, in)
	})

	th.RunSynctest(t, "error is first", func(t *testing.T) {
		in := FromChan(th.FromRange(1, 21), nil)
		in = replaceWithError(in, 1, fmt.Errorf("err001"))
		in = th.DelayEach(in, 1*time.Second)

		stopwatch := th.StartStopwatch()
		x, ok, err := First(in)
		stopwatch.Stop()

		th.ExpectError(t, err, "err001")
		th.ExpectValue(t, ok, false)
		th.ExpectValue(t, x, 0)
		th.ExpectOpenChan(t, in)
		th.ExpectValue(t, stopwatch.Elapsed(), 1*time.Second)

		time.Sleep(24 * time.Hour) // eventually drained

		th.ExpectDrainedChan(t, in)
	})

	th.RunSynctest(t, "value alongside error", func(t *testing.T) {
		in := make(chan Try[int], 1)
		in <- Try[int]{Value: 10, Error: fmt.Errorf("err")}
		close(in)

		x, ok, err := First(in)

		th.ExpectError(t, err, "err")
		th.ExpectValue(t, ok, false)
		th.ExpectValue(t, x, 0) // zeroed
	})

	t.Run("unclosed", func(t *testing.T) {
		th.ExpectLeak(t, func(t *testing.T) {
			in := FromChan(th.FromRange(1, 21), nil)
			in = th.DontClose(in)
			x, ok, err := First(in)

			th.ExpectNoError(t, err)
			th.ExpectValue(t, ok, true)
			th.ExpectValue(t, x, 1)
		})
	})

	th.RunSynctest(t, "context", func(t *testing.T) {
		ctx, scope := WithContext(t.Context())

		in := FromSlice([]int{}, nil)

		x, ok, err := First(in, scope)

		th.ExpectNoError(t, err)
		th.ExpectValue(t, ok, false)
		th.ExpectValue(t, x, 0)
		th.ExpectDrainedChan(t, in)
		th.ExpectCanceledContext(t, ctx)
	})

	th.RunSynctest(t, "context (early cancellation)", func(t *testing.T) {
		ctx, scope := WithContext(t.Context())

		var stopwatch th.Stopwatch
		context.AfterFunc(ctx, stopwatch.Stop)

		in := FromChan(th.FromRange(1, 21), nil)
		in = th.DelayEach(in, 1*time.Second)

		stopwatch.Start()
		x, ok, err := First(in, scope)

		th.ExpectNoError(t, err)
		th.ExpectValue(t, ok, true)
		th.ExpectValue(t, x, 1)
		th.ExpectDrainedChan(t, in)
		th.ExpectCanceledContext(t, ctx)
		th.ExpectValue(t, stopwatch.Elapsed(), 1*time.Second)
	})
}

func TestForEach(t *testing.T) {
	th.TestLevels(t, []int{1, 5}, func(t *testing.T, n int) {

		t.Run("nil", func(t *testing.T) {
			th.ExpectBlock(t, func(t *testing.T) {
				_ = ForEach(nil, n, func(int) error { return nil })
			})
		})

		th.RunSynctest(t, "no errors", func(t *testing.T) {
			in := FromChan(th.FromRange(0, 20), nil)

			var sum atomic.Int64
			err := ForEach(in, n, func(x int) error {
				th.SimulateWork(1*time.Second, 2*time.Second)
				sum.Add(int64(x))
				return nil
			})

			th.ExpectNoError(t, err)
			th.ExpectValue(t, sum.Load(), 19*20/2)
			th.ExpectDrainedChan(t, in)
		})

		th.RunSynctest(t, "error in input", func(t *testing.T) {
			in := FromChan(th.FromRange(0, 1000), nil)
			in = replaceWithError(in, 200, fmt.Errorf("err200"))
			in = th.DelayEach(in, 1)

			var extraCalls atomic.Int64
			err := ForEach(in, n, func(x int) error {
				th.SimulateWork(1*time.Second, 2*time.Second)
				extraCalls.Add(1)
				return nil
			})
			extraCalls.Store(0)

			th.ExpectError(t, err, "err200")
			th.ExpectOpenChan(t, in)

			time.Sleep(24 * time.Hour) // eventually drained

			th.ExpectDrainedChan(t, in)
			if n == 1 {
				th.ExpectValue(t, extraCalls.Load(), 0)
			} else {
				th.ExpectBetween(t, extraCalls.Load(), 1, 50)
			}
		})

		th.RunSynctest(t, "error in func", func(t *testing.T) {
			in := FromChan(th.FromRange(0, 1000), nil)
			in = th.DelayEach(in, 1)

			var extraCalls atomic.Int64
			var stopwatch th.Stopwatch
			err := ForEach(in, n, func(x int) error {
				th.SimulateWork(1*time.Second, 2*time.Second)
				extraCalls.Add(1)
				if x == 200 {
					stopwatch.Start()
					return fmt.Errorf("err200")
				}
				return nil
			})
			extraCalls.Store(0)
			stopwatch.Stop()

			th.ExpectError(t, err, "err200")
			th.ExpectOpenChan(t, in)
			th.ExpectValue(t, stopwatch.Elapsed(), 0)

			time.Sleep(24 * time.Hour) // eventually drained

			th.ExpectDrainedChan(t, in)
			if n == 1 {
				th.ExpectValue(t, extraCalls.Load(), 0)
			} else {
				th.ExpectBetween(t, extraCalls.Load(), 1, 50)
			}
		})

		t.Run("unclosed", func(t *testing.T) {
			th.ExpectLeak(t, func(t *testing.T) {
				in := FromChan(th.FromRange(0, 1000), nil)
				in = replaceWithError(in, 200, fmt.Errorf("err200"))
				in = th.DontClose(in)

				err := ForEach(in, n, func(int) error {
					return nil
				})

				th.ExpectError(t, err, "err200")
			})
		})

		th.RunSynctest(t, "context", func(t *testing.T) {
			ctx, scope := WithContext(t.Context())

			in := FromChan(th.FromRange(0, 20), nil)

			var state int64
			err := ForEach(in, n, func(x int) error {
				th.SimulateWork(1*time.Second, 2*time.Second)
				atomic.AddInt64(&state, 1)
				return nil
			}, scope)

			th.ExpectNoError(t, err)

			th.ExpectNoRace(state)
			th.ExpectDrainedChan(t, in)
			th.ExpectCanceledContext(t, ctx)
		})

		th.RunSynctest(t, "context (early cancellation)", func(t *testing.T) {
			ctx, scope := WithContext(t.Context())

			var stopwatch th.Stopwatch
			context.AfterFunc(ctx, stopwatch.Stop)

			in := FromChan(th.FromRange(0, 1000), nil)
			in = th.DelayEach(in, 1)

			var state int64
			err := ForEach(in, n, func(x int) error {
				th.SimulateWork(1*time.Second, 2*time.Second)
				atomic.AddInt64(&state, 1)
				if x == 200 {
					stopwatch.Start()
					return fmt.Errorf("err200")
				}
				return nil
			}, scope)

			th.ExpectError(t, err, "err200")

			th.ExpectNoRace(state)
			th.ExpectDrainedChan(t, in)
			th.ExpectCanceledContext(t, ctx)
			th.ExpectValue(t, stopwatch.Elapsed(), 0)
		})

		th.RunSynctest(t, "concurrency", func(t *testing.T) {
			in := FromChan(th.FromRange(0, 100), nil)

			var gauge th.InFlightGauge

			_ = ForEach(in, n, func(x int) error {
				gauge.Enter()
				defer gauge.Exit()
				th.SimulateWork(1*time.Second, 2*time.Second)

				return nil
			})

			th.ExpectValue(t, gauge.Max(), n)
		})

	})

	th.RunSynctest(t, "n=1 determinism", func(t *testing.T) {
		in := FromSlice([]int{1, 2, 3, 4, 5}, nil)

		// race detector must not complain about seen being accessed w/o synchronization
		var seen []int
		err := ForEach(in, 1, func(x int) error {
			seen = append(seen, x)
			return nil
		})

		th.ExpectNoError(t, err)
		th.ExpectSlice(t, seen, []int{1, 2, 3, 4, 5})
	})
}

// Any is a thin wrapper over ForEach. We test only Any's own semantics.
func TestAny(t *testing.T) {
	th.TestLevels(t, []int{1, 5}, func(t *testing.T, n int) {

		th.RunSynctest(t, "empty", func(t *testing.T) {
			in := FromSlice([]int{}, nil)
			res, err := Any(in, n, func(x int) (bool, error) {
				return false, nil
			})

			th.ExpectNoError(t, err)
			th.ExpectValue(t, res, false)
			th.ExpectDrainedChan(t, in)
		})

		th.RunSynctest(t, "none satisfy", func(t *testing.T) {
			in := FromChan(th.FromRange(0, 100), nil)
			res, err := Any(in, n, func(x int) (bool, error) {
				th.SimulateWork(1*time.Second, 2*time.Second)
				return false, nil
			})

			th.ExpectNoError(t, err)
			th.ExpectValue(t, res, false)
			th.ExpectDrainedChan(t, in)
		})

		th.RunSynctest(t, "match is first", func(t *testing.T) {
			in := FromChan(th.FromRange(0, 1000), nil)
			in = th.DelayEach(in, 1)

			// the match at 200 wins over the error at 500
			res, err := Any(in, n, func(x int) (bool, error) {
				th.SimulateWork(1*time.Second, 2*time.Second)
				if x == 200 {
					return true, nil
				}
				if x == 500 {
					return false, fmt.Errorf("err500")
				}
				return false, nil
			})

			th.ExpectNoError(t, err)
			th.ExpectValue(t, res, true)

			time.Sleep(24 * time.Hour) // wait for the background drain before closing the synctest bubble
		})

		th.RunSynctest(t, "error is first", func(t *testing.T) {
			in := FromChan(th.FromRange(0, 1000), nil)
			in = th.DelayEach(in, 1)

			// the error at 200 wins over the match at 500
			res, err := Any(in, n, func(x int) (bool, error) {
				th.SimulateWork(1*time.Second, 2*time.Second)
				if x == 200 {
					return false, fmt.Errorf("err200")
				}
				if x == 500 {
					return true, nil
				}
				return false, nil
			})

			th.ExpectError(t, err, "err200")
			th.ExpectValue(t, res, false)

			time.Sleep(24 * time.Hour) // wait for the background drain before closing the synctest bubble
		})

		th.RunSynctest(t, "(true,err) tuple", func(t *testing.T) {
			in := FromChan(th.FromRange(0, 1000), nil)
			res, err := Any(in, n, func(x int) (bool, error) {
				if x == 200 {
					return true, fmt.Errorf("err200")
				}
				return false, nil
			})

			th.ExpectError(t, err, "err200")
			th.ExpectValue(t, res, false)
		})

		th.RunSynctest(t, "context (early cancellation)", func(t *testing.T) {
			ctx, scope := WithContext(t.Context())

			in := FromChan(th.FromRange(0, 1000), nil)
			in = th.DelayEach(in, 1)

			res, err := Any(in, n, func(x int) (bool, error) {
				th.SimulateWork(1*time.Second, 2*time.Second)
				return x == 200, nil
			}, scope)

			th.ExpectNoError(t, err)
			th.ExpectValue(t, res, true)
			th.ExpectDrainedChan(t, in)
			th.ExpectCanceledContext(t, ctx)
		})
	})
}

// All is a thin wrapper over ForEach. We test only All's own semantics.
func TestAll(t *testing.T) {
	th.TestLevels(t, []int{1, 5}, func(t *testing.T, n int) {

		th.RunSynctest(t, "empty", func(t *testing.T) {
			in := FromSlice([]int{}, nil)

			// vacuous truth: an empty stream satisfies All
			res, err := All(in, n, func(int) (bool, error) {
				return false, nil
			})

			th.ExpectNoError(t, err)
			th.ExpectValue(t, res, true)
			th.ExpectDrainedChan(t, in)
		})

		th.RunSynctest(t, "all satisfy", func(t *testing.T) {
			in := FromChan(th.FromRange(0, 100), nil)
			res, err := All(in, n, func(x int) (bool, error) {
				th.SimulateWork(1*time.Second, 2*time.Second)
				return true, nil
			})

			th.ExpectNoError(t, err)
			th.ExpectValue(t, res, true)
			th.ExpectDrainedChan(t, in)
		})

		th.RunSynctest(t, "counterexample is first", func(t *testing.T) {
			in := FromChan(th.FromRange(0, 1000), nil)
			in = th.DelayEach(in, 1)

			// the counterexample at 200 wins over the error at 500
			res, err := All(in, n, func(x int) (bool, error) {
				th.SimulateWork(1*time.Second, 2*time.Second)
				if x == 200 {
					return false, nil
				}
				if x == 500 {
					return false, fmt.Errorf("err500")
				}
				return true, nil
			})

			th.ExpectNoError(t, err)
			th.ExpectValue(t, res, false)

			time.Sleep(24 * time.Hour) // wait for the background drain before closing the synctest bubble
		})

		th.RunSynctest(t, "error is first", func(t *testing.T) {
			in := FromChan(th.FromRange(0, 1000), nil)
			in = th.DelayEach(in, 1)

			// the error at 200 wins over the counterexample at 500
			res, err := All(in, n, func(x int) (bool, error) {
				th.SimulateWork(1*time.Second, 2*time.Second)
				if x == 200 {
					return false, fmt.Errorf("err200")
				}
				if x == 500 {
					return false, nil
				}
				return true, nil
			})

			th.ExpectError(t, err, "err200")
			th.ExpectValue(t, res, false)

			time.Sleep(24 * time.Hour) // wait for the background drain before closing the synctest bubble
		})

		th.RunSynctest(t, "(true,err) tuple", func(t *testing.T) {
			in := FromChan(th.FromRange(0, 1000), nil)
			res, err := All(in, n, func(x int) (bool, error) {
				if x == 200 {
					return true, fmt.Errorf("err200")
				}
				return true, nil
			})

			th.ExpectError(t, err, "err200")
			th.ExpectValue(t, res, false)
		})

		th.RunSynctest(t, "context (early cancellation)", func(t *testing.T) {
			ctx, scope := WithContext(t.Context())

			in := FromChan(th.FromRange(0, 1000), nil)
			in = th.DelayEach(in, 1)

			res, err := All(in, n, func(x int) (bool, error) {
				th.SimulateWork(1*time.Second, 2*time.Second)
				return x != 200, nil
			}, scope)

			th.ExpectNoError(t, err)
			th.ExpectValue(t, res, false)
			th.ExpectDrainedChan(t, in)
			th.ExpectCanceledContext(t, ctx)
		})
	})
}
