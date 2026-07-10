package rill

import (
	"fmt"
	"testing"

	"github.com/destel/rill/internal/th"
)

func TestBatch(t *testing.T) {
	// most logic is covered by the chans pkg tests

	th.RunSynctest(t, "correctness", func(t *testing.T) {
		in := FromChan(th.FromRange(0, 10), fmt.Errorf("err0"))
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

	th.RunSynctest(t, "correctness", func(t *testing.T) {
		in := Generate(func(send func([]int), sendErr func(error)) {
			send([]int{1, 2})
			sendErr(fmt.Errorf("err1"))
			send([]int{10, 11, 12})
			send([]int{13, 14})
			sendErr(fmt.Errorf("err2"))
			send([]int{20, 21})
		})

		out := Unbatch(in)

		outSlice, outErrs := toSliceAndErrors(out)
		th.ExpectSlice(t, outSlice, []int{1, 2, 10, 11, 12, 13, 14, 20, 21})
		th.ExpectSlice(t, outErrs, []string{"err1", "err2"})
	})
}
