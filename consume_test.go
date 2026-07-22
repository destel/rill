package rill

import (
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

		th.ExpectDrainedChan(t, in)

		th.ExpectNoError(t, err)
	})

	th.RunSynctest(t, "no errors", func(t *testing.T) {
		in := FromChan(th.FromRange(0, 20), nil)
		err := Err(in)

		th.ExpectDrainedChan(t, in)

		th.ExpectNoError(t, err)
	})

	th.RunSynctest(t, "error", func(t *testing.T) {
		in := FromChan(th.FromRange(0, 20), nil)
		in = replaceWithError(in, 10, fmt.Errorf("err010"))
		in = replaceWithError(in, 15, fmt.Errorf("err015"))
		in = th.DelayEach(in, 1*time.Nanosecond) // needed for inStillOpen assertion

		err := Err(in)

		th.ExpectError(t, err, "err010")

		_, inStillOpen := <-in
		th.ExpectValue(t, inStillOpen, true)

		th.WaitForInflightWork()
		th.ExpectDrainedChan(t, in)
	})

	t.Run("unclosed", func(t *testing.T) {
		th.ExpectLeak(t, func(t *testing.T) {
			in := FromChan(th.FromRange(0, 20), nil)
			in = replaceWithError(in, 10, fmt.Errorf("err010"))
			in = th.DontClose(in)

			err := Err(in)

			th.ExpectError(t, err, "err010")
		})
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
		_, ok, err := First(in)

		th.ExpectDrainedChan(t, in)

		th.ExpectNoError(t, err)
		th.ExpectValue(t, ok, false)
	})

	th.RunSynctest(t, "value is first", func(t *testing.T) {
		in := FromChan(th.FromRange(0, 20), nil)
		in = replaceWithError(in, 10, fmt.Errorf("err010"))
		in = th.DelayEach(in, 1*time.Nanosecond) // needed for inStillOpen assertion

		x, ok, err := First(in)

		th.ExpectNoError(t, err)
		th.ExpectValue(t, ok, true)
		th.ExpectValue(t, x, 0)

		_, inStillOpen := <-in
		th.ExpectValue(t, inStillOpen, true)

		th.WaitForInflightWork()
		th.ExpectDrainedChan(t, in)
	})

	th.RunSynctest(t, "error is first", func(t *testing.T) {
		in := FromChan(th.FromRange(0, 20), nil)
		in = replaceWithError(in, 0, fmt.Errorf("err000"))
		in = th.DelayEach(in, 1*time.Nanosecond) // needed for inStillOpen assertion

		x, ok, err := First(in)

		th.ExpectError(t, err, "err000")
		th.ExpectValue(t, x, 0)
		th.ExpectValue(t, ok, false)

		_, inStillOpen := <-in
		th.ExpectValue(t, inStillOpen, true)

		th.WaitForInflightWork()
		th.ExpectDrainedChan(t, in)
	})

	t.Run("unclosed", func(t *testing.T) {
		th.ExpectLeak(t, func(t *testing.T) {
			in := FromChan(th.FromRange(0, 20), nil)
			in = th.DontClose(in)
			x, ok, err := First(in)

			th.ExpectNoError(t, err)
			th.ExpectValue(t, ok, true)
			th.ExpectValue(t, x, 0)
		})
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

			th.ExpectDrainedChan(t, in)

			th.ExpectNoError(t, err)
			th.ExpectValue(t, sum.Load(), int64(19*20/2))
		})

		th.RunSynctest(t, "error in input", func(t *testing.T) {
			in := FromChan(th.FromRange(0, 1000), nil)
			in = replaceWithError(in, 200, fmt.Errorf("err200"))
			in = th.DelayEach(in, 1*time.Nanosecond) // needed for inStillOpen assertion

			var extraCalls atomic.Int64
			err := ForEach(in, n, func(x int) error {
				extraCalls.Add(1)
				th.SimulateWork(1*time.Second, 2*time.Second)
				return nil
			})
			extraCalls.Store(0)

			th.ExpectError(t, err, "err200")

			_, inStillOpen := <-in
			th.ExpectValue(t, inStillOpen, true)

			th.WaitForInflightWork()
			th.ExpectDrainedChan(t, in)

			if n == 1 {
				th.ExpectValue(t, extraCalls.Load(), 0)
			} else {
				th.ExpectBetween(t, extraCalls.Load(), 0, 50)
			}
		})

		th.RunSynctest(t, "error in func", func(t *testing.T) {
			in := FromChan(th.FromRange(0, 1000), nil)
			in = th.DelayEach(in, 1*time.Nanosecond) // needed for inStillOpen assertion

			var extraCalls atomic.Int64
			err := ForEach(in, n, func(x int) error {
				extraCalls.Add(1)
				th.SimulateWork(1*time.Second, 2*time.Second)
				if x == 200 {
					return fmt.Errorf("err200")
				}
				return nil
			})
			extraCalls.Store(0)

			th.ExpectError(t, err, "err200")

			_, inStillOpen := <-in
			th.ExpectValue(t, inStillOpen, true)

			th.WaitForInflightWork()
			th.ExpectDrainedChan(t, in)

			if n == 1 {
				th.ExpectValue(t, extraCalls.Load(), 0)
			} else {
				th.ExpectBetween(t, extraCalls.Load(), 0, 50)
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
		})

		th.RunSynctest(t, "none satisfy", func(t *testing.T) {
			in := FromChan(th.FromRange(0, 100), nil)
			res, err := Any(in, n, func(x int) (bool, error) {
				th.SimulateWork(1*time.Second, 2*time.Second)
				return false, nil
			})

			th.ExpectNoError(t, err)
			th.ExpectValue(t, res, false)
		})

		th.RunSynctest(t, "match is first", func(t *testing.T) {
			in := FromChan(th.FromRange(0, 1000), nil)

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

			th.WaitForInflightWork()
		})

		th.RunSynctest(t, "error is first", func(t *testing.T) {
			in := FromChan(th.FromRange(0, 1000), nil)

			// the error at 200 wins over the would-be match at 500
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

			th.WaitForInflightWork()
		})

		th.RunSynctest(t, "(true,err) tuple", func(t *testing.T) {
			in := FromChan(th.FromRange(0, 1000), nil)
			res, err := Any(in, n, func(x int) (bool, error) {
				th.SimulateWork(1*time.Second, 2*time.Second)
				if x == 200 {
					return true, fmt.Errorf("err200")
				}
				return false, nil
			})

			th.ExpectError(t, err, "err200")
			th.ExpectValue(t, res, false)

			th.WaitForInflightWork()
		})
	})
}

// All is a thin wrapper over ForEach. We test only All's own semantics.
func TestAll(t *testing.T) {
	th.TestLevels(t, []int{1, 5}, func(t *testing.T, n int) {

		th.RunSynctest(t, "empty", func(t *testing.T) {
			// vacuous truth: an empty stream satisfies All
			res, err := All(FromSlice([]int{}, nil), n, func(int) (bool, error) {
				return false, nil
			})

			th.ExpectNoError(t, err)
			th.ExpectValue(t, res, true)
		})

		th.RunSynctest(t, "all satisfy", func(t *testing.T) {
			in := FromChan(th.FromRange(0, 100), nil)
			res, err := All(in, n, func(x int) (bool, error) {
				th.SimulateWork(1*time.Second, 2*time.Second)
				return true, nil
			})

			th.ExpectNoError(t, err)
			th.ExpectValue(t, res, true)
		})

		th.RunSynctest(t, "counterexample is first", func(t *testing.T) {
			in := FromChan(th.FromRange(0, 1000), nil)

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

			th.WaitForInflightWork()
		})

		th.RunSynctest(t, "error is first", func(t *testing.T) {
			in := FromChan(th.FromRange(0, 1000), nil)

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

			th.WaitForInflightWork()
		})

		th.RunSynctest(t, "(true,err) tuple", func(t *testing.T) {
			in := FromChan(th.FromRange(0, 1000), nil)
			res, err := All(in, n, func(x int) (bool, error) {
				th.SimulateWork(1*time.Second, 2*time.Second)
				if x == 200 {
					return true, fmt.Errorf("err200")
				}
				return true, nil
			})

			th.ExpectError(t, err, "err200")
			th.ExpectValue(t, res, false)

			th.WaitForInflightWork()
		})
	})
}
