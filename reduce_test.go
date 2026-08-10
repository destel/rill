package rill

import (
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/destel/rill/internal/th"
)

func TestReduce(t *testing.T) {
	th.TestLevels(t, []int{1, 4, 7}, func(t *testing.T, n int) {

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

		th.RunSynctest(t, "concurrency", func(t *testing.T) {
			in := FromChan(th.FromRange(0, 100), nil)

			var gauge th.InFlightGauge

			_, _, _ = Reduce(in, n, func(x, y int) (int, error) {
				gauge.Enter()
				defer gauge.Exit()
				th.SimulateWork(1*time.Second, 2*time.Second)

				return x + y, nil
			})

			th.ExpectValue(t, gauge.Max(), n)
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

		th.RunSynctest(t, "error in func (last)", func(t *testing.T) {
			in := FromChan(th.FromRange(0, 1000), nil)

			var i atomic.Int64
			out, ok, err := Reduce(in, n, func(x, y int) (int, error) {
				th.SimulateWork(1*time.Second, 2*time.Second)

				if i.Add(1) == 999 {
					return 0, fmt.Errorf("err999")
				}
				return x + y, nil
			})

			th.ExpectError(t, err, "err999")
			th.ExpectValue(t, out, 0)
			th.ExpectValue(t, ok, false)

			th.ExpectDrainedChan(t, in)
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

		th.RunSynctest(t, "ordering", func(t *testing.T) {
			tokens := make([]string, 100)
			for i := range tokens {
				tokens[i] = fmt.Sprintf("%d", i)
			}

			slowTokens := []string{"20", "40", "60", "80"}

			in := FromSlice(tokens, nil)

			out, ok, err := Reduce(in, n, func(x, y string) (string, error) {
				th.SimulateWork(1*time.Second, 2*time.Second)

				// Force out-of-order completion: a few leaves are expensive enough
				// that while a worker sleeps on one, the others finish all or most
				// of the remaining work first (at n=4 all four workers can be asleep
				// at once), so the late partials must re-attach in position.
				if slices.Contains(slowTokens, x) || slices.Contains(slowTokens, y) {
					th.SimulateWork(100*time.Second, 200*time.Second)
				}

				return x + "|" + y, nil
			})

			expected := strings.Join(tokens, "|")

			th.ExpectDrainedChan(t, in)

			th.ExpectNoError(t, err)
			th.ExpectValue(t, out, expected)
			th.ExpectValue(t, ok, true)
		})

	})

}

func TestMapReduce(t *testing.T) {
	th.TestVariants(t, "nm", []int{1, 4}, func(t *testing.T, nm int) {
		th.TestVariants(t, "nr", []int{1, 4, 7}, func(t *testing.T, nr int) {

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
			})

			th.RunSynctest(t, "single error stream", func(t *testing.T) {
				in := FromSlice([]int{}, fmt.Errorf("err0"))

				out, err := MapReduce(in,
					nm, func(x int) (string, int, error) {
						th.SimulateWork(1*time.Second, 2*time.Second)
						return fmt.Sprint(x), x, nil
					},
					nr, func(x, y int) (int, error) {
						th.SimulateWork(1*time.Second, 2*time.Second)
						return x + y, nil
					},
				)

				th.WaitForInflightWork()
				th.ExpectDrainedChan(t, in)

				th.ExpectError(t, err, "err0")
				th.ExpectMap(t, out, nil)
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

			th.RunSynctest(t, "concurrency", func(t *testing.T) {
				in := FromChan(th.FromRange(0, 100), nil)

				// To reach max concurrency in the reduce phase, the map phase must outpace it
				// rather than become the bottleneck. Under synctest we get that by giving
				// mappers a much smaller work than reducers.
				var mapGauge th.InFlightGauge
				var reduceGauge th.InFlightGauge

				_, _ = MapReduce(in,
					nm, func(x int) (string, int, error) {
						mapGauge.Enter()
						defer mapGauge.Exit()
						th.SimulateWork(1*time.Second, 2*time.Second)

						return fmt.Sprintf("%d mod 3", x%3), 1, nil
					},
					nr, func(x, y int) (int, error) {
						reduceGauge.Enter()
						defer reduceGauge.Exit()
						th.SimulateWork(10*time.Second, 20*time.Second)

						return x + y, nil
					},
				)

				th.ExpectValue(t, mapGauge.Max(), nm)
				th.ExpectValue(t, reduceGauge.Max(), nr)
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
				th.ExpectMap(t, out, nil)

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
				th.ExpectMap(t, out, nil)

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
				th.ExpectMap(t, out, nil)

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

			th.RunSynctest(t, "error in reducer (last)", func(t *testing.T) {
				in := FromChan(th.FromRange(0, 1000), nil)

				var i atomic.Int64
				out, err := MapReduce(in,
					nm, func(x int) (string, int, error) {
						th.SimulateWork(1*time.Second, 2*time.Second)
						return fmt.Sprintf("%d-digit", len(fmt.Sprint(x))), x, nil
					},
					nr, func(x, y int) (int, error) {
						th.SimulateWork(10*time.Second, 20*time.Second)
						if i.Add(1) == 9+89+899 {
							return 0, fmt.Errorf("err997")
						}
						return x + y, nil
					},
				)

				th.ExpectDrainedChan(t, in)

				th.ExpectError(t, err, "err997")
				th.ExpectMap(t, out, nil)
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
					th.ExpectMap(t, out, nil)
				})
			})

			th.RunSynctest(t, "ordering", func(t *testing.T) {
				const numKeys = 3
				slowTokens := []string{"20", "40", "60", "80"}

				in := FromChan(th.FromRange(0, 100), nil)

				out, err := MapReduce(in,
					nm, func(x int) (int, string, error) {
						th.SimulateWork(1*time.Second, 2*time.Second)
						return x % numKeys, fmt.Sprint(x), nil
					},
					nr, func(x, y string) (string, error) {
						th.SimulateWork(10*time.Second, 20*time.Second)

						// Force out-of-order completion: a few leaves are expensive
						// enough that while a worker sleeps on one, the others finish
						// most of the remaining work first, so the late partials must
						// re-attach in position.
						if slices.Contains(slowTokens, x) || slices.Contains(slowTokens, y) {
							th.SimulateWork(500*time.Second, 600*time.Second)
						}

						return x + "|" + y, nil
					},
				)

				expected := make(map[int]string, numKeys)
				for i := range 100 {
					k := i % numKeys
					if expected[k] == "" {
						expected[k] = fmt.Sprint(i)
					} else {
						expected[k] += "|" + fmt.Sprint(i)
					}
				}

				th.ExpectDrainedChan(t, in)

				th.ExpectNoError(t, err)
				th.ExpectMap(t, out, expected)
			})

		})
	})

}

func TestMergeMaps(t *testing.T) {
	concat := func(x, y string) (string, error) { return x + "|" + y, nil }

	failingConcat := func(x, y string) (string, error) {
		if x == "b" || y == "b" {
			return "", fmt.Errorf("boom")
		}
		return x + "|" + y, nil
	}

	sameMap := func(a, b map[int]string) bool {
		return reflect.ValueOf(a).Pointer() == reflect.ValueOf(b).Pointer()
	}

	t.Run("acc is larger", func(t *testing.T) {
		acc := map[int]string{1: "a", 2: "b", 3: "c"}
		m := map[int]string{2: "y", 4: "w"}
		mBefore := maps.Clone(m)

		res, leftover, err := mergeMaps(acc, m, concat)

		th.ExpectNoError(t, err)
		th.ExpectMap(t, res, map[int]string{1: "a", 2: "b|y", 3: "c", 4: "w"})
		th.ExpectMap(t, leftover, mBefore)

		// the larger map is the storage; the smaller one is a leftover
		th.ExpectValue(t, sameMap(res, acc), true)
		th.ExpectValue(t, sameMap(leftover, m), true)

	})

	t.Run("acc is larger with reducer error", func(t *testing.T) {
		acc := map[int]string{1: "a", 2: "b", 3: "c"}
		m := map[int]string{2: "y", 4: "w"}
		mBefore := maps.Clone(m)

		res, leftover, err := mergeMaps(acc, m, failingConcat)

		th.ExpectError(t, err, "boom")
		th.ExpectMap(t, leftover, mBefore)

		// an aborted merge leaves the storage roles and the leftover unaffected
		th.ExpectValue(t, sameMap(res, acc), true)
		th.ExpectValue(t, sameMap(leftover, m), true)

	})

	t.Run("acc is smaller", func(t *testing.T) {
		acc := map[int]string{2: "b", 4: "d"}
		m := map[int]string{1: "x", 2: "y", 3: "z"}
		accBefore := maps.Clone(acc)

		res, leftover, err := mergeMaps(acc, m, concat)

		th.ExpectNoError(t, err)
		th.ExpectMap(t, res, map[int]string{1: "x", 2: "b|y", 3: "z", 4: "d"})
		th.ExpectMap(t, leftover, accBefore)

		th.ExpectValue(t, sameMap(res, m), true)
		th.ExpectValue(t, sameMap(leftover, acc), true)

	})

	t.Run("acc is smaller with reducer error", func(t *testing.T) {
		acc := map[int]string{2: "b", 4: "d"}
		m := map[int]string{1: "x", 2: "y", 3: "z"}
		accBefore := maps.Clone(acc)

		res, leftover, err := mergeMaps(acc, m, failingConcat)

		th.ExpectError(t, err, "boom")
		th.ExpectMap(t, leftover, accBefore)

		th.ExpectValue(t, sameMap(res, m), true)
		th.ExpectValue(t, sameMap(leftover, acc), true)

	})
}
