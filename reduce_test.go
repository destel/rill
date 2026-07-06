package rill

import (
	"fmt"
	"sync/atomic"
	"testing"
	"testing/synctest"
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
				return x + y, nil
			})

			synctest.Wait()
			th.ExpectDrainedChan(t, in)

			th.ExpectNoError(t, err)
			th.ExpectValue(t, ok, false)
		})

		th.RunSynctest(t, th.Name("single item, no error", n), func(t *testing.T) {
			in := FromSlice([]int{5}, nil)

			out, ok, err := Reduce(in, n, func(x, y int) (int, error) {
				return x + y, nil
			})

			synctest.Wait()
			th.ExpectDrainedChan(t, in)

			th.ExpectNoError(t, err)
			th.ExpectValue(t, out, 5)
			th.ExpectValue(t, ok, true)
		})

		th.RunSynctest(t, th.Name("single item, error", n), func(t *testing.T) {
			in := FromSlice([]int{}, fmt.Errorf("err0"))

			_, ok, err := Reduce(in, n, func(x, y int) (int, error) {
				return x + y, nil
			})

			synctest.Wait()
			th.ExpectDrainedChan(t, in)

			th.ExpectError(t, err, "err0")
			th.ExpectValue(t, ok, false)
		})

		th.RunSynctest(t, th.Name("no errors", n), func(t *testing.T) {
			in := FromChan(th.FromRange(0, 100), nil)

			out, ok, err := Reduce(in, n, func(x, y int) (int, error) {
				return x + y, nil
			})

			synctest.Wait()
			th.ExpectDrainedChan(t, in)

			th.ExpectNoError(t, err)
			th.ExpectValue(t, out, 99*100/2)
			th.ExpectValue(t, ok, true)
		})

		th.RunSynctest(t, th.Name("error in input", n), func(t *testing.T) {
			in := FromChan(th.FromRange(0, 1000), nil)
			in = replaceWithError(in, 200, fmt.Errorf("err200"))

			var cnt atomic.Int64
			_, _, err := Reduce(in, n, func(x, y int) (int, error) {
				// Balance per-item work so early exit stays observable
				// See comments in TestForEach for more details
				th.RandomSleep(1*time.Second, 2*time.Second)

				cnt.Add(1)
				return x + y, nil
			})

			time.Sleep(10 * time.Second)
			th.ExpectDrainedChan(t, in)

			th.ExpectError(t, err, "err200")
			if cnt.Load() > 250 {
				t.Errorf("early exit did not happen")
			}
		})

		th.RunSynctest(t, th.Name("error in func", n), func(t *testing.T) {
			in := FromChan(th.FromRange(0, 1000), nil)

			var cnt atomic.Int64
			_, _, err := Reduce(in, n, func(x, y int) (int, error) {
				// Balance per-item work so early exit stays observable
				// See comments in TestForEach for more details
				th.RandomSleep(1*time.Second, 2*time.Second)

				if cnt.Add(1) == 200 {
					return 0, fmt.Errorf("err200")
				}

				return x + y, nil
			})

			time.Sleep(10 * time.Second)
			th.ExpectDrainedChan(t, in)

			th.ExpectError(t, err, "err200")
			if cnt.Load() > 250 {
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
						s := fmt.Sprint(x)
						return fmt.Sprintf("%d-digit", len(s)), x, nil
					},
					nr, func(x, y int) (int, error) {
						return x + y, nil
					})

				synctest.Wait()
				th.ExpectDrainedChan(t, in)

				th.ExpectNoError(t, err)
				th.ExpectMap(t, out, map[string]int{})
			})

			th.RunSynctest(t, th.Name("no errors", nm, nr), func(t *testing.T) {
				in := FromChan(th.FromRange(0, 1000), nil)

				out, err := MapReduce(in,
					nm, func(x int) (string, int, error) {
						s := fmt.Sprint(x)
						return fmt.Sprintf("%d-digit", len(s)), x, nil
					},
					nr, func(x, y int) (int, error) {
						return x + y, nil
					},
				)

				synctest.Wait()
				th.ExpectDrainedChan(t, in)

				th.ExpectNoError(t, err)
				th.ExpectMap(t, out, map[string]int{
					"1-digit": (0 + 9) * 10 / 2,
					"2-digit": (10 + 99) * 90 / 2,
					"3-digit": (100 + 999) * 900 / 2,
				})
			})

			th.RunSynctest(t, th.Name("error in input", nm, nr), func(t *testing.T) {
				in := FromChan(th.FromRange(0, 1000), nil)
				in = replaceWithError(in, 200, fmt.Errorf("err200"))

				var cntMap, cntReduce atomic.Int64
				out, err := MapReduce(in,
					nm, func(x int) (string, int, error) {
						// Balance per-item work so early exit stays observable
						// See comments in TestForEach for more details
						th.RandomSleep(1*time.Second, 2*time.Second)

						cntMap.Add(1)
						s := fmt.Sprint(x)
						return fmt.Sprintf("%d-digit", len(s)), x, nil
					},
					nr, func(x, y int) (int, error) {
						// Reducer must be slower than mapper to achieve full concurrency.
						th.RandomSleep(10*time.Second, 20*time.Second)

						cntReduce.Add(1)
						return x + y, nil
					},
				)

				time.Sleep(30 * time.Second)
				th.ExpectDrainedChan(t, in)

				th.ExpectError(t, err, "err200")
				th.ExpectMap(t, out, map[string]int{})

				if cntMap.Load() > 250 {
					t.Errorf("early exit did not happen")
				}
				if cntReduce.Load() > 250 {
					t.Errorf("early exit did not happen")
				}
			})

			th.RunSynctest(t, th.Name("error in mapper", nm, nr), func(t *testing.T) {
				in := FromChan(th.FromRange(0, 1000), nil)

				var cntMap, cntReduce atomic.Int64
				out, err := MapReduce(in,
					nm, func(x int) (string, int, error) {
						// Balance per-item work so early exit stays observable
						// See comments in TestForEach for more details
						th.RandomSleep(1*time.Second, 2*time.Second)

						if cntMap.Add(1) == 200 {
							return "", 0, fmt.Errorf("err200")
						}
						s := fmt.Sprint(x)
						return fmt.Sprintf("%d-digit", len(s)), x, nil
					},
					nr, func(x, y int) (int, error) {
						// Reducer must be slower than mapper to achieve full concurrency.
						th.RandomSleep(10*time.Second, 20*time.Second)

						cntReduce.Add(1)
						return x + y, nil
					},
				)

				time.Sleep(30 * time.Second)
				th.ExpectDrainedChan(t, in)

				th.ExpectError(t, err, "err200")
				th.ExpectMap(t, out, map[string]int{})

				if cntMap.Load() > 250 {
					t.Errorf("early exit did not happen")
				}
				if cntReduce.Load() > 250 {
					t.Errorf("early exit did not happen")
				}
			})

			th.RunSynctest(t, th.Name("error in reducer", nm, nr), func(t *testing.T) {
				in := FromChan(th.FromRange(0, 1000), nil)

				var cntMap, cntReduce atomic.Int64
				out, err := MapReduce(in,
					nm, func(x int) (string, int, error) {
						// Balance per-item work so early exit stays observable
						// See comments in TestForEach for more details
						th.RandomSleep(1*time.Second, 2*time.Second)

						cntMap.Add(1)
						s := fmt.Sprint(x)
						return fmt.Sprintf("%d-digit", len(s)), x, nil
					},
					nr, func(x, y int) (int, error) {
						// Reducer must be slower than mapper to achieve full concurrency.
						th.RandomSleep(10*time.Second, 20*time.Second)

						if cntReduce.Add(1) == 200 {
							return 0, fmt.Errorf("err200")
						}
						return x + y, nil
					},
				)

				time.Sleep(30 * time.Second)
				th.ExpectDrainedChan(t, in)

				th.ExpectError(t, err, "err200")
				th.ExpectMap(t, out, map[string]int{})

				if cntMap.Load() > 250 {
					t.Errorf("early exit did not happen")
				}
				if cntReduce.Load() > 250 {
					t.Errorf("early exit did not happen")
				}
			})

		}
	}
}
