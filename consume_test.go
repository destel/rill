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
		_, _, err := First(in)

		synctest.Wait()
		th.ExpectDrainedChan(t, in)

		th.ExpectError(t, err, "err000")
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

		th.RunSynctest(t, th.Name("error in input", n), func(t *testing.T) {
			in := FromChan(th.FromRange(0, 1000), nil)
			in = replaceWithError(in, 200, fmt.Errorf("err200"))

			var cnt atomic.Int64
			err := ForEach(in, n, func(x int) error {
				th.SimulateWork(1*time.Second, 2*time.Second)

				cnt.Add(1)
				return nil
			})

			th.WaitForInflightWork()
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
				th.SimulateWork(1*time.Second, 2*time.Second)

				if x == 100 {
					return fmt.Errorf("err100")
				}
				cnt.Add(1)
				return nil
			})

			th.WaitForInflightWork()
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
					th.SimulateWork(1*time.Second, 2*time.Second)

					if x == testCase.functionErrorPos {
						return false, fmt.Errorf("err%03d", testCase.functionErrorPos)
					}

					cnt.Add(1)
					return x != testCase.conditionBreakPos, nil
				})

				th.WaitForInflightWork()
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
