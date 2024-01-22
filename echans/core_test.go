package echans

import (
	"fmt"
	"sort"
	"sync/atomic"
	"testing"
	"time"

	"github.com/destel/rill/chans"
	"github.com/destel/rill/internal/th"
)

func TestMap(t *testing.T) {
	t.Run("no errors", func(t *testing.T) {
		in := Wrap(th.FromRange(0, 10), nil, nil)

		out := Map(in, 3, func(x int) (string, error) {
			return fmt.Sprintf("%02d", x), nil
		})

		outSlice, errSlice := toSliceAndErrors(out)
		sort.Strings(outSlice)
		sort.Strings(errSlice)

		th.ExpectSlice(t, outSlice, []string{"00", "01", "02", "03", "04", "05", "06", "07", "08", "09"})
		th.ExpectSlice(t, errSlice, []string{})
	})

	t.Run("errors", func(t *testing.T) {
		in := Wrap(th.FromRange(0, 10), nil, nil)

		stage1 := Map(in, 5, func(x int) (int, error) {
			if x == 3 {
				return 0, fmt.Errorf("err1")
			}
			if x == 4 {
				return 0, fmt.Errorf("err2")
			}

			return x, nil
		})

		stage2 := Map(stage1, 5, func(x int) (string, error) {
			if x == 7 {
				return "", fmt.Errorf("err3")
			}

			return fmt.Sprintf("%02d", x), nil
		})

		out := stage2

		outSlice, errSlice := toSliceAndErrors(out)
		sort.Strings(outSlice)
		sort.Strings(errSlice)

		th.ExpectSlice(t, outSlice, []string{"00", "01", "02", "05", "06", "08", "09"})
		th.ExpectSlice(t, errSlice, []string{"err1", "err2", "err3"})
	})
}

func TestFilter(t *testing.T) {
	t.Run("no errors", func(t *testing.T) {
		in := Wrap(th.FromRange(0, 10), nil, nil)

		out := Filter(in, 3, func(x int) (bool, error) {
			return x%2 == 0, nil
		})

		outSlice, errSlice := toSliceAndErrors(out)
		sort.Ints(outSlice)
		sort.Strings(errSlice)

		th.ExpectSlice(t, outSlice, []int{0, 2, 4, 6, 8})
		th.ExpectSlice(t, errSlice, []string{})
	})

	t.Run("errors", func(t *testing.T) {
		in := Wrap(th.FromRange(0, 10), nil, nil)

		stage1 := Filter(in, 5, func(x int) (bool, error) {
			if x == 3 {
				return false, fmt.Errorf("err1")
			}
			if x == 4 {
				return false, fmt.Errorf("err2")
			}

			return true, nil
		})

		stage2 := Filter(stage1, 5, func(x int) (bool, error) {
			if x == 7 {
				return false, fmt.Errorf("err3")
			}

			return true, nil
		})

		out := stage2

		outSlice, errSlice := toSliceAndErrors(out)
		sort.Ints(outSlice)
		sort.Strings(errSlice)

		th.ExpectSlice(t, outSlice, []int{0, 1, 2, 5, 6, 8, 9})
		th.ExpectSlice(t, errSlice, []string{"err1", "err2", "err3"})
	})
}

func TestFlatMap(t *testing.T) {
	t.Run("no errors", func(t *testing.T) {
		in := Wrap(th.FromRange(0, 10), nil, nil)

		out := FlatMap(in, 3, func(x int) <-chan Try[string] {
			return FromSlice([]string{
				fmt.Sprintf("%02dA", x),
				fmt.Sprintf("%02dB", x),
			})
		})

		outSlice, errSlice := toSliceAndErrors(out)
		sort.Strings(outSlice)
		sort.Strings(errSlice)

		th.ExpectSlice(t, outSlice, []string{"00A", "00B", "01A", "01B", "02A", "02B", "03A", "03B", "04A", "04B", "05A", "05B", "06A", "06B", "07A", "07B", "08A", "08B", "09A", "09B"})
		th.ExpectSlice(t, errSlice, []string{})
	})

	t.Run("errors", func(t *testing.T) {
		in := Wrap(th.FromRange(0, 10), nil, nil)

		stage1 := Map(in, 5, func(x int) (int, error) {
			if x == 3 {
				return 0, fmt.Errorf("err1")
			}
			if x == 4 {
				return 0, fmt.Errorf("err2")
			}

			return x, nil
		})

		stage2 := FlatMap(stage1, 5, func(x int) <-chan Try[string] {
			return FromSlice([]string{
				fmt.Sprintf("%02dA", x),
				fmt.Sprintf("%02dB", x),
			})
		})

		out := stage2

		outSlice, errSlice := toSliceAndErrors(out)
		sort.Strings(outSlice)
		sort.Strings(errSlice)

		th.ExpectSlice(t, outSlice, []string{"00A", "00B", "01A", "01B", "02A", "02B", "05A", "05B", "06A", "06B", "07A", "07B", "08A", "08B", "09A", "09B"})
		th.ExpectSlice(t, errSlice, []string{"err1", "err2"})
	})

}

func TestForEach(t *testing.T) {
	t.Run("no errors", func(t *testing.T) {
		sum := int64(0)

		in := Wrap(th.FromRange(0, 10), nil, nil)

		err := ForEach(in, 3, func(x int) error {
			atomic.AddInt64(&sum, int64(x))
			return nil
		})

		th.ExpectNoError(t, err)
		th.ExpectValue(t, sum, int64(9*10/2))
	})

	t.Run("error in input", func(t *testing.T) {
		th.NotHang(t, 10*time.Second, func() {
			done := make(chan struct{})
			defer close(done)

			in := Wrap(th.InfiniteChan(done), nil, nil)
			in = putErrorAt(in, fmt.Errorf("err"), 100)

			defer chans.DrainNB(in)

			err := ForEach(in, 3, func(x int) error {
				return nil
			})

			th.ExpectError(t, err, fmt.Errorf("err"))
		})
	})

	t.Run("error in func", func(t *testing.T) {
		th.NotHang(t, 10*time.Second, func() {
			done := make(chan struct{})
			defer close(done)

			in := Wrap(th.InfiniteChan(done), nil, nil)

			err := ForEach(in, 3, func(x int) error {
				if x == 100 {
					return fmt.Errorf("err")
				}
				return nil
			})

			th.ExpectError(t, err, fmt.Errorf("err"))
		})
	})

	t.Run("first error is returned", func(t *testing.T) {
		in := Wrap(th.FromRange(0, 100), nil, nil)

		in = putErrorAt(in, fmt.Errorf("err1"), 10)
		in = putErrorAt(in, fmt.Errorf("err2"), 20)
		in = putErrorAt(in, fmt.Errorf("err3"), 30)

		err := ForEach(in, 1, func(x int) error {
			return nil
		})

		th.ExpectError(t, err, fmt.Errorf("err1"))
	})
}
