package rill

import (
	"errors"
	"fmt"
	"slices"
	"testing"
	"testing/synctest"

	"github.com/destel/rill/internal/th"
)

func TestToSeq2(t *testing.T) {
	th.RunSynctestExpectBlock(t, "nil", func(t *testing.T) {
		for range ToSeq2[int](nil) {
		}
	})

	th.RunSynctest(t, "normal", func(t *testing.T) {
		in := FromChan(th.FromSlice([]int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}), nil)
		in = replaceWithError(in, 5, fmt.Errorf("err5"))
		in = replaceWithError(in, 8, fmt.Errorf("err8"))

		outSlice := make([]int, 0, 10)
		errSlice := make([]string, 0, 10)

		out := ToSeq2(in)
		for val, err := range out {
			outSlice = append(outSlice, val)
			if err != nil {
				errSlice = append(errSlice, err.Error())
			}
		}

		synctest.Wait()
		th.ExpectDrainedChan(t, in)

		th.ExpectSlice(t, outSlice, []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})
		th.ExpectSlice(t, errSlice, []string{"err5", "err8"})
	})

	th.RunSynctest(t, "early exit", func(t *testing.T) {
		in := FromChan(th.FromSlice([]int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}), nil)
		in = replaceWithError(in, 8, fmt.Errorf("err8"))

		outSlice := make([]int, 0, 10)
		errSlice := make([]string, 0, 10)

		out := ToSeq2(in)
		for val, err := range out {
			if val == 5 {
				break
			}

			outSlice = append(outSlice, val)
			if err != nil {
				errSlice = append(errSlice, err.Error())
			}
		}

		synctest.Wait()
		th.ExpectDrainedChan(t, in)

		th.ExpectSlice(t, outSlice, []int{0, 1, 2, 3, 4})
		th.ExpectSlice(t, errSlice, nil)
	})
}

func TestFromSeq(t *testing.T) {
	t.Run("nils", func(t *testing.T) {
		in := FromSeq[int](nil, nil)
		th.ExpectValue(t, in, nil)
	})

	th.RunSynctest(t, "no error", func(t *testing.T) {
		in := slices.Values([]int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})
		out := FromSeq(in, nil)

		outSlice, outErrs := toSliceAndErrors(out)
		th.ExpectSlice(t, outSlice, []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})
		th.ExpectSlice(t, outErrs, nil)
	})

	th.RunSynctest(t, "error", func(t *testing.T) {
		in := slices.Values([]int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})
		out := FromSeq(in, errors.New("some error"))

		outSlice, outErrs := toSliceAndErrors(out)
		th.ExpectSlice(t, outSlice, nil)
		th.ExpectSlice(t, outErrs, []string{"some error"})
	})
}

func TestFromSeq2(t *testing.T) {
	t.Run("nils", func(t *testing.T) {
		in := FromSeq2[int](nil)
		th.ExpectValue(t, in, nil)
	})

	th.RunSynctest(t, "normal", func(t *testing.T) {
		// generate from 0 to 9, and when the value is  5, yield error
		gen := func(yield func(x int, err error) bool) {
			for i := range 10 {
				var err error
				if i == 5 {
					err = fmt.Errorf("err5")
				}
				if !yield(i, err) {
					break
				}
			}
		}

		in := FromSeq2(gen)

		outSlice, outErrs := toSliceAndErrors(in)
		th.ExpectSlice(t, outSlice, []int{0, 1, 2, 3, 4, 6, 7, 8, 9})
		th.ExpectSlice(t, outErrs, []string{"err5"})
	})
}
