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
		t.Run(th.Name("empty", n), func(t *testing.T) {
			in := FromSlice([]int{}, nil)

			_, ok, err := Reduce(in, n, func(x, y int) (int, error) {

				return x + y, nil
			})

			th.ExpectNoError(t, err)
			th.ExpectValue(t, ok, false)
			th.ExpectDrainedChan(t, in)
		})

		t.Run(th.Name("no errors", n), func(t *testing.T) {
			in := FromChan(th.FromRange(0, 100), nil)

			var cnt atomic.Int64
			out, ok, err := Reduce(in, n, func(x, y int) (int, error) {
				cnt.Add(1)
				return x + y, nil
			})

			th.ExpectNoError(t, err)
			th.ExpectValue(t, out, 99*100/2)
			th.ExpectValue(t, ok, true)
			th.ExpectValue(t, cnt.Load(), 99)
			th.ExpectDrainedChan(t, in)
		})

		t.Run(th.Name("error in input", n), func(t *testing.T) {
			in := FromChan(th.FromRange(0, 1000), nil)
			in = replaceWithError(in, 100, fmt.Errorf("err100"))

			var cnt atomic.Int64
			_, _, err := Reduce(in, n, func(x, y int) (int, error) {
				cnt.Add(1)
				return x + y, nil
			})

			th.ExpectError(t, err, "err100")
			if cnt.Load() > 900 {
				t.Errorf("early exit did not happen")
			}

			time.Sleep(1 * time.Second)

			th.ExpectDrainedChan(t, in)
			if cnt.Load() > 900 {
				t.Errorf("extra calls to f were made")
			}
		})

		t.Run(th.Name("error in func", n), func(t *testing.T) {
			in := FromChan(th.FromRange(0, 1000), nil)

			var cnt atomic.Int64
			_, _, err := Reduce(in, n, func(x, y int) (int, error) {
				if cnt.Add(1) == 100 {
					return 0, fmt.Errorf("err100")
				}

				return x + y, nil
			})

			th.ExpectError(t, err, "err100")
			if cnt.Load() > 900 {
				t.Errorf("early exit did not happen")
			}

			time.Sleep(1 * time.Second)

			th.ExpectDrainedChan(t, in)
			if cnt.Load() > 900 {
				t.Errorf("extra calls to f were made")
			}
		})
	}
}

func TestMapReduce(t *testing.T) {
	for _, nm := range []int{1, 4} {
		for _, nr := range []int{1, 4} {
			t.Run(th.Name("empty", nm, nr), func(t *testing.T) {
				in := FromSlice([]int{}, nil)

				out, err := MapReduce(in,
					nm, func(x int) (string, int, error) {
						s := fmt.Sprint(x)
						return fmt.Sprintf("%d-digit", len(s)), x, nil
					},
					nr, func(x, y int) (int, error) {
						return x + y, nil
					})

				th.ExpectNoError(t, err)
				th.ExpectMap(t, out, map[string]int{})
				th.ExpectDrainedChan(t, in)
			})

			t.Run(th.Name("no errors", nm, nr), func(t *testing.T) {
				in := FromChan(th.FromRange(0, 1000), nil)

				var cntMap, cntReduce int64
				out, err := MapReduce(in,
					nm, func(x int) (string, int, error) {
						atomic.AddInt64(&cntMap, 1)
						s := fmt.Sprint(x)
						return fmt.Sprintf("%d-digit", len(s)), x, nil
					},
					nr, func(x, y int) (int, error) {
						atomic.AddInt64(&cntReduce, 1)
						return x + y, nil
					},
				)

				th.ExpectNoError(t, err)
				th.ExpectMap(t, out, map[string]int{
					"1-digit": (0 + 9) * 10 / 2,
					"2-digit": (10 + 99) * 90 / 2,
					"3-digit": (100 + 999) * 900 / 2,
				})
				th.ExpectValue(t, cntMap, 1000)
				th.ExpectValue(t, cntReduce, 9+89+899)
				th.ExpectDrainedChan(t, in)
			})

			t.Run(th.Name("error in input", nm, nr), func(t *testing.T) {
				in := FromChan(th.FromRange(0, 1000), nil)
				in = replaceWithError(in, 100, fmt.Errorf("err100"))

				var cntMap, cntReduce atomic.Int64
				_, err := MapReduce(in,
					nm, func(x int) (string, int, error) {
						cntMap.Add(1)
						s := fmt.Sprint(x)
						return fmt.Sprintf("%d-digit", len(s)), x, nil
					},
					nr, func(x, y int) (int, error) {
						cntReduce.Add(1)
						return x + y, nil
					},
				)

				th.ExpectError(t, err, "err100")
				if cntMap.Load() > 900 {
					t.Errorf("early exit did not happen")
				}
				if cntReduce.Load() > 900 {
					t.Errorf("early exit did not happen")
				}

				time.Sleep(1 * time.Second)

				th.ExpectDrainedChan(t, in)
				if cntMap.Load() > 900 {
					t.Errorf("extra calls to f were made")
				}
				if cntReduce.Load() > 900 {
					t.Errorf("extra calls to f were made")
				}
			})

			t.Run(th.Name("error in mapper", nm, nr), func(t *testing.T) {
				in := FromChan(th.FromRange(0, 1000), nil)

				var cntMap, cntReduce atomic.Int64
				_, err := MapReduce(in,
					nm, func(x int) (string, int, error) {
						if cntMap.Add(1) == 100 {
							return "", 0, fmt.Errorf("err100")
						}
						s := fmt.Sprint(x)
						return fmt.Sprintf("%d-digit", len(s)), x, nil
					},
					nr, func(x, y int) (int, error) {
						cntReduce.Add(1)
						return x + y, nil
					},
				)

				th.ExpectError(t, err, "err100")
				if cntMap.Load() > 900 {
					t.Errorf("early exit did not happen")
				}
				if cntReduce.Load() > 900 {
					t.Errorf("early exit did not happen")
				}

				time.Sleep(1 * time.Second)

				th.ExpectDrainedChan(t, in)
				if cntMap.Load() > 900 {
					t.Errorf("extra calls to f were made")
				}
				if cntReduce.Load() > 900 {
					t.Errorf("extra calls to f were made")
				}
			})

			t.Run(th.Name("error in reducer", nm, nr), func(t *testing.T) {
				in := FromChan(th.FromRange(0, 1000), nil)

				var cntMap, cntReduce atomic.Int64
				_, err := MapReduce(in,
					nm, func(x int) (string, int, error) {
						cntMap.Add(1)
						s := fmt.Sprint(x)
						return fmt.Sprintf("%d-digit", len(s)), x, nil
					},
					nr, func(x, y int) (int, error) {
						if cntReduce.Add(1) == 100 {
							return 0, fmt.Errorf("err100")
						}
						return x + y, nil
					},
				)

				th.ExpectError(t, err, "err100")
				if cntMap.Load() > 900 {
					t.Errorf("early exit did not happen")
				}
				if cntReduce.Load() > 900 {
					t.Errorf("early exit did not happen")
				}

				time.Sleep(1 * time.Second)

				th.ExpectDrainedChan(t, in)
				if cntMap.Load() > 900 {
					t.Errorf("extra calls to f were made")
				}
				if cntReduce.Load() > 900 {
					t.Errorf("extra calls to f were made")
				}
			})

		}
	}
}
