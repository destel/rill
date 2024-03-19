package echans

import (
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/destel/rill/internal/th"
)

func universalMap[A, B any](ord bool, in <-chan Try[A], n int, f func(A) (B, error)) <-chan Try[B] {
	if ord {
		return OrderedMap(in, n, f)
	}
	return Map(in, n, f)
}

func TestMap(t *testing.T) {
	th.TestBothOrderings(t, func(t *testing.T, ord bool) {
		for _, n := range []int{1, 5} {

			t.Run(th.Name("nil", n), func(t *testing.T) {
				out := universalMap(ord, nil, n, func(x int) (int, error) { return x, nil })
				th.ExpectValue(t, out, nil)
			})

			t.Run(th.Name("correctness", n), func(t *testing.T) {
				in := Wrap(th.FromRange(0, 20), nil)
				in = replaceWithError(in, 15, fmt.Errorf("err15"))

				out := universalMap(ord, in, n, func(x int) (string, error) {
					if x == 5 {
						return "", fmt.Errorf("err05")
					}
					if x == 6 {
						return "", fmt.Errorf("err06")
					}

					return fmt.Sprintf("%03d", x), nil
				})

				outSlice, errSlice := toSliceAndErrors(out)

				expectedSlice := make([]string, 0, 20)
				for i := 0; i < 20; i++ {
					if i == 5 || i == 6 || i == 15 {
						continue
					}
					expectedSlice = append(expectedSlice, fmt.Sprintf("%03d", i))
				}

				sort.Strings(outSlice)
				sort.Strings(errSlice)

				th.ExpectSlice(t, outSlice, expectedSlice)
				th.ExpectSlice(t, errSlice, []string{"err05", "err06", "err15"})
			})

			t.Run(th.Name("ordering", n), func(t *testing.T) {
				in := Wrap(th.FromRange(0, 20000), nil)

				out := universalMap(ord, in, n, func(x int) (int, error) {
					if x%2 == 0 {
						return x, fmt.Errorf("err%06d", x)
					}

					return x, nil
				})

				outSlice, errSlice := toSliceAndErrors(out)

				if ord || n == 1 {
					th.ExpectSorted(t, outSlice)
					th.ExpectSorted(t, errSlice)
				} else {
					th.ExpectUnsorted(t, outSlice)
					th.ExpectUnsorted(t, errSlice)
				}

			})

		}
	})
}

func universalFilter(ord bool, in <-chan Try[int], n int, f func(int) (bool, error)) <-chan Try[int] {
	if ord {
		return OrderedFilter(in, n, f)
	}
	return Filter(in, n, f)
}

func TestFilter(t *testing.T) {
	th.TestBothOrderings(t, func(t *testing.T, ord bool) {
		for _, n := range []int{1, 5} {

			t.Run(th.Name("nil", n), func(t *testing.T) {
				out := universalFilter(ord, nil, n, func(x int) (bool, error) { return true, nil })
				th.ExpectValue(t, out, nil)
			})

			t.Run(th.Name("correctness", n), func(t *testing.T) {
				in := Wrap(th.FromRange(0, 20), nil)
				in = replaceWithError(in, 15, fmt.Errorf("err15"))

				out := universalFilter(ord, in, n, func(x int) (bool, error) {
					if x == 5 {
						return false, fmt.Errorf("err05")
					}
					if x == 6 {
						return true, fmt.Errorf("err06")
					}

					return x%2 == 0, nil
				})

				outSlice, errSlice := toSliceAndErrors(out)

				expectedSlice := make([]int, 0, 20)
				for i := 0; i < 20; i++ {
					if i%2 == 1 || i == 5 || i == 6 || i == 15 {
						continue
					}
					expectedSlice = append(expectedSlice, i)
				}

				th.Sort(outSlice)
				th.Sort(errSlice)

				th.ExpectSlice(t, outSlice, expectedSlice)
				th.ExpectSlice(t, errSlice, []string{"err05", "err06", "err15"})
			})

			t.Run(th.Name("ordering", n), func(t *testing.T) {
				in := Wrap(th.FromRange(0, 20000), nil)

				out := universalFilter(ord, in, n, func(x int) (bool, error) {
					switch x % 3 {
					case 2:
						return false, fmt.Errorf("err%06d", x)
					case 1:
						return false, nil
					default:
						return true, nil

					}
				})

				outSlice, errSlice := toSliceAndErrors(out)

				if ord || n == 1 {
					th.ExpectSorted(t, outSlice)
					th.ExpectSorted(t, errSlice)
				} else {
					th.ExpectUnsorted(t, outSlice)
					th.ExpectUnsorted(t, errSlice)
				}
			})

		}
	})
}

func universalFlatMap[A, B any](ord bool, in <-chan Try[A], n int, f func(A) <-chan Try[B]) <-chan Try[B] {
	if ord {
		return OrderedFlatMap(in, n, f)
	}
	return FlatMap(in, n, f)
}

func TestFlatMap(t *testing.T) {
	th.TestBothOrderings(t, func(t *testing.T, ord bool) {
		for _, n := range []int{1, 5} {

			t.Run(th.Name("nil", n), func(t *testing.T) {
				out := universalFlatMap(ord, nil, n, func(x int) <-chan Try[string] { return nil })
				th.ExpectValue(t, out, nil)
			})

			t.Run(th.Name("correctness", n), func(t *testing.T) {
				in := Wrap(th.FromRange(0, 20), nil)
				in = replaceWithError(in, 5, fmt.Errorf("err05"))
				in = replaceWithError(in, 15, fmt.Errorf("err15"))

				out := universalFlatMap(ord, in, n, func(x int) <-chan Try[string] {
					return FromSlice([]string{
						fmt.Sprintf("%03dA", x),
						fmt.Sprintf("%03dB", x),
					})
				})

				outSlice, errSlice := toSliceAndErrors(out)

				expectedSlice := make([]string, 0, 20*2)
				for i := 0; i < 20; i++ {
					if i == 5 || i == 15 {
						continue
					}
					expectedSlice = append(expectedSlice, fmt.Sprintf("%03dA", i), fmt.Sprintf("%03dB", i))
				}

				sort.Strings(outSlice)
				sort.Strings(errSlice)

				th.ExpectSlice(t, outSlice, expectedSlice)
				th.ExpectSlice(t, errSlice, []string{"err05", "err15"})
			})

			t.Run(th.Name("ordering", n), func(t *testing.T) {
				in := Wrap(th.FromRange(0, 20000), nil)
				in = OrderedMap(in, 1, func(x int) (int, error) {
					if x%2 == 0 {
						return x, fmt.Errorf("err%06d", x)
					}
					return x, nil
				})

				out := universalFlatMap(ord, in, n, func(x int) <-chan Try[string] {
					return FromSlice([]string{
						fmt.Sprintf("%06dA", x),
						fmt.Sprintf("%06dB", x),
					})
				})

				outSlice, errSlice := toSliceAndErrors(out)

				if ord || n == 1 {
					th.ExpectSorted(t, outSlice)
					th.ExpectSorted(t, errSlice)
				} else {
					th.ExpectUnsorted(t, outSlice)
					th.ExpectUnsorted(t, errSlice)
				}
			})

		}
	})
}

func universalCatch(ord bool, in <-chan Try[int], n int, f func(error) error) <-chan Try[int] {
	if ord {
		return OrderedCatch(in, n, f)
	}
	return Catch(in, n, f)
}

func TestCatch(t *testing.T) {
	th.TestBothOrderings(t, func(t *testing.T, ord bool) {
		for _, n := range []int{1, 5} {
			t.Run(th.Name("nil", n), func(t *testing.T) {
				out := universalCatch(ord, nil, n, func(err error) error { return nil })
				th.ExpectValue(t, out, nil)
			})

			t.Run(th.Name("correctness", n), func(t *testing.T) {
				in := Wrap(th.FromRange(0, 20), nil)
				in = replaceWithError(in, 5, fmt.Errorf("err05"))
				in = replaceWithError(in, 10, fmt.Errorf("err10"))
				in = replaceWithError(in, 15, fmt.Errorf("err15"))

				out := universalCatch(ord, in, n, func(err error) error {
					if err.Error() == "err05" {
						return nil // handled
					}
					if err.Error() == "err10" {
						return fmt.Errorf("%w wrapped", err) // wrapped/replaced
					}

					return err // leave as is
				})

				outSlice, errSlice := toSliceAndErrors(out)

				expectedSlice := make([]int, 0, 20)
				for i := 0; i < 20; i++ {
					if i == 5 || i == 10 || i == 15 {
						continue
					}
					expectedSlice = append(expectedSlice, i)
				}

				th.Sort(outSlice)
				th.Sort(errSlice)

				th.ExpectSlice(t, outSlice, expectedSlice)
				th.ExpectSlice(t, errSlice, []string{"err10 wrapped", "err15"})
			})

			t.Run(th.Name("ordering", n), func(t *testing.T) {
				in := Wrap(th.FromRange(0, 20000), nil)
				in = OrderedMap(in, 1, func(x int) (int, error) {
					if x%2 == 0 {
						return x, fmt.Errorf("err%06d", x)
					}
					return x, nil
				})

				out := universalCatch(ord, in, n, func(err error) error {
					return fmt.Errorf("%w wrapped", err)
				})

				outSlice, errSlice := toSliceAndErrors(out)

				if ord || n == 1 {
					th.ExpectSorted(t, outSlice)
					th.ExpectSorted(t, errSlice)
				} else {
					th.ExpectUnsorted(t, outSlice)
					th.ExpectUnsorted(t, errSlice)
				}
			})

		}
	})
}

func TestForEach(t *testing.T) {
	for _, n := range []int{1, 5} {

		t.Run(th.Name("no errors", n), func(t *testing.T) {
			in := Wrap(th.FromRange(0, 10), nil)

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
				in := Wrap(th.FromRange(0, 1000), nil)
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
				in := Wrap(th.FromRange(0, 1000), nil)

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
				if cnt > 150 {
					t.Errorf("early exit did not happen")
				}

				// wait until it drained
				time.Sleep(1 * time.Second)
				th.ExpectDrainedChan(t, in)
			})
		})

		t.Run(th.Name("ordering", n), func(t *testing.T) {
			in := Wrap(th.FromRange(0, 20000), nil)

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
		in := Wrap(th.FromRange(0, 100), nil)

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
