package rill

import (
	"fmt"
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

			var sum atomic.Int64
			err := ForEach(in, n, func(x int) error {
				sum.Add(int64(x))
				return nil
			})

			th.ExpectNoError(t, err)
			th.ExpectValue(t, sum.Load(), int64(9*10/2))
		})

		t.Run(th.Name("error in input", n), func(t *testing.T) {
			th.ExpectNotHang(t, 10*time.Second, func() {
				in := FromChan(th.FromRange(0, 1000), nil)
				in = replaceWithError(in, 100, fmt.Errorf("err100"))

				var cnt atomic.Int64
				err := ForEach(in, n, func(x int) error {
					cnt.Add(1)
					return nil
				})

				th.ExpectError(t, err, "err100")
				if cnt.Load() > 900 {
					t.Errorf("early return did not happen")
				}

				time.Sleep(1 * time.Second)

				th.ExpectDrainedChan(t, in)
				if cnt.Load() > 900 {
					t.Errorf("extra calls to f were made")
				}
			})
		})

		t.Run(th.Name("error in func", n), func(t *testing.T) {
			th.ExpectNotHang(t, 10*time.Second, func() {
				in := FromChan(th.FromRange(0, 1000), nil)

				var cnt atomic.Int64
				err := ForEach(in, n, func(x int) error {
					if x == 100 {
						return fmt.Errorf("err100")
					}
					cnt.Add(1)
					return nil
				})

				th.ExpectError(t, err, "err100")
				if cnt.Load() > 900 {
					t.Errorf("early return did not happen")
				}

				// wait until it drained
				time.Sleep(1 * time.Second)

				th.ExpectDrainedChan(t, in)
				if cnt.Load() > 900 {
					t.Errorf("extra calls to f were made")
				}
			})
		})
	}
}

func TestAnyAll(t *testing.T) {
	for _, n := range []int{1, 5} {
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

			var cnt atomic.Int64
			ok, err := All(in, n, func(x int) (bool, error) {
				cnt.Add(1)
				return x < 100, nil
			})

			th.ExpectNoError(t, err)
			th.ExpectValue(t, ok, false)
			if cnt.Load() > 900 {
				t.Errorf("early exit did not happen")
			}

			// wait until it drained
			time.Sleep(1 * time.Second)

			th.ExpectDrainedChan(t, in)
			if cnt.Load() > 900 {
				t.Errorf("extra calls to f were made")
			}
		})

		t.Run(th.Name("error in input,false", n), func(t *testing.T) {
			in := FromChan(th.FromRange(0, 1000), nil)
			in = replaceWithError(in, 500, fmt.Errorf("err500"))

			var cnt atomic.Int64
			ok, err := All(in, n, func(x int) (bool, error) {
				cnt.Add(1)
				return x < 100, nil
			})

			th.ExpectNoError(t, err) // error was swallowed by early exit
			th.ExpectValue(t, ok, false)
			if cnt.Load() > 900 {
				t.Errorf("early exit did not happen")
			}

			// wait until it drained
			time.Sleep(1 * time.Second)

			th.ExpectDrainedChan(t, in)
			if cnt.Load() > 900 {
				t.Errorf("extra calls to f were made")
			}
		})

		t.Run(th.Name("error in func,false", n), func(t *testing.T) {
			in := FromChan(th.FromRange(0, 1000), nil)

			var cnt atomic.Int64
			ok, err := All(in, n, func(x int) (bool, error) {
				cnt.Add(1)
				if x == 500 {
					return false, fmt.Errorf("err500")
				}
				return x < 100, nil
			})

			th.ExpectNoError(t, err) // error was swallowed by early exit
			th.ExpectValue(t, ok, false)
			if cnt.Load() > 900 {
				t.Errorf("early exit did not happen")
			}

			// wait until it drained
			time.Sleep(1 * time.Second)

			th.ExpectDrainedChan(t, in)
			if cnt.Load() > 900 {
				t.Errorf("extra calls to f were made")
			}
		})

		//---

		t.Run(th.Name("no errors,true", n), func(t *testing.T) {
			in := FromChan(th.FromRange(0, 1000), nil)

			var cnt atomic.Int64
			ok, err := All(in, n, func(x int) (bool, error) {
				cnt.Add(1)
				return x < 10000, nil
			})

			th.ExpectNoError(t, err)
			th.ExpectValue(t, ok, true)

			th.ExpectDrainedChan(t, in)
		})

		t.Run(th.Name("error in input,true", n), func(t *testing.T) {
			in := FromChan(th.FromRange(0, 1000), nil)
			in = replaceWithError(in, 500, fmt.Errorf("err500"))

			var cnt atomic.Int64
			_, err := All(in, n, func(x int) (bool, error) {
				cnt.Add(1)
				return x < 10000, nil
			})

			th.ExpectError(t, err, "err500")
			if cnt.Load() > 900 {
				t.Errorf("early exit did not happen")
			}

			// wait until it drained
			time.Sleep(1 * time.Second)

			th.ExpectDrainedChan(t, in)
			if cnt.Load() > 900 {
				t.Errorf("extra calls to f were made")
			}
		})

		t.Run(th.Name("error in func,true", n), func(t *testing.T) {
			in := FromChan(th.FromRange(0, 1000), nil)

			var cnt atomic.Int64
			_, err := All(in, n, func(x int) (bool, error) {
				cnt.Add(1)
				if x == 500 {
					return false, fmt.Errorf("err500")
				}
				return x < 10000, nil
			})

			th.ExpectError(t, err, "err500")
			if cnt.Load() > 900 {
				t.Errorf("early exit did not happen")
			}

			// wait until it drained
			time.Sleep(1 * time.Second)

			th.ExpectDrainedChan(t, in)
			if cnt.Load() > 900 {
				t.Errorf("extra calls to f were made")
			}
		})

	}

}
