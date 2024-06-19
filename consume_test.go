package rill

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/destel/rill/internal/th"
)

func TestErr(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		in := FromChan(th.FromSlice([]int{}), nil)
		err := Err(in)

		th.ExpectNoError(t, err)
	})

	t.Run("no errors", func(t *testing.T) {
		in := FromChan(th.FromRange(0, 100), nil)
		err := Err(in)

		th.ExpectNoError(t, err)
	})

	t.Run("error", func(t *testing.T) {
		in := FromChan(th.FromRange(0, 1000), nil)
		in = replaceWithError(in, 100, fmt.Errorf("err100"))

		err := Err(in)
		th.ExpectError(t, err, "err100")

		// wait until it drained
		time.Sleep(1 * time.Second)
		th.ExpectDrainedChan(t, in)
	})
}

func TestFirst(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		in := FromChan(th.FromSlice([]int{}), nil)
		_, ok, err := First(in)

		th.ExpectNoError(t, err)
		th.ExpectValue(t, ok, false)
	})

	t.Run("value is first", func(t *testing.T) {
		in := FromChan(th.FromRange(1, 1000), nil)
		in = replaceWithError(in, 100, fmt.Errorf("err100"))
		x, ok, err := First(in)

		th.ExpectNoError(t, err)
		th.ExpectValue(t, ok, true)
		th.ExpectValue(t, x, 1)

		// wait until it drained
		time.Sleep(1 * time.Second)
		th.ExpectDrainedChan(t, in)
	})

	t.Run("error is first", func(t *testing.T) {
		in := FromChan(th.FromRange(1, 1000), nil)
		in = replaceWithError(in, 1, fmt.Errorf("err1"))
		_, _, err := First(in)

		th.ExpectError(t, err, "err1")

		// wait until it drained
		time.Sleep(1 * time.Second)
		th.ExpectDrainedChan(t, in)
	})
}

func TestForEach(t *testing.T) {
	for _, n := range []int{1, 5} {

		t.Run(th.Name("no errors", n), func(t *testing.T) {
			in := FromChan(th.FromRange(0, 10), nil)

			sum := int64(0)
			err := ForEach(in, n, func(x int) error {
				atomic.AddInt64(&sum, int64(x))
				return nil
			})

			th.ExpectNoError(t, err)
			th.ExpectValue(t, sum, int64(9*10/2))
		})

		t.Run(th.Name("error in input", n), func(t *testing.T) {
			th.ExpectNotHang(t, 10*time.Second, func() {
				in := FromChan(th.FromRange(0, 1000), nil)
				in = replaceWithError(in, 100, fmt.Errorf("err100"))

				cnt := int64(0)
				err := ForEach(in, n, func(x int) error {
					atomic.AddInt64(&cnt, 1)
					return nil
				})

				th.ExpectError(t, err, "err100")
				if cnt < 100 {
					t.Errorf("expected at least 100 iterations to complete")
				}
				if cnt > 150 {
					t.Errorf("early exit did not happen")
				}

				time.Sleep(1 * time.Second)
				th.ExpectDrainedChan(t, in)
			})
		})

		t.Run(th.Name("error in func", n), func(t *testing.T) {
			th.ExpectNotHang(t, 10*time.Second, func() {
				in := FromChan(th.FromRange(0, 1000), nil)

				cnt := int64(0)
				err := ForEach(in, n, func(x int) error {
					if x == 100 {
						return fmt.Errorf("err100")
					}
					atomic.AddInt64(&cnt, 1)
					return nil
				})

				th.ExpectError(t, err, "err100")
				if cnt < 100 {
					t.Errorf("expected at least 100 iterations to complete")
				}
				if cnt == 1000 {
					t.Errorf("early exit did not happen")
				}

				// wait until it drained
				time.Sleep(1 * time.Second)
				th.ExpectDrainedChan(t, in)
			})
		})

		t.Run(th.Name("ordering", n), func(t *testing.T) {
			in := FromChan(th.FromRange(0, 20000), nil)

			var mu sync.Mutex
			outSlice := make([]int, 0, 20000)

			err := ForEach(in, n, func(x int) error {
				mu.Lock()
				outSlice = append(outSlice, x)
				mu.Unlock()
				return nil
			})

			th.ExpectNoError(t, err)
			if n == 1 {
				th.ExpectSorted(t, outSlice)
			} else {
				th.ExpectUnsorted(t, outSlice)
			}
		})

	}

	t.Run("deterministic when n=1", func(t *testing.T) {
		in := FromChan(th.FromRange(0, 100), nil)

		in = replaceWithError(in, 10, fmt.Errorf("err10"))
		in = replaceWithError(in, 11, fmt.Errorf("err11"))
		in = replaceWithError(in, 12, fmt.Errorf("err12"))

		maxX := -1

		err := ForEach(in, 1, func(x int) error {
			if x > maxX {
				maxX = x
			}
			return nil
		})

		th.ExpectValue(t, maxX, 9)
		th.ExpectError(t, err, "err10")
	})
}

func TestAnyAll(t *testing.T) {
	for _, n := range []int{1} {
		t.Run(th.Name("empty", n), func(t *testing.T) {
			in := FromSlice([]int{}, nil)

			res, err := All(in, 1, func(int) (bool, error) {
				return false, nil
			})

			th.ExpectNoError(t, err)
			th.ExpectValue(t, res, true)
		})

		t.Run(th.Name("no errors,false", n), func(t *testing.T) {
			in := FromChan(th.FromRange(0, 1000), nil)

			var cnt int64
			ok, err := All(in, n, func(x int) (bool, error) {
				atomic.AddInt64(&cnt, 1)
				return x < 100, nil
			})

			th.ExpectNoError(t, err)
			th.ExpectValue(t, ok, false)

			if cnt == 1000 {
				t.Errorf("early exit did not happen")
			}
		})

		t.Run(th.Name("error in input,false", n), func(t *testing.T) {
			in := FromChan(th.FromRange(0, 1000), nil)
			in = replaceWithError(in, 500, fmt.Errorf("err500"))

			var cnt int64
			ok, err := All(in, n, func(x int) (bool, error) {
				atomic.AddInt64(&cnt, 1)
				return x < 100, nil
			})

			th.ExpectNoError(t, err) // error was swallowed by early exit
			th.ExpectValue(t, ok, false)

			if cnt == 1000 {
				t.Errorf("early exit did not happen")
			}
		})

		t.Run(th.Name("error in func,false", n), func(t *testing.T) {
			in := FromChan(th.FromRange(0, 1000), nil)

			var cnt int64
			ok, err := All(in, n, func(x int) (bool, error) {
				atomic.AddInt64(&cnt, 1)
				if x == 500 {
					return false, fmt.Errorf("err500")
				}
				return x < 100, nil
			})

			th.ExpectNoError(t, err) // error was swallowed by early exit
			th.ExpectValue(t, ok, false)

			if cnt == 1000 {
				t.Errorf("early exit did not happen")
			}
		})

		//---

		t.Run(th.Name("no errors,true", n), func(t *testing.T) {
			in := FromChan(th.FromRange(0, 1000), nil)

			var cnt int64
			ok, err := All(in, n, func(x int) (bool, error) {
				atomic.AddInt64(&cnt, 1)
				return x < 1000, nil
			})

			th.ExpectNoError(t, err)
			th.ExpectValue(t, ok, true)
			th.ExpectValue(t, cnt, 1000)
		})

		t.Run(th.Name("error in input,true", n), func(t *testing.T) {
			in := FromChan(th.FromRange(0, 1000), nil)
			in = replaceWithError(in, 500, fmt.Errorf("err500"))

			var cnt int64
			_, err := All(in, n, func(x int) (bool, error) {
				atomic.AddInt64(&cnt, 1)
				return x < 1000, nil
			})

			th.ExpectError(t, err, "err500")

			if cnt == 1000 {
				t.Errorf("early exit did not happen")
			}
		})

		t.Run(th.Name("error in func,true", n), func(t *testing.T) {
			in := FromChan(th.FromRange(0, 1000), nil)

			var cnt int64
			_, err := All(in, n, func(x int) (bool, error) {
				atomic.AddInt64(&cnt, 1)
				if x == 500 {
					return false, fmt.Errorf("err500")
				}
				return x < 1000, nil
			})

			th.ExpectError(t, err, "err500")

			if cnt == 1000 {
				t.Errorf("early exit did not happen")
			}
		})

	}

}
