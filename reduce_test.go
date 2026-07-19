package rill

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/destel/rill/internal/th"
)

func TestReduce(t *testing.T) {
	th.TestLevels(t, []int{1, 4}, func(t *testing.T, n int) {

		t.Run("nil", func(t *testing.T) {
			th.ExpectBlock(t, func(t *testing.T) {
				_, _, _ = Reduce(nil, n, func(x, y int) (int, error) { return x + y, nil })
			})
		})

		th.RunSynctest(t, "empty", func(t *testing.T) {
			in := FromSlice([]int{}, nil)

			out, ok, err := Reduce(in, n, func(x, y int) (int, error) {
				th.SimulateWork(1*time.Second, 2*time.Second)
				return x + y, nil
			})

			th.ExpectDrainedChan(t, in)

			th.ExpectNoError(t, err)
			th.ExpectValue(t, out, 0)
			th.ExpectValue(t, ok, false)
		})

		th.RunSynctest(t, "single value stream", func(t *testing.T) {
			in := FromSlice([]int{5}, nil)

			var reducerCalls atomic.Int64
			out, ok, err := Reduce(in, n, func(x, y int) (int, error) {
				reducerCalls.Add(1)
				th.SimulateWork(1*time.Second, 2*time.Second)
				return x + y, nil
			})

			th.ExpectDrainedChan(t, in)

			th.ExpectNoError(t, err)
			th.ExpectValue(t, out, 5)
			th.ExpectValue(t, ok, true)

			th.ExpectValue(t, reducerCalls.Load(), 0)
		})

		th.RunSynctest(t, "single error stream", func(t *testing.T) {
			in := FromSlice([]int{}, fmt.Errorf("err0"))

			out, ok, err := Reduce(in, n, func(x, y int) (int, error) {
				th.SimulateWork(1*time.Second, 2*time.Second)
				return x + y, nil
			})

			th.WaitForInflightWork()
			th.ExpectDrainedChan(t, in)

			th.ExpectError(t, err, "err0")
			th.ExpectValue(t, out, 0)
			th.ExpectValue(t, ok, false)
		})

		th.RunSynctest(t, "no errors", func(t *testing.T) {
			in := FromChan(th.FromRange(0, 100), nil)

			out, ok, err := Reduce(in, n, func(x, y int) (int, error) {
				th.SimulateWork(1*time.Second, 2*time.Second)
				return x + y, nil
			})

			th.ExpectDrainedChan(t, in)

			th.ExpectNoError(t, err)
			th.ExpectValue(t, out, 99*100/2)
			th.ExpectValue(t, ok, true)
		})

		th.RunSynctest(t, "error in input", func(t *testing.T) {
			in := FromChan(th.FromRange(0, 1000), nil)
			in = replaceWithError(in, 200, fmt.Errorf("err200"))
			in = th.DelayEach(in, 1*time.Nanosecond) // needed for inStillOpen assertion

			var extraCalls atomic.Int64
			out, ok, err := Reduce(in, n, func(x, y int) (int, error) {
				extraCalls.Add(1)
				th.SimulateWork(1*time.Second, 2*time.Second)
				return x + y, nil
			})
			extraCalls.Store(0)

			th.ExpectError(t, err, "err200")
			th.ExpectValue(t, out, 0)
			th.ExpectValue(t, ok, false)

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
			var i atomic.Int64
			out, ok, err := Reduce(in, n, func(x, y int) (int, error) {
				extraCalls.Add(1)
				th.SimulateWork(1*time.Second, 2*time.Second)
				if i.Add(1) == 200 {
					return 0, fmt.Errorf("err200")
				}
				return x + y, nil
			})
			extraCalls.Store(0)

			th.ExpectError(t, err, "err200")
			th.ExpectValue(t, out, 0)
			th.ExpectValue(t, ok, false)

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

				out, ok, err := Reduce(in, n, func(x, y int) (int, error) {
					return x + y, nil
				})

				th.ExpectError(t, err, "err200")
				th.ExpectValue(t, out, 0)
				th.ExpectValue(t, ok, false)
			})
		})

	})

	th.RunSynctest(t, "n=1 determinism", func(t *testing.T) {
		in := FromSlice([]string{"1", "2", "3", "4", "5"}, nil)

		// race detector must not complain about dummy being accessed w/o synchronization
		dummy := 0
		out, ok, err := Reduce(in, 1, func(x, y string) (string, error) {
			dummy++
			return x + y, nil
		})

		th.ExpectNoError(t, err)
		th.ExpectValue(t, out, "12345")
		th.ExpectValue(t, ok, true)

		th.ExpectValue(t, dummy, 4)
	})
}

func TestMapReduce(t *testing.T) {
	th.TestVariants(t, "nm", []int{1, 4}, func(t *testing.T, nm int) {
		th.TestVariants(t, "nr", []int{1, 4}, func(t *testing.T, nr int) {

			t.Run("nil", func(t *testing.T) {
				th.ExpectBlock(t, func(t *testing.T) {
					_, _ = MapReduce(nil,
						nm, func(x int) (string, int, error) {
							return fmt.Sprint(x), x, nil
						},
						nr, func(x, y int) (int, error) {
							return x + y, nil
						})
				})
			})

			th.RunSynctest(t, "empty", func(t *testing.T) {
				in := FromSlice([]int{}, nil)

				out, err := MapReduce(in,
					nm, func(x int) (string, int, error) {
						th.SimulateWork(1*time.Second, 2*time.Second)
						return fmt.Sprintf("%d-digit", len(fmt.Sprint(x))), x, nil
					},
					nr, func(x, y int) (int, error) {
						th.SimulateWork(10*time.Second, 20*time.Second)
						return x + y, nil
					})

				th.ExpectDrainedChan(t, in)

				th.ExpectNoError(t, err)
				th.ExpectMap(t, out, map[string]int{})
				if out == nil {
					t.Errorf("expected empty non-nil map for empty input, got nil")
				}
			})

			th.RunSynctest(t, "single value keys", func(t *testing.T) {
				in := FromSlice([]int{1, 2, 3, 4, 5}, nil)

				var reducerCalls atomic.Int64

				out, err := MapReduce(in,
					nm, func(x int) (string, int, error) {
						th.SimulateWork(1*time.Second, 2*time.Second)
						return fmt.Sprint(x), x, nil
					},
					nr, func(x, y int) (int, error) {
						reducerCalls.Add(1)
						th.SimulateWork(10*time.Second, 20*time.Second)
						return x + y, nil
					},
				)

				th.ExpectDrainedChan(t, in)

				th.ExpectNoError(t, err)
				th.ExpectMap(t, out, map[string]int{
					"1": 1,
					"2": 2,
					"3": 3,
					"4": 4,
					"5": 5,
				})

				th.ExpectValue(t, reducerCalls.Load(), 0)
			})

			th.RunSynctest(t, "no errors", func(t *testing.T) {
				in := FromChan(th.FromRange(0, 200), nil)

				out, err := MapReduce(in,
					nm, func(x int) (string, int, error) {
						th.SimulateWork(1*time.Second, 2*time.Second)
						return fmt.Sprintf("%d-digit", len(fmt.Sprint(x))), x, nil
					},
					nr, func(x, y int) (int, error) {
						th.SimulateWork(10*time.Second, 20*time.Second)
						return x + y, nil
					},
				)

				th.ExpectDrainedChan(t, in)

				th.ExpectNoError(t, err)
				th.ExpectMap(t, out, map[string]int{
					"1-digit": (0 + 9) * 10 / 2,
					"2-digit": (10 + 99) * 90 / 2,
					"3-digit": (100 + 199) * 100 / 2,
				})
			})

			th.RunSynctest(t, "error in input", func(t *testing.T) {
				in := FromChan(th.FromRange(0, 1000), nil)
				in = replaceWithError(in, 200, fmt.Errorf("err200"))
				in = th.DelayEach(in, 1*time.Nanosecond) // needed for inStillOpen assertion

				var extraMapCalls, extraReduceCalls atomic.Int64
				out, err := MapReduce(in,
					nm, func(x int) (string, int, error) {
						extraMapCalls.Add(1)
						th.SimulateWork(1*time.Second, 2*time.Second)
						return fmt.Sprintf("%d-digit", len(fmt.Sprint(x))), x, nil
					},
					nr, func(x, y int) (int, error) {
						extraReduceCalls.Add(1)
						th.SimulateWork(10*time.Second, 20*time.Second)
						return x + y, nil
					},
				)
				extraMapCalls.Store(0)
				extraReduceCalls.Store(0)

				th.ExpectError(t, err, "err200")
				if out != nil {
					t.Errorf("expected nil map on error, got %v", out)
				}

				_, inStillOpen := <-in
				th.ExpectValue(t, inStillOpen, true)

				th.WaitForInflightWork()
				th.ExpectDrainedChan(t, in)

				if nm == 1 && nr == 1 {
					th.ExpectValue(t, extraMapCalls.Load(), 0)
					th.ExpectValue(t, extraReduceCalls.Load(), 0)
				} else {
					th.ExpectBetween(t, extraMapCalls.Load(), 0, 50)
					th.ExpectBetween(t, extraReduceCalls.Load(), 0, 50)
				}
			})

			th.RunSynctest(t, "error in mapper", func(t *testing.T) {
				in := FromChan(th.FromRange(0, 1000), nil)
				in = th.DelayEach(in, 1*time.Nanosecond) // needed for inStillOpen assertion

				var extraMapCalls, extraReduceCalls atomic.Int64
				var i atomic.Int64
				out, err := MapReduce(in,
					nm, func(x int) (string, int, error) {
						extraMapCalls.Add(1)
						th.SimulateWork(1*time.Second, 2*time.Second)
						if i.Add(1) == 200 {
							return "", 0, fmt.Errorf("err200")
						}
						return fmt.Sprintf("%d-digit", len(fmt.Sprint(x))), x, nil
					},
					nr, func(x, y int) (int, error) {
						extraReduceCalls.Add(1)
						th.SimulateWork(10*time.Second, 20*time.Second)
						return x + y, nil
					},
				)
				extraMapCalls.Store(0)
				extraReduceCalls.Store(0)

				th.ExpectError(t, err, "err200")
				if out != nil {
					t.Errorf("expected nil map on error, got %v", out)
				}

				_, inStillOpen := <-in
				th.ExpectValue(t, inStillOpen, true)

				th.WaitForInflightWork()
				th.ExpectDrainedChan(t, in)

				if nm == 1 && nr == 1 {
					th.ExpectValue(t, extraMapCalls.Load(), 0)
					th.ExpectValue(t, extraReduceCalls.Load(), 0)
				} else {
					th.ExpectBetween(t, extraMapCalls.Load(), 0, 50)
					th.ExpectBetween(t, extraReduceCalls.Load(), 0, 50)
				}
			})

			th.RunSynctest(t, "error in reducer", func(t *testing.T) {
				in := FromChan(th.FromRange(0, 1000), nil)
				in = th.DelayEach(in, 1*time.Nanosecond) // needed for inStillOpen assertion

				var extraMapCalls, extraReduceCalls atomic.Int64
				var i atomic.Int64
				out, err := MapReduce(in,
					nm, func(x int) (string, int, error) {
						extraMapCalls.Add(1)
						th.SimulateWork(1*time.Second, 2*time.Second)
						return fmt.Sprintf("%d-digit", len(fmt.Sprint(x))), x, nil
					},
					nr, func(x, y int) (int, error) {
						extraReduceCalls.Add(1)
						th.SimulateWork(10*time.Second, 20*time.Second)
						if i.Add(1) == 200 {
							return 0, fmt.Errorf("err200")
						}
						return x + y, nil
					},
				)
				extraMapCalls.Store(0)
				extraReduceCalls.Store(0)

				th.ExpectError(t, err, "err200")
				if out != nil {
					t.Errorf("expected nil map on error, got %v", out)
				}

				_, inStillOpen := <-in
				th.ExpectValue(t, inStillOpen, true)

				th.WaitForInflightWork()
				th.ExpectDrainedChan(t, in)

				if nm == 1 && nr == 1 {
					th.ExpectValue(t, extraMapCalls.Load(), 0)
					th.ExpectValue(t, extraReduceCalls.Load(), 0)
				} else {
					th.ExpectBetween(t, extraMapCalls.Load(), 0, 50)
					th.ExpectBetween(t, extraReduceCalls.Load(), 0, 50)
				}
			})

			t.Run("unclosed", func(t *testing.T) {
				th.ExpectLeak(t, func(t *testing.T) {
					in := FromChan(th.FromRange(0, 1000), nil)
					in = replaceWithError(in, 200, fmt.Errorf("err200"))
					in = th.DontClose(in)

					out, err := MapReduce(in,
						nm, func(int) (string, int, error) {
							return "", 0, nil
						},
						nr, func(int, int) (int, error) {
							return 0, nil
						},
					)

					th.ExpectError(t, err, "err200")
					if out != nil {
						t.Errorf("expected nil map on error, got %v", out)
					}
				})
			})

		})
	})

	th.RunSynctest(t, "n=1 determinism", func(t *testing.T) {
		in := FromSlice([]int{1, 2, 3, 4, 5}, nil)

		// race detector must not complain about dummy being accessed w/o synchronization
		dummy1 := 0
		dummy2 := 0
		out, err := MapReduce(in,
			1, func(x int) (bool, string, error) {
				dummy1++
				return x%2 == 0, fmt.Sprint(x), nil
			},
			1, func(x, y string) (string, error) {
				dummy2++
				return x + y, nil
			})

		th.ExpectNoError(t, err)
		th.ExpectMap(t, out, map[bool]string{
			true:  "24",
			false: "135",
		})

		th.ExpectValue(t, dummy1, 5)
		th.ExpectValue(t, dummy2, 3)
	})
}
