package rill

import (
	"fmt"
	"sync/atomic"
	"testing"
	"testing/synctest"
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
		err := Err(in)

		synctest.Wait()
		th.ExpectDrainedChan(t, in)

		th.ExpectError(t, err, "err010")
	})

	t.Run("unclosed", func(t *testing.T) {
		th.ExpectLeak(t, func(t *testing.T) {
			in := FromChan(th.FromRange(0, 20), nil)
			in = replaceWithError(in, 10, fmt.Errorf("err010"))
			in = th.DontClose(in)

			_ = Err(in)
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
		x, ok, err := First(in)

		synctest.Wait()
		th.ExpectDrainedChan(t, in)

		th.ExpectNoError(t, err)
		th.ExpectValue(t, ok, true)
		th.ExpectValue(t, x, 0)
	})

	th.RunSynctest(t, "error is first", func(t *testing.T) {
		in := FromChan(th.FromRange(0, 20), nil)
		in = replaceWithError(in, 0, fmt.Errorf("err000"))
		_, ok, err := First(in)

		synctest.Wait()
		th.ExpectDrainedChan(t, in)

		th.ExpectError(t, err, "err000")
		th.ExpectValue(t, ok, false)
	})

	t.Run("unclosed", func(t *testing.T) {
		th.ExpectLeak(t, func(t *testing.T) {
			in := FromChan(th.FromRange(0, 20), nil)
			in = th.DontClose(in)
			_, _, _ = First(in)
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

func TestAll(t *testing.T) {
	// All is a thin negating wrapper over Any (All == !Any(!f)). Concurrency and
	// short-circuiting live in Any and are covered by TestAny; these are just
	// semantic smoke checks that the negation is wired correctly.

	th.RunSynctest(t, "empty", func(t *testing.T) {
		res, err := All(FromSlice([]int{}, nil), 1, func(int) (bool, error) {
			return false, nil
		})
		th.ExpectNoError(t, err)
		th.ExpectValue(t, res, true)
	})

	th.RunSynctest(t, "all satisfy", func(t *testing.T) {
		res, err := All(FromSlice([]int{2, 4, 6}, nil), 1, func(x int) (bool, error) {
			return x%2 == 0, nil
		})
		th.ExpectNoError(t, err)
		th.ExpectValue(t, res, true)
	})

	th.RunSynctest(t, "one does not satisfy", func(t *testing.T) {
		res, err := All(FromSlice([]int{2, 3, 4}, nil), 1, func(x int) (bool, error) {
			return x%2 == 0, nil
		})
		th.ExpectNoError(t, err)
		th.ExpectValue(t, res, false)
	})
}

func TestAnyAllAlwaysFalseOnError(t *testing.T) {
	// Test that both Any and All always return false when the predicate returns an error.

	testCases := []struct {
		name     string
		function func(in <-chan Try[int], n int, f func(int) (bool, error)) (bool, error)
		ret      bool
	}{
		{"All-false", All[int], false},
		{"All-true", All[int], true},
		{"Any-false", Any[int], false},
		{"Any-true", Any[int], true},
	}

	for _, testCase := range testCases {
		th.RunSynctest(t, testCase.name, func(t *testing.T) {
			in := FromChan(th.FromRange(0, 100), nil)
			res, err := testCase.function(in, 1, func(int) (bool, error) {
				return testCase.ret, fmt.Errorf("some error")
			})

			th.ExpectError(t, err, "some error")
			th.ExpectValue(t, res, false)
		})
	}
}
