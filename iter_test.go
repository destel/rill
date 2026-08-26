package rill

import (
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

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
		in = th.DelayEach(in, 1)

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

		var expectedSlice []Item[int]
		expectedSlice = appendVal(expectedSlice, 0, 1, 2, 3, 4)

		th.ExpectSlice(t, outSlice, expectedSlice)
		th.ExpectOpenChan(t, in)

		time.Sleep(24 * time.Hour) // eventually drained

		th.ExpectDrainedChan(t, in)
	})

	t.Run("never ranged", func(t *testing.T) {
		th.ExpectLeak(t, func(t *testing.T) {
			in := FromChan(th.FromRange(0, 20), nil)

			_ = ToSeq2(in)

			time.Sleep(24 * time.Hour) // not drained even eventually

			th.ExpectOpenChan(t, in)
		})
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

	th.RunSynctest(t, "settlement", func(t *testing.T) {
		in := FromChan(th.FromRange(0, 10), nil)

		settled, opt := Settlement()
		out := ToSeq2(in, opt)

		var outSlice []int
		for val, _ := range out {
			outSlice = append(outSlice, val)
		}

		th.ExpectSlice(t, outSlice, []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})
		th.ExpectDrainedChan(t, in)

		<-settled // should not leak
	})

	th.RunSynctest(t, "settlement (early return)", func(t *testing.T) {
		in := FromChan(th.FromRange(0, 10), nil)
		in = th.DelayEach(in, 1)

		settled, opt := Settlement()
		out := ToSeq2(in, opt)

		var outSlice []int
		for val, _ := range out {
			if val == 5 {
				break
			}
			outSlice = append(outSlice, val)
		}

		th.ExpectSlice(t, outSlice, []int{0, 1, 2, 3, 4})
		th.ExpectOpenChan(t, in)

		<-settled

		th.ExpectDrainedChan(t, in)
	})

	th.RunSynctest(t, "settlement (double consumption does not panic)", func(t *testing.T) {
		in := FromChan(th.FromRange(0, 20), nil)

		settled, opt := Settlement()
		out := ToSeq2(in, opt)

		for range out {
		}
		for range out {
		}

		<-settled // asserts that settlement is reported
		th.ExpectDrainedChan(t, in)
	})
}

func TestFromSeq(t *testing.T) {
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

	t.Run("error with nil iterator", func(t *testing.T) {
		out := FromSeq[int](nil, errors.New("some error"))

		outSlice := toItemSlice(out)

		var expectedSlice []Item[int]
		expectedSlice = appendErr(expectedSlice, errors.New("some error"))

		th.ExpectSlice(t, outSlice, expectedSlice)
	})
}

func TestFromSeq2(t *testing.T) {
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

		out := FromSeq2(gen)
		outSlice := toItemSlice(out)

		var expectedSlice []Item[int]
		expectedSlice = appendVal(expectedSlice, 0, 1, 2, 3, 4)
		expectedSlice = appendErr(expectedSlice, fmt.Errorf("err5"))
		expectedSlice = appendVal(expectedSlice, 6, 7, 8, 9)

		th.ExpectSlice(t, outSlice, expectedSlice)
	})
}
