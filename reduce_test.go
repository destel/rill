package rill

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/destel/rill/internal/th"
)

func TestReduce(t *testing.T) {
	for _, n := range []int{1, 4} {
		t.Run(th.Name("nil", n), func(t *testing.T) {
			th.ExpectBlock(t, func(t *testing.T) {
				_, _, _ = Reduce(nil, n, func(x, y int) (int, error) { return x + y, nil })
			})
		})

		th.RunSynctest(t, th.Name("empty", n), func(t *testing.T) {
			in := FromSlice([]int{}, nil)

			_, ok, err := Reduce(in, n, func(x, y int) (int, error) {
				th.SimulateWork(1*time.Second, 2*time.Second)
				return x + y, nil
			})

			th.ExpectDrainedChan(t, in)

			th.ExpectNoError(t, err)
			th.ExpectValue(t, ok, false)
		})

		th.RunSynctest(t, th.Name("single item", n), func(t *testing.T) {
			in := FromSlice([]int{5}, nil)

			out, ok, err := Reduce(in, n, func(x, y int) (int, error) {
				th.SimulateWork(1*time.Second, 2*time.Second)
				return x + y, nil
			})

			th.ExpectDrainedChan(t, in)

			th.ExpectNoError(t, err)
			th.ExpectValue(t, out, 5)
			th.ExpectValue(t, ok, true)
		})

		th.RunSynctest(t, th.Name("single error stream", n), func(t *testing.T) {
			in := FromSlice([]int{}, fmt.Errorf("err0"))

			_, ok, err := Reduce(in, n, func(x, y int) (int, error) {
				th.SimulateWork(1*time.Second, 2*time.Second)
				return x + y, nil
			})

			th.WaitForInflightWork()
			th.ExpectDrainedChan(t, in)

			th.ExpectError(t, err, "err0")
			th.ExpectValue(t, ok, false)
		})

		th.RunSynctest(t, th.Name("no errors", n), func(t *testing.T) {
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

		th.RunSynctest(t, th.Name("error in input", n), func(t *testing.T) {
			in := FromChan(th.FromRange(0, 1000), nil)
			in = replaceWithError(in, 200, fmt.Errorf("err200"))

			var iterations atomic.Int64
			_, _, err := Reduce(in, n, func(x, y int) (int, error) {
				th.SimulateWork(1*time.Second, 2*time.Second)
				iterations.Add(1)
				return x + y, nil
			})

			th.WaitForInflightWork()
			th.ExpectDrainedChan(t, in)

			th.ExpectError(t, err, "err200")

			if iterations.Load() > 250 {
				t.Errorf("early exit did not happen")
			}
		})

		th.RunSynctest(t, th.Name("error in func", n), func(t *testing.T) {
			in := FromChan(th.FromRange(0, 1000), nil)

			var iterations atomic.Int64
			_, _, err := Reduce(in, n, func(x, y int) (int, error) {
				th.SimulateWork(1*time.Second, 2*time.Second)
				if iterations.Add(1) == 200 {
					return 0, fmt.Errorf("err200")
				}
				return x + y, nil
			})

			th.WaitForInflightWork()
			th.ExpectDrainedChan(t, in)

			th.ExpectError(t, err, "err200")

			if iterations.Load() > 250 {
				t.Errorf("early exit did not happen")
			}
		})
	}
}

func TestMapReduce(t *testing.T) {
	for _, nm := range []int{1, 4} {
		for _, nr := range []int{1, 4} {
			t.Run(th.Name("nil", nm, nr), func(t *testing.T) {
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

			th.RunSynctest(t, th.Name("empty", nm, nr), func(t *testing.T) {
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
			})

			th.RunSynctest(t, th.Name("no errors", nm, nr), func(t *testing.T) {
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

			th.RunSynctest(t, th.Name("error in input", nm, nr), func(t *testing.T) {
				in := FromChan(th.FromRange(0, 1000), nil)
				in = replaceWithError(in, 200, fmt.Errorf("err200"))

				var iterationsMap, iterationsReduce atomic.Int64
				out, err := MapReduce(in,
					nm, func(x int) (string, int, error) {
						th.SimulateWork(1*time.Second, 2*time.Second)
						iterationsMap.Add(1)
						return fmt.Sprintf("%d-digit", len(fmt.Sprint(x))), x, nil
					},
					nr, func(x, y int) (int, error) {
						th.SimulateWork(10*time.Second, 20*time.Second)
						iterationsReduce.Add(1)
						return x + y, nil
					},
				)

				th.WaitForInflightWork()
				th.ExpectDrainedChan(t, in)

				th.ExpectError(t, err, "err200")
				th.ExpectMap(t, out, map[string]int{})

				if iterationsMap.Load() > 250 {
					t.Errorf("early exit did not happen")
				}
				if iterationsReduce.Load() > 250 {
					t.Errorf("early exit did not happen")
				}
			})

			th.RunSynctest(t, th.Name("error in mapper", nm, nr), func(t *testing.T) {
				in := FromChan(th.FromRange(0, 1000), nil)

				var iterationsMap, iterationsReduce atomic.Int64
				out, err := MapReduce(in,
					nm, func(x int) (string, int, error) {
						th.SimulateWork(1*time.Second, 2*time.Second)
						if iterationsMap.Add(1) == 200 {
							return "", 0, fmt.Errorf("err200")
						}
						return fmt.Sprintf("%d-digit", len(fmt.Sprint(x))), x, nil
					},
					nr, func(x, y int) (int, error) {
						th.SimulateWork(10*time.Second, 20*time.Second)

						iterationsReduce.Add(1)
						return x + y, nil
					},
				)

				th.WaitForInflightWork()
				th.ExpectDrainedChan(t, in)

				th.ExpectError(t, err, "err200")
				th.ExpectMap(t, out, map[string]int{})

				if iterationsMap.Load() > 250 {
					t.Errorf("early exit did not happen")
				}
				if iterationsReduce.Load() > 250 {
					t.Errorf("early exit did not happen")
				}
			})

			th.RunSynctest(t, th.Name("error in reducer", nm, nr), func(t *testing.T) {
				in := FromChan(th.FromRange(0, 1000), nil)

				var iterationsMap, iterationsReduce atomic.Int64
				out, err := MapReduce(in,
					nm, func(x int) (string, int, error) {
						th.SimulateWork(1*time.Second, 2*time.Second)
						iterationsMap.Add(1)
						return fmt.Sprintf("%d-digit", len(fmt.Sprint(x))), x, nil
					},
					nr, func(x, y int) (int, error) {
						th.SimulateWork(10*time.Second, 20*time.Second)
						if iterationsReduce.Add(1) == 200 {
							return 0, fmt.Errorf("err200")
						}
						return x + y, nil
					},
				)

				th.WaitForInflightWork()
				th.ExpectDrainedChan(t, in)

				th.ExpectError(t, err, "err200")
				th.ExpectMap(t, out, map[string]int{})

				if iterationsMap.Load() > 250 {
					t.Errorf("early exit did not happen")
				}
				if iterationsReduce.Load() > 250 {
					t.Errorf("early exit did not happen")
				}
			})

		}
	}
}
