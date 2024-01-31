package echans

import (
	"fmt"
	"sort"
	"sync/atomic"
	"testing"
	"time"

	"github.com/destel/rill/internal/th"
)

func testname(name string, ordered bool) string {
	if ordered {
		return fmt.Sprintf("%s_ordered", name)
	} else {
		return name
	}
}

func doMap[A, B any](ord bool, in <-chan Try[A], n int, f func(A) (B, error)) <-chan Try[B] {
	if ord {
		return OrderedMap(in, n, f)
	}
	return Map(in, n, f)
}

func TestMap(t *testing.T) {
	for _, ord := range []bool{false, true} {

		t.Run(testname("correctness", ord), func(t *testing.T) {
			in := Wrap(th.FromRange(0, 20), nil)
			in = replaceWithError(in, 15, fmt.Errorf("err15"))

			out := doMap(ord, in, 5, func(x int) (string, error) {
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

		t.Run(testname("ordering", ord), func(t *testing.T) {
			in := Wrap(th.FromRange(0, 10000), nil)

			out := doMap(ord, in, 50, func(x int) (int, error) {
				if x%2 == 0 {
					return x, fmt.Errorf("err%06d", x)
				}

				return x, nil
			})

			outSlice, errSlice := toSliceAndErrors(out)

			if ord {
				th.ExpectSorted(t, outSlice)
				th.ExpectSorted(t, errSlice)
			} else {
				th.ExpectUnsorted(t, outSlice)
				th.ExpectUnsorted(t, errSlice)
			}

		})

	}
}

func doFilter(ord bool, in <-chan Try[int], n int, f func(int) (bool, error)) <-chan Try[int] {
	if ord {
		return OrderedFilter(in, n, f)
	}
	return Filter(in, n, f)
}

func TestFilter(t *testing.T) {
	for _, ord := range []bool{false, true} {

		t.Run(testname("correctness", ord), func(t *testing.T) {
			in := Wrap(th.FromRange(0, 20), nil)
			in = replaceWithError(in, 15, fmt.Errorf("err15"))

			out := doFilter(ord, in, 5, func(x int) (bool, error) {
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

		t.Run(testname("ordering", ord), func(t *testing.T) {
			in := Wrap(th.FromRange(0, 10000), nil)

			out := doFilter(ord, in, 50, func(x int) (bool, error) {
				if x%2 == 0 {
					return false, fmt.Errorf("err%06d", x)
				}

				return true, nil
			})

			outSlice, errSlice := toSliceAndErrors(out)

			if ord {
				th.ExpectSorted(t, outSlice)
				th.ExpectSorted(t, errSlice)
			} else {
				th.ExpectUnsorted(t, outSlice)
				th.ExpectUnsorted(t, errSlice)
			}
		})

	}
}

func doFlatMap[A, B any](ord bool, in <-chan Try[A], n int, f func(A) <-chan Try[B]) <-chan Try[B] {
	if ord {
		return OrderedFlatMap(in, n, f)
	}
	return FlatMap(in, n, f)
}

func TestFlatMap(t *testing.T) {
	for _, ord := range []bool{false, true} {

		t.Run(testname("correctness", ord), func(t *testing.T) {
			in := Wrap(th.FromRange(0, 20), nil)
			in = replaceWithError(in, 5, fmt.Errorf("err05"))
			in = replaceWithError(in, 15, fmt.Errorf("err15"))

			out := doFlatMap(ord, in, 5, func(x int) <-chan Try[string] {
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

		t.Run(testname("ordering", ord), func(t *testing.T) {
			in := Wrap(th.FromRange(0, 10000), nil)
			in = OrderedMap(in, 1, func(x int) (int, error) {
				if x%2 == 0 {
					return x, fmt.Errorf("err%06d", x)
				}
				return x, nil
			})

			out := doFlatMap(ord, in, 50, func(x int) <-chan Try[string] {
				return FromSlice([]string{
					fmt.Sprintf("%06dA", x),
					fmt.Sprintf("%06dB", x),
				})
			})

			outSlice, errSlice := toSliceAndErrors(out)

			if ord {
				th.ExpectSorted(t, outSlice)
				th.ExpectSorted(t, errSlice)
			} else {
				th.ExpectUnsorted(t, outSlice)
				th.ExpectUnsorted(t, errSlice)
			}
		})

	}
}

func doCatch(ord bool, in <-chan Try[int], n int, f func(error) error) <-chan Try[int] {
	if ord {
		return OrderedCatch(in, n, f)
	}
	return Catch(in, n, f)
}

func TestCatch(t *testing.T) {
	for _, ord := range []bool{false, true} {

		t.Run(testname("correctness", ord), func(t *testing.T) {
			in := Wrap(th.FromRange(0, 20), nil)
			in = replaceWithError(in, 5, fmt.Errorf("err05"))
			in = replaceWithError(in, 12, fmt.Errorf("err12"))
			in = replaceWithError(in, 15, fmt.Errorf("err15"))

			out := doCatch(ord, in, 5, func(err error) error {
				if err.Error() == "err05" {
					return nil // handled
				}
				if err.Error() == "err12" {
					return fmt.Errorf("%w wrapped", err) // handled
				}

				return err // leave as is
			})

			outSlice, errSlice := toSliceAndErrors(out)

			expectedSlice := make([]int, 0, 20)
			for i := 0; i < 20; i++ {
				if i == 5 || i == 12 || i == 15 {
					continue
				}
				expectedSlice = append(expectedSlice, i)
			}

			th.Sort(outSlice)
			th.Sort(errSlice)

			th.ExpectSlice(t, outSlice, expectedSlice)
			th.ExpectSlice(t, errSlice, []string{"err12 wrapped", "err15"})
		})

		t.Run(testname("ordering", ord), func(t *testing.T) {
			in := Wrap(th.FromRange(0, 10000), nil)
			in = OrderedMap(in, 1, func(x int) (int, error) {
				if x%2 == 0 {
					return x, fmt.Errorf("err%06d", x)
				}
				return x, nil
			})

			out := doCatch(ord, in, 50, func(err error) error {
				return fmt.Errorf("%w wrapped", err)
			})

			outSlice, errSlice := toSliceAndErrors(out)

			if ord {
				th.ExpectSorted(t, outSlice)
				th.ExpectSorted(t, errSlice)
			} else {
				th.ExpectUnsorted(t, outSlice)
				th.ExpectUnsorted(t, errSlice)
			}
		})

	}
}

func TestForEach(t *testing.T) {
	t.Run("no errors", func(t *testing.T) {
		in := Wrap(th.FromRange(0, 10), nil)

		sum := int64(0)
		err := ForEach(in, 3, func(x int) error {
			atomic.AddInt64(&sum, int64(x))
			return nil
		})

		th.ExpectNoError(t, err)
		th.ExpectValue(t, sum, int64(9*10/2))
	})

	t.Run("error in input", func(t *testing.T) {
		th.ExpectNotHang(t, 10*time.Second, func() {
			done := make(chan struct{})
			defer close(done)

			in := Wrap(th.InfiniteChan(done), nil)
			in = replaceWithError(in, 100, fmt.Errorf("err1"))

			sum := int64(0)
			err := ForEach(in, 3, func(x int) error {
				atomic.AddInt64(&sum, int64(x))
				return nil
			})

			th.ExpectError(t, err, "err1")
			if sum == 0 {
				t.Errorf("expected at least few iterations to complete")
			}

		})
	})

	t.Run("error in func", func(t *testing.T) {
		th.ExpectNotHang(t, 10*time.Second, func() {
			done := make(chan struct{})
			defer close(done)

			in := Wrap(th.InfiniteChan(done), nil)

			sum := int64(0)
			err := ForEach(in, 3, func(x int) error {
				if x == 100 {
					return fmt.Errorf("err1")
				}
				atomic.AddInt64(&sum, int64(x))
				return nil
			})

			th.ExpectError(t, err, "err1")
			if sum == 0 {
				t.Errorf("expected at least few iterations to complete")
			}
		})
	})

	t.Run("ordering_1", func(t *testing.T) {
		in := Wrap(th.FromRange(0, 10000), nil)

		prev := -1
		err := ForEach(in, 1, func(x int) error {
			if x < prev {
				return fmt.Errorf("expected ordered processing")
			}
			prev = x
			return nil
		})

		th.ExpectNoError(t, err)
	})

	t.Run("first error is returned when n=1", func(t *testing.T) {
		in := Wrap(th.FromRange(0, 100), nil)

		in = replaceWithError(in, 10, fmt.Errorf("err1"))
		in = replaceWithError(in, 20, fmt.Errorf("err2"))
		in = replaceWithError(in, 30, fmt.Errorf("err3"))

		err := ForEach(in, 1, func(x int) error {
			return nil
		})

		th.ExpectError(t, err, "err1")
	})

}
