package rill

import (
	"fmt"
	"testing"
	"testing/synctest"

	"github.com/destel/rill/internal/th"
)

func TestWrap(t *testing.T) {
	item := Wrap(10, fmt.Errorf("err"))
	th.ExpectValue(t, item.Value, 10)
	th.ExpectError(t, item.Error, "err")
}

func TestFromSlice(t *testing.T) {
	th.RunSynctest(t, "error", func(t *testing.T) {
		out := FromSlice([]int{1, 2, 3, 4}, fmt.Errorf("some error"))
		outSlice, errs := toSliceAndErrors(out)

		th.ExpectSlice(t, outSlice, nil)
		th.ExpectSlice(t, errs, []string{"some error"})
	})

	for _, inputSize := range []int{0, 20, 4000} {
		th.RunSynctest(t, th.Name("no error", inputSize), func(t *testing.T) {
			inSlice := make([]int, inputSize)
			for i := range inSlice {
				inSlice[i] = i
			}

			out := FromSlice(inSlice, nil)
			outSlice, errs := toSliceAndErrors(out)

			th.ExpectSlice(t, outSlice, inSlice)
			th.ExpectSlice(t, errs, nil)
		})
	}
}

func TestToSlice(t *testing.T) {
	th.RunSynctestExpectBlock(t, "nil", func(t *testing.T) {
		ToSlice[int](nil)
	})

	th.RunSynctest(t, "no errors", func(t *testing.T) {
		in := FromSlice([]int{0, 1, 2, 3, 4}, nil)
		outSlice, err := ToSlice(in)

		th.ExpectSlice(t, outSlice, []int{0, 1, 2, 3, 4})
		th.ExpectNoError(t, err)
	})

	th.RunSynctest(t, "errors", func(t *testing.T) {
		inSlice := make([]int, 20)
		for i := range 20 {
			inSlice[i] = i
		}

		in := FromSlice(inSlice, nil)
		in = replaceWithError(in, 15, fmt.Errorf("err015"))
		in = replaceWithError(in, 18, fmt.Errorf("err018"))

		outSlice, err := ToSlice(in)

		th.ExpectSlice(t, outSlice, inSlice[:15])
		th.ExpectError(t, err, "err015")

		synctest.Wait()
		th.ExpectDrainedChan(t, in)
	})
}

func TestFromChan(t *testing.T) {
	th.RunSynctestExpectBlock(t, "nil no errors", func(t *testing.T) {
		out := FromChan[int](nil, nil)
		ToSlice(out)
	})

	th.RunSynctestExpectBlock(t, "nil with error", func(t *testing.T) {
		out := FromChan[int](nil, fmt.Errorf("err"))

		outSlice, errs, _ := toSliceAndErrorsNB(out)
		th.ExpectSlice(t, outSlice, nil)
		th.ExpectSlice(t, errs, []string{"err"})
	})

	th.RunSynctest(t, "no error", func(t *testing.T) {
		inSlice := []int{0, 1, 2, 3, 4, 5}

		out := FromChan(th.FromSlice(inSlice), nil)
		outSlice, errs := toSliceAndErrors(out)

		th.ExpectSlice(t, outSlice, inSlice)
		th.ExpectSlice(t, errs, nil)
	})

	th.RunSynctest(t, "error", func(t *testing.T) {
		inSlice := []int{0, 1, 2, 3, 4, 5}

		out := FromChan(th.FromSlice(inSlice), fmt.Errorf("some error"))
		outSlice, errs, _ := toSliceAndErrorsNB(out)

		th.ExpectSlice(t, outSlice, inSlice)
		th.ExpectSlice(t, errs, []string{"some error"})
	})

	th.RunSynctest(t, "error should go before values", func(t *testing.T) {
		inSlice := []int{0, 1, 2, 3, 4, 5}

		out := FromChan(th.FromSlice(inSlice), fmt.Errorf("some error"))
		defer Discard(out)

		item := <-out
		th.ExpectError(t, item.Error, "some error")

		item = <-out
		th.ExpectValue(t, item.Value, inSlice[0])
		th.ExpectNoError(t, item.Error)

		item = <-out
		th.ExpectValue(t, item.Value, inSlice[1])
		th.ExpectNoError(t, item.Error)
	})
}

func TestFromChans(t *testing.T) {
	t.Run("nils", func(t *testing.T) {
		out := FromChans[int](nil, nil)
		th.ExpectValue(t, out, nil)
	})

	th.RunSynctest(t, "nil values", func(t *testing.T) {
		out := FromChans[int](nil, th.FromSlice([]error{fmt.Errorf("err001"), fmt.Errorf("err002")}))
		outSlice, errs := toSliceAndErrors(out)
		th.ExpectSlice(t, outSlice, nil)
		th.ExpectSlice(t, errs, []string{"err001", "err002"})
	})

	th.RunSynctest(t, "nil errors", func(t *testing.T) {
		out := FromChans(th.FromSlice([]int{0, 1, 2, 3, 4}), nil)
		outSlice, errs := toSliceAndErrors(out)
		th.ExpectSlice(t, outSlice, []int{0, 1, 2, 3, 4})
		th.ExpectSlice(t, errs, nil)
	})

	th.RunSynctest(t, "not nil", func(t *testing.T) {
		out := FromChans(th.FromSlice([]int{0, 1, 2, 3, 4}), th.FromSlice([]error{fmt.Errorf("err001"), fmt.Errorf("err002")}))
		outSlice, errs := toSliceAndErrors(out)
		th.ExpectSlice(t, outSlice, []int{0, 1, 2, 3, 4})
		th.ExpectSlice(t, errs, []string{"err001", "err002"})
	})
}

func TestToChans(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		out, errs := ToChans[int](nil)
		th.ExpectValue(t, out, nil)
		th.ExpectValue(t, errs, nil)
	})

	th.RunSynctest(t, "normal", func(t *testing.T) {
		in := FromSlice([]int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}, nil)
		in = replaceWithError(in, 3, fmt.Errorf("err003"))
		in = replaceWithError(in, 7, fmt.Errorf("err007"))

		out, errs := ToChans(in)

		var outSlice []int
		var errsSlice []error
		th.DoConcurrently(
			func() { outSlice = th.ToSlice(out) },
			func() { errsSlice = th.ToSlice(errs) },
		)

		th.ExpectSlice(t, outSlice, []int{0, 1, 2, 4, 5, 6, 8, 9})
		th.ExpectErrorSlice(t, errsSlice, []string{"err003", "err007"})
	})
}

func TestGenerate(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		in := Generate(func(send func(int), sendErr func(error)) {
			for i := range 10 {
				if i%2 == 0 {
					send(i)
				} else {
					sendErr(fmt.Errorf("err%d", i))
				}
			}
		})

		outSlice, errSlice := toSliceAndErrors(in)

		th.ExpectSlice(t, outSlice, []int{0, 2, 4, 6, 8})
		th.ExpectSlice(t, errSlice, []string{"err1", "err3", "err5", "err7", "err9"})
	})
}
