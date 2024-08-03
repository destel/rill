//go:build go1.23
// +build go1.23

package rill

import (
	"errors"
	"fmt"
	"iter"
	"testing"
	"time"

	"github.com/destel/rill/internal/th"
)

func rangeInt(from, to int) iter.Seq[int] {
	return func(yield func(i int) bool) {
		for i := from; i < to; i++ {
			if !yield(i) {
				break
			}
		}
	}
}

func TestToSeq2(t *testing.T) {
	t.Run("errors", func(t *testing.T) {
		in := FromSeq(rangeInt(0, 20), nil)
		expectedErrs := []error{fmt.Errorf("err15"), fmt.Errorf("err18")}
		in = replaceWithError(in, 15, expectedErrs[0])
		in = replaceWithError(in, 18, expectedErrs[1])

		var outSlice []int
		var outErrs []error
		for i, err := range ToSeq2(in) {
			outSlice = append(outSlice, i)
			if err != nil {
				outErrs = append(outErrs, err)
			}
		}

		th.ExpectSlice(t, outSlice, []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19})
		th.ExpectSlice(t, outErrs, expectedErrs)

		time.Sleep(1 * time.Second)
		th.ExpectDrainedChan(t, in)
	})

	t.Run("errors with break", func(t *testing.T) {
		in := FromSeq(rangeInt(0, 20), nil)
		in = replaceWithError(in, 15, fmt.Errorf("err15"))
		in = replaceWithError(in, 18, fmt.Errorf("err18"))

		var outSlice []int
		var outErr error

		// sceneraio: let's client side determine when to break
		for i, err := range ToSeq2(in) {
			if err != nil {
				outErr = err
				break
			}
			outSlice = append(outSlice, i)
		}

		th.ExpectSlice(t, outSlice, []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14})
		th.ExpectError(t, outErr, "err15")

		time.Sleep(1 * time.Second)
		th.ExpectDrainedChan(t, in)
	})
}

func TestFromSeq(t *testing.T) {
	t.Run("normal ", func(t *testing.T) {
		in := FromSeq(rangeInt(0, 20), nil)

		outSlice, outErrs := toSliceAndErrors(in)
		th.Sort(outSlice)

		th.ExpectSlice(t, outSlice, []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19})
		th.ExpectSlice(t, outErrs, nil)
	})

	t.Run("with error", func(t *testing.T) {
		in := FromSeq(rangeInt(0, 20), errors.New("err"))
		a := <-in
		th.ExpectDrainedChan(t, in)
		th.ExpectError(t, a.Error, "err")
	})
}

func TestFromSeq2(t *testing.T) {
	// generate from 0 to 7, and when the value is  5, yield error
	err5 := errors.New("err5")
	gen := func(yield func(x int, err error) bool) {
		for i := 0; i < 8; i++ {
			var err error
			if i == 5 {
				err = err5
			}
			if !yield(i, err) {
				break
			}
		}
	}

	in := FromSeq2(gen)

	var outSlice []int
	var outError []error
	for a := range in {
		outSlice = append(outSlice, a.Value)
		outError = append(outError, a.Error)
	}

	th.ExpectSlice(t, outSlice, []int{0, 1, 2, 3, 4, 5, 6, 7})
	th.ExpectSlice(t, outError, []error{nil, nil, nil, nil, nil, err5, nil, nil})
}
