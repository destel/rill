package echans

import (
	"fmt"
	"sort"
	"testing"

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

		th.ExpectSlice(t, []string{"00", "01", "02", "03", "04", "05", "06", "07", "08", "09"}, outSlice)
		th.ExpectSlice(t, []string{}, errSlice)
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

		th.ExpectSlice(t, []string{"00", "01", "02", "05", "06", "08", "09"}, outSlice)
		th.ExpectSlice(t, []string{"err1", "err2", "err3"}, errSlice)
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

		th.ExpectSlice(t, []int{0, 2, 4, 6, 8}, outSlice)
		th.ExpectSlice(t, []string{}, errSlice)
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

		th.ExpectSlice(t, []int{0, 1, 2, 5, 6, 8, 9}, outSlice)
		th.ExpectSlice(t, []string{"err1", "err2", "err3"}, errSlice)
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

		th.ExpectSlice(t, []string{"00A", "00B", "01A", "01B", "02A", "02B", "03A", "03B", "04A", "04B", "05A", "05B", "06A", "06B", "07A", "07B", "08A", "08B", "09A", "09B"}, outSlice)
		th.ExpectSlice(t, []string{}, errSlice)
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

		th.ExpectSlice(t, []string{"00A", "00B", "01A", "01B", "02A", "02B", "05A", "05B", "06A", "06B", "07A", "07B", "08A", "08B", "09A", "09B"}, outSlice)
		th.ExpectSlice(t, []string{"err1", "err2"}, errSlice)
	})

}
