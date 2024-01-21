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
			return fmt.Sprintf("%03d", x), nil
		})

		outSlice, errSlice := toSliceAndErrors(out)
		sort.Strings(outSlice)
		sort.Strings(errSlice)

		th.ExpectSlice(t, []string{"000", "001", "002", "003", "004", "005", "006", "007", "008", "009"}, outSlice)
		th.ExpectSlice(t, []string{}, errSlice)
	})

	t.Run("errors", func(t *testing.T) {
		in := Wrap(th.FromRange(0, 10), nil, nil)

		stage1 := Map(in, 5, func(x int) (int, error) {
			if x == 3 || x == 4 {
				return 0, fmt.Errorf("err1")
			}

			return x, nil
		})

		stage2 := Map(stage1, 5, func(x int) (string, error) {
			if x == 7 {
				return "", fmt.Errorf("err2")
			}

			return fmt.Sprintf("%03d", x), nil
		})

		out := stage2

		outSlice, errSlice := toSliceAndErrors(out)
		sort.Strings(outSlice)
		sort.Strings(errSlice)

		th.ExpectSlice(t, []string{"000", "001", "002", "005", "006", "008", "009"}, outSlice)
		th.ExpectSlice(t, []string{"err1", "err1", "err2"}, errSlice)
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
			if x == 3 || x == 4 {
				return false, fmt.Errorf("err1")
			}

			return true, nil
		})

		stage2 := Filter(stage1, 5, func(x int) (bool, error) {
			if x == 7 {
				return false, fmt.Errorf("err2")
			}

			return true, nil
		})

		out := stage2

		outSlice, errSlice := toSliceAndErrors(out)
		sort.Ints(outSlice)
		sort.Strings(errSlice)

		th.ExpectSlice(t, []int{0, 1, 2, 5, 6, 8, 9}, outSlice)
		th.ExpectSlice(t, []string{"err1", "err1", "err2"}, errSlice)
	})
}

func TestFlatMap(t *testing.T) {
	in := Wrap(th.FromRange(0, 10), nil, nil)

	stage1 := Map(in, 5, func(x int) (int, error) {
		if x == 3 || x == 4 {
			return 0, fmt.Errorf("err1")
		}

		return x, nil
	})

	stage2 := FlatMap(stage1, 5, func(x int) <-chan Try[string] {
		return FromSlice([]string{
			fmt.Sprintf("%03dA", x),
			fmt.Sprintf("%03dB", x),
		})
	})

	out := stage2

	outSlice, errSlice := toSliceAndErrors(out)
	sort.Strings(outSlice)
	sort.Strings(errSlice)

	th.ExpectSlice(t, []string{"000A", "000B", "001A", "001B", "002A", "002B", "005A", "005B", "006A", "006B", "007A", "007B", "008A", "008B", "009A", "009B"}, outSlice)
}
