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
		in := FromChan(th.FromSlice([]int{}), nil)
		err := Err(in)

		synctest.Wait()
		th.ExpectDrainedChan(t, in)

		th.ExpectNoError(t, err)
	})

	th.RunSynctest(t, "no errors", func(t *testing.T) {
		in := FromChan(th.FromRange(0, 100), nil)
		err := Err(in)

		synctest.Wait()
		th.ExpectDrainedChan(t, in)

		th.ExpectNoError(t, err)
	})

	th.RunSynctest(t, "error", func(t *testing.T) {
		in := FromChan(th.FromRange(0, 100), nil)
		in = replaceWithError(in, 10, fmt.Errorf("err010"))
		err := Err(in)

		synctest.Wait()
		th.ExpectDrainedChan(t, in)

		th.ExpectError(t, err, "err010")
	})
}

func TestFirst(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		th.ExpectBlock(t, func(t *testing.T) {
			_, _, _ = First[int](nil)
		})
	})

	th.RunSynctest(t, "empty", func(t *testing.T) {
		in := FromChan(th.FromSlice([]int{}), nil)
		_, ok, err := First(in)

		synctest.Wait()
		th.ExpectDrainedChan(t, in)

		th.ExpectNoError(t, err)
		th.ExpectValue(t, ok, false)
	})

	th.RunSynctest(t, "value is first", func(t *testing.T) {
		in := FromChan(th.FromRange(1, 100), nil)
		in = replaceWithError(in, 10, fmt.Errorf("err010"))
		x, ok, err := First(in)

		synctest.Wait()
		th.ExpectDrainedChan(t, in)

		th.ExpectNoError(t, err)
		th.ExpectValue(t, ok, true)
		th.ExpectValue(t, x, 1)
	})

	th.RunSynctest(t, "error is first", func(t *testing.T) {
		in := FromChan(th.FromRange(1, 100), nil)
		in = replaceWithError(in, 1, fmt.Errorf("err001"))
		_, _, err := First(in)

		synctest.Wait()
		th.ExpectDrainedChan(t, in)

		th.ExpectError(t, err, "err001")
	})
}

func TestForEach(t *testing.T) {
	for _, n := range []int{1, 5} {
		t.Run(th.Name("nil", n), func(t *testing.T) {
			th.ExpectBlock(t, func(t *testing.T) {
				_ = ForEach(nil, n, func(int) error { return nil })
			})
		})

		th.RunSynctest(t, th.Name("no errors", n), func(t *testing.T) {
			in := FromChan(th.FromRange(0, 10), nil)

			var sum atomic.Int64
			err := ForEach(in, n, func(x int) error {
				sum.Add(int64(x))
				return nil
			})

			synctest.Wait()
			th.ExpectDrainedChan(t, in)

			th.ExpectNoError(t, err)
			th.ExpectValue(t, sum.Load(), int64(9*10/2))
		})

		th.RunSynctest(t, th.Name("error in input", n), func(t *testing.T) {
			in := FromChan(th.FromRange(0, 1000), nil)
			in = replaceWithError(in, 200, fmt.Errorf("err200"))

			var cnt atomic.Int64
			err := ForEach(in, n, func(x int) error {
				// Early exit isn't instantaneous in concurrent code: one goroutine detects the
				// condition and initiates the exit, and it takes time to propagate to the others.
				// Until it does, the other workers keep pulling items from the input.
				//
				// For this test we can't rely entirely on the scheduler. We might get unlucky and the early exit
				// will propagate slowly enough that the other workers reach the end of the input channel.
				//
				// We use sleeps and synctest to give every item a roughly equal but randomized amount of work.
				// This aligns with real-world pipelines, and it bounds how far the fastest worker can overrun the slowest one,
				// and therefore how many extra items can get processed after the early exit was initiated.
				// The upper bound is:
				//   maxExtra = (n - 1) * maxSleep / minSleep
				th.RandomSleep(1*time.Second, 2*time.Second)

				cnt.Add(1)
				return nil
			})

			// Make sure all sleeping workers have had a chance to finish
			time.Sleep(10 * time.Second)
			th.ExpectDrainedChan(t, in)

			th.ExpectError(t, err, "err200")
			if cnt.Load() > 250 {
				t.Errorf("early return did not happen")
			}
		})

		th.RunSynctest(t, th.Name("error in func", n), func(t *testing.T) {
			in := FromChan(th.FromRange(0, 1000), nil)

			var cnt atomic.Int64
			err := ForEach(in, n, func(x int) error {
				// Balance per-item work so early exit stays observable
				th.RandomSleep(1*time.Second, 2*time.Second)

				if x == 100 {
					return fmt.Errorf("err100")
				}
				cnt.Add(1)
				return nil
			})

			time.Sleep(10 * time.Second)
			th.ExpectDrainedChan(t, in)

			th.ExpectError(t, err, "err100")
			if cnt.Load() > 150 {
				t.Errorf("early return did not happen")
			}
		})
	}
}

func TestAnyAll(t *testing.T) {
	cases := []struct {
		name string

		inputErrorPos     int
		functionErrorPos  int
		conditionBreakPos int

		expectError        string
		expectedResult     bool
		expectedIterations int
	}{
		{name: "no errors,cond is true", inputErrorPos: -1, functionErrorPos: -1, conditionBreakPos: -1, expectedResult: true, expectError: "", expectedIterations: 1000},
		{name: "no errors,cond is false", inputErrorPos: -1, functionErrorPos: -1, conditionBreakPos: 200, expectedResult: false, expectError: "", expectedIterations: 200},

		{name: "early error in input,cond is true", inputErrorPos: 300, functionErrorPos: -1, conditionBreakPos: -1, expectedResult: false, expectError: "err300", expectedIterations: 300},
		{name: "early error in input,cond is false", inputErrorPos: 300, functionErrorPos: -1, conditionBreakPos: 500, expectedResult: false, expectError: "err300", expectedIterations: 300},
		{name: "late error in input,cond is false", inputErrorPos: 700, functionErrorPos: -1, conditionBreakPos: 500, expectedResult: false, expectError: "", expectedIterations: 500},

		{name: "early error in func,cond is true", inputErrorPos: -1, functionErrorPos: 200, conditionBreakPos: -1, expectedResult: false, expectError: "err200", expectedIterations: 200},
		{name: "early error in func,cond is false", inputErrorPos: -1, functionErrorPos: 200, conditionBreakPos: 400, expectedResult: false, expectError: "err200", expectedIterations: 200},
		{name: "late error in func,cond is false", inputErrorPos: -1, functionErrorPos: 600, conditionBreakPos: 400, expectedResult: false, expectError: "", expectedIterations: 400},
	}

	for _, n := range []int{1, 5} {
		t.Run(th.Name("nil", n), func(t *testing.T) {
			th.ExpectBlock(t, func(t *testing.T) {
				_, _ = All(nil, n, func(int) (bool, error) { return true, nil })
			})
		})

		th.RunSynctest(t, th.Name("empty", n), func(t *testing.T) {
			in := FromSlice([]int{}, nil)

			res, err := All(in, n, func(int) (bool, error) {
				return false, nil
			})

			synctest.Wait()
			th.ExpectDrainedChan(t, in)

			th.ExpectNoError(t, err)
			th.ExpectValue(t, res, true)
		})

		for _, testCase := range cases {
			th.RunSynctest(t, th.Name(testCase.name, n), func(t *testing.T) {
				in := FromChan(th.FromRange(0, 1000), nil)
				if testCase.inputErrorPos >= 0 {
					in = replaceWithError(in, testCase.inputErrorPos, fmt.Errorf("err%03d", testCase.inputErrorPos))
				}

				var cnt atomic.Int64
				ok, err := All(in, n, func(x int) (bool, error) {
					// Balance per-item work so early exit stays observable
					// See comments in TestForEach for more details
					th.RandomSleep(1*time.Second, 2*time.Second)

					if x == testCase.functionErrorPos {
						return false, fmt.Errorf("err%03d", testCase.functionErrorPos)
					}

					cnt.Add(1)
					return x != testCase.conditionBreakPos, nil
				})

				time.Sleep(10 * time.Second)
				th.ExpectDrainedChan(t, in)

				if testCase.expectError != "" {
					th.ExpectError(t, err, testCase.expectError)
				} else {
					th.ExpectNoError(t, err)
				}

				th.ExpectValue(t, ok, testCase.expectedResult)

				if cnt := cnt.Load(); cnt > int64(testCase.expectedIterations+50) {
					t.Errorf("early return did not happen")
				} else if cnt < int64(testCase.expectedIterations) {
					t.Errorf("early return misconfigured in the test case")
				}
			})

		}
	}

}
