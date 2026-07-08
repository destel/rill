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
	t.Run("nil", func(t *testing.T) {
		th.ExpectBlock(t, func(t *testing.T) {
			for range ToSeq2[int](nil) {
			}
		})
	})

	th.RunSynctest(t, "normal", func(t *testing.T) {
		in := FromSlice([]int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}, nil)
		in = replaceWithError(in, 5, fmt.Errorf("err5"))
		in = replaceWithError(in, 8, fmt.Errorf("err8"))

		out := ToSeq2(in)

		var outSlice []Item[int]
		for val, err := range out {
			if err != nil {
				outSlice = appendErr(outSlice, err)
				continue
			}
			outSlice = appendVal(outSlice, val)
		}

		synctest.Wait()
		th.ExpectDrainedChan(t, in)

		var expectedSlice []Item[int]
		expectedSlice = appendVal(expectedSlice, 0, 1, 2, 3, 4)
		expectedSlice = appendErr(expectedSlice, fmt.Errorf("err5"))
		expectedSlice = appendVal(expectedSlice, 6, 7)
		expectedSlice = appendErr(expectedSlice, fmt.Errorf("err8"))
		expectedSlice = appendVal(expectedSlice, 9)

		th.ExpectSlice(t, outSlice, expectedSlice)
	})

	th.RunSynctest(t, "early exit", func(t *testing.T) {
		in := FromSlice([]int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}, nil)
		in = replaceWithError(in, 8, fmt.Errorf("err8"))

		out := ToSeq2(in)

		var outSlice []Item[int]
		for val, err := range out {
			if val == 5 {
				break
			}

			if err != nil {
				outSlice = appendErr(outSlice, err)
				continue
			}
			outSlice = appendVal(outSlice, val)
		}

		synctest.Wait()
		th.ExpectDrainedChan(t, in)

		var expectedSlice []Item[int]
		expectedSlice = appendVal(expectedSlice, 0, 1, 2, 3, 4)

		th.ExpectSlice(t, outSlice, expectedSlice)
	})

	t.Run("unclosed", func(t *testing.T) {
		th.ExpectLeak(t, func(t *testing.T) {
			in := FromChan(th.FromRange(0, 20), nil)
			in = th.DontClose(in)

			out := ToSeq2(in)
			for range out {
				break // early return immediately
			}
		})
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

		outSlice := toItemSlice(out)

		var expectedSlice []Item[int]
		expectedSlice = appendVal(expectedSlice, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9)

		th.ExpectSlice(t, outSlice, expectedSlice)
	})

	th.RunSynctest(t, "error", func(t *testing.T) {
		in := slices.Values([]int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})
		out := FromSeq(in, errors.New("some error"))

		outSlice := toItemSlice(out)

		var expectedSlice []Item[int]
		expectedSlice = appendErr(expectedSlice, errors.New("some error"))

		th.ExpectSlice(t, outSlice, expectedSlice)
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
				var val int = i

				if i == 5 {
					val = 0
					err = fmt.Errorf("err5")
				}
				if !yield(val, err) {
					break
				}
			}
		}

		out := FromSeq2(gen)
		outSlice := toItemSlice(out)

		var expectedSlice []Item[int]
		expectedSlice = appendVal(expectedSlice, 0, 1, 2, 3, 4)
		expectedSlice = appendErr(expectedSlice, fmt.Errorf("err5"))
		expectedSlice = appendVal(expectedSlice, 6, 7, 8, 9)

		th.ExpectSlice(t, outSlice, expectedSlice)
	})
}
