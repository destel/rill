//go:build go1.23
// +build go1.23

package rill

import (
	"fmt"
	"iter"
	"testing"
	"time"

	"github.com/destel/rill/internal/th"
)

func TestFromIterSeq123(t *testing.T) {
	t.Run("input iter.Seq type", func(t *testing.T) {
		var s iter.Seq[any]
		res := FromIterSeq[any](s, nil)
		th.ExpectValue(t, res, nil)
	})

	t.Run("output iter.Seq type", func(t *testing.T) {
		var _ iter.Seq2[any, error] = ToIterSeq[any](nil)
	})

	t.Run("errors", func(t *testing.T) {
		in := FromIterSeq(rangeInt(0, 20), nil)
		expectedErrs := []error{fmt.Errorf("err15"), fmt.Errorf("err18")}
		in = replaceWithError(in, 15, expectedErrs[0])
		in = replaceWithError(in, 18, expectedErrs[1])

		var outSlice []int
		var outErrs []error
		for i, err := range ToIterSeq(in) {
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
		in := FromIterSeq(rangeInt(0, 20), nil)
		in = replaceWithError(in, 15, fmt.Errorf("err15"))
		in = replaceWithError(in, 18, fmt.Errorf("err18"))

		var outSlice []int
		var outErr error

		// sceneraio: let's client side determine when to break
		for i, err := range ToIterSeq(in) {
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
