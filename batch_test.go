package rill

import (
	"fmt"
	"testing"

	"github.com/destel/rill/internal/th"
)

func TestBatch(t *testing.T) {
	// most logic is covered by the chans pkg tests

	t.Run("correctness", func(t *testing.T) {
		in := WrapChan(th.FromRange(0, 10), fmt.Errorf("err0"))
		in = replaceWithError(in, 5, fmt.Errorf("err5"))
		in = replaceWithError(in, 7, fmt.Errorf("err7"))

		batches, errs := toSliceAndErrors(Batch(in, 3, -1))

		th.ExpectValue(t, len(batches), 3)
		th.ExpectSlice(t, batches[0], []int{0, 1, 2})
		th.ExpectSlice(t, batches[1], []int{3, 4, 6})
		th.ExpectSlice(t, batches[2], []int{8, 9})

		th.ExpectSlice(t, errs, []string{"err0", "err5", "err7"})
	})

}

func TestUnbatch(t *testing.T) {
	// most logic is covered by the common package tests

	t.Run("correctness", func(t *testing.T) {
		in := WrapSlice([][]int{{1, 2}, {3, 4}, {5, 6}, {7, 8}, {9, 10}})
		in = OrderedMap(in, 1, func(x []int) ([]int, error) {
			if x[0] == 3 {
				return nil, fmt.Errorf("err3")
			}
			if x[0] == 7 {
				return nil, fmt.Errorf("err7")
			}
			return x, nil
		})

		values, errs := toSliceAndErrors(Unbatch(in))

		th.ExpectSlice(t, values, []int{1, 2, 5, 6, 9, 10})
		th.ExpectSlice(t, errs, []string{"err3", "err7"})
	})
}
