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
		})

		t.Run(th.Name("no errors", n), func(t *testing.T) {
			in := FromChan(th.FromRange(0, 100), nil)

			cnt := int64(0)
			out, ok, err := Reduce(in, n, func(x, y int) (int, error) {
				atomic.AddInt64(&cnt, 1)
				return x + y, nil
			})

			th.ExpectNoError(t, err)
			th.ExpectValue(t, out, 99*100/2)
			th.ExpectValue(t, ok, true)
			th.ExpectValue(t, cnt, 99)
		})

		t.Run(th.Name("error in input", n), func(t *testing.T) {
			in := FromChan(th.FromRange(0, 1000), nil)
			in = replaceWithError(in, 100, fmt.Errorf("err100"))

			cnt := int64(0)
			_, _, err := Reduce(in, n, func(x, y int) (int, error) {
				atomic.AddInt64(&cnt, 1)
				return x + y, nil
			})

			th.ExpectError(t, err, "err100")
			if cnt == 999 {
				t.Errorf("early exit did not happen")
			}

			time.Sleep(1 * time.Second)
			th.ExpectDrainedChan(t, in)
		})

		t.Run(th.Name("error in func", n), func(t *testing.T) {
			in := FromChan(th.FromRange(0, 1000), nil)

			cnt := int64(0)
			_, _, err := Reduce(in, n, func(x, y int) (int, error) {
				if atomic.AddInt64(&cnt, 1) == 100 {
					return 0, fmt.Errorf("err100")
				}

				return x + y, nil
			})

			th.ExpectError(t, err, "err100")
			if cnt == 999 {
				t.Errorf("early exit did not happen")
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
			})

			t.Run(th.Name("error in input", nm, nr), func(t *testing.T) {
				in := FromChan(th.FromRange(0, 1000), nil)
				in = replaceWithError(in, 100, fmt.Errorf("err100"))

				var cntMap, cntReduce int64
				_, err := MapReduce(in,
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

				th.ExpectError(t, err, "err100")
				if cntMap == 1000 {
					t.Errorf("early exit did not happen")
				}
				if cntReduce == 9+89+899 {
					t.Errorf("early exit did not happen")
				}
			})

			t.Run(th.Name("error in mapper", nm, nr), func(t *testing.T) {
				in := FromChan(th.FromRange(0, 1000), nil)

				var cntMap, cntReduce int64
				_, err := MapReduce(in,
					nm, func(x int) (string, int, error) {
						if atomic.AddInt64(&cntMap, 1) == 100 {
							return "", 0, fmt.Errorf("err100")
						}
						s := fmt.Sprint(x)
						return fmt.Sprintf("%d-digit", len(s)), x, nil
					},
					nr, func(x, y int) (int, error) {
						atomic.AddInt64(&cntReduce, 1)
						return x + y, nil
					},
				)

				th.ExpectError(t, err, "err100")
				if cntMap == 1000 {
					t.Errorf("early exit did not happen")
				}
				if cntReduce == 9+89+899 {
					t.Errorf("early exit did not happen")
				}
			})

			t.Run(th.Name("error in reducer", nm, nr), func(t *testing.T) {
				in := FromChan(th.FromRange(0, 1000), nil)

				var cntMap, cntReduce int64
				_, err := MapReduce(in,
					nm, func(x int) (string, int, error) {
						atomic.AddInt64(&cntMap, 1)
						s := fmt.Sprint(x)
						return fmt.Sprintf("%d-digit", len(s)), x, nil
					},
					nr, func(x, y int) (int, error) {
						if atomic.AddInt64(&cntReduce, 1) == 100 {
							return 0, fmt.Errorf("err100")
						}
						return x + y, nil
					},
				)

				th.ExpectError(t, err, "err100")
				if cntMap == 1000 {
					t.Errorf("early exit did not happen")
				}
				if cntReduce == 9+89+899 {
					t.Errorf("early exit did not happen")
				}
			})

		}
	}
}
