package rill

import (
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/destel/rill/internal/th"
)

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
