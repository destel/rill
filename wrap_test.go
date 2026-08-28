package rill

import (
	"fmt"
	"testing"
	"testing/synctest"
	"time"

	"github.com/destel/rill/internal/th"
)

func TestWrap(t *testing.T) {
	item := Wrap(10, nil)
	th.ExpectValue(t, item.Value, 10)
	th.ExpectNoError(t, item.Error)

	item = Wrap(10, fmt.Errorf("err"))
	th.ExpectValue(t, item.Value, 0)
	th.ExpectError(t, item.Error, "err")
}

func TestFromSlice(t *testing.T) {
	th.TestVariants(t, "input_size", []int{0, 20, 4000}, func(t *testing.T, inputSize int) {
		th.RunSynctest(t, "no error", func(t *testing.T) {
			var inSlice []int
			var expectedSlice []Item[int]

			for i := range inputSize {
				inSlice = append(inSlice, i)
				expectedSlice = appendVal(expectedSlice, i)
			}

			out := FromSlice(inSlice, nil)
			outSlice := toItemSlice(out)

			th.ExpectSlice(t, outSlice, expectedSlice)
		})

		th.RunSynctest(t, "error", func(t *testing.T) {
			var inSlice []int
			var expectedSlice []Item[int]

			err := fmt.Errorf("some error")

			for i := range inputSize {
				inSlice = append(inSlice, i)
				expectedSlice = appendVal(expectedSlice, i)
			}
			expectedSlice = appendErr(expectedSlice, err)

			out := FromSlice(inSlice, err)
			outSlice := toItemSlice(out)

			th.ExpectSlice(t, outSlice, expectedSlice)
		})
	})

	t.Run("round trip", func(t *testing.T) {
		expectedValues := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
		expectedErr := fmt.Errorf("partial result")

		values, err := ToSlice(FromSlice(expectedValues, expectedErr))

		th.ExpectSlice(t, values, expectedValues)
		th.ExpectError(t, err, "partial result")
	})
}

func TestToSlice(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		th.ExpectBlock(t, func(t *testing.T) {
			_, _ = ToSlice[int](nil)
		})
	})

	th.RunSynctest(t, "no errors", func(t *testing.T) {
		in := FromSlice([]int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}, nil)

		outSlice, err := ToSlice(in)

		th.ExpectSlice(t, outSlice, []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})
		th.ExpectNoError(t, err)
		th.ExpectDrainedChan(t, in)
	})

	th.RunSynctest(t, "errors", func(t *testing.T) {
		in := FromSlice([]int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}, nil)
		in = replaceWithError(in, 5, fmt.Errorf("err005"))
		in = replaceWithError(in, 7, fmt.Errorf("err007"))
		in = th.DelayEach(in, 1)

		outSlice, err := ToSlice(in)

		th.ExpectSlice(t, outSlice, []int{0, 1, 2, 3, 4})
		th.ExpectError(t, err, "err005")
		th.ExpectOpenChan(t, in)

		time.Sleep(24 * time.Hour) // eventually drained

		th.ExpectDrainedChan(t, in)
	})

	t.Run("unclosed", func(t *testing.T) {
		th.ExpectLeak(t, func(t *testing.T) {
			in := FromSlice([]int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}, nil)
			in = replaceWithError(in, 5, fmt.Errorf("err005"))
			in = th.DontClose(in)

			_, _ = ToSlice(in)
		})
	})

	th.RunSynctest(t, "settlement", func(t *testing.T) {
		in := FromSlice([]int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}, nil)

		settled, opt := Settlement()
		outSlice, err := ToSlice(in, opt)

		th.ExpectSlice(t, outSlice, []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})
		th.ExpectNoError(t, err)
		th.ExpectDrainedChan(t, in)

		<-settled // should not leak
	})

	th.RunSynctest(t, "settlement (early return)", func(t *testing.T) {
		in := FromSlice([]int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}, nil)
		in = replaceWithError(in, 5, fmt.Errorf("err005"))
		in = replaceWithError(in, 7, fmt.Errorf("err007"))
		in = th.DelayEach(in, 1)

		settled, opt := Settlement()
		outSlice, err := ToSlice(in, opt)

		th.ExpectSlice(t, outSlice, []int{0, 1, 2, 3, 4})
		th.ExpectError(t, err, "err005")
		th.ExpectOpenChan(t, in)

		<-settled

		th.ExpectDrainedChan(t, in)
	})
}

func TestFromChan(t *testing.T) {
	t.Run("nil no errors", func(t *testing.T) {
		th.ExpectBlock(t, func(t *testing.T) {
			out := FromChan[int](nil, nil)
			toItemSlice(out)
		})

	})

	th.RunSynctest(t, "nil with error", func(t *testing.T) {
		out := FromChan[int](nil, fmt.Errorf("err"))
		outSlice := toItemSlice(out)

		var expectedSlice []Item[int]
		expectedSlice = appendErr(expectedSlice, fmt.Errorf("err"))

		th.ExpectSlice(t, outSlice, expectedSlice)
	})

	th.RunSynctest(t, "non-nil with error", func(t *testing.T) {
		in := th.FromSlice([]int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})

		out := FromChan(in, fmt.Errorf("some error"))
		outSlice := toItemSlice(out)

		var expectedSlice []Item[int]
		expectedSlice = appendErr(expectedSlice, fmt.Errorf("some error"))

		th.ExpectSlice(t, outSlice, expectedSlice)

		// the channel is not consumed
		th.ExpectValue(t, <-in, 0)
	})

	th.RunSynctest(t, "no error", func(t *testing.T) {
		inSlice := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}

		out := FromChan(th.FromSlice(inSlice), nil)
		outSlice := toItemSlice(out)

		var expectedSlice []Item[int]
		expectedSlice = appendVal(expectedSlice, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9)

		th.ExpectSlice(t, outSlice, expectedSlice)
	})
}

func TestFromChans(t *testing.T) {
	t.Run("nils", func(t *testing.T) {
		out := FromChans[int](nil, nil)
		th.ExpectValue(t, out, nil)
	})

	// A nil input is an input that's never closed, so the output stays open forever.
	t.Run("nil values", func(t *testing.T) {
		var errs []string

		th.ExpectBlock(t, func(t *testing.T) {
			out := FromChans[int](nil, th.FromSlice([]error{fmt.Errorf("err001"), fmt.Errorf("err002")}))
			for x := range out {
				if x.Error != nil {
					errs = append(errs, x.Error.Error())
				}
			}
		})

		th.ExpectSlice(t, errs, []string{"err001", "err002"})
	})

	t.Run("nil errors", func(t *testing.T) {
		var outSlice []int

		th.ExpectBlock(t, func(t *testing.T) {
			out := FromChans(th.FromSlice([]int{0, 1, 2, 3, 4}), nil)
			for x := range out {
				outSlice = append(outSlice, x.Value)
			}
		})

		th.ExpectSlice(t, outSlice, []int{0, 1, 2, 3, 4})
	})

	th.RunSynctest(t, "not nil", func(t *testing.T) {
		out := FromChans(
			th.FromSlice([]int{0, 1, 2, 3, 4}),
			th.FromSlice([]error{fmt.Errorf("err001"), fmt.Errorf("err002")}),
		)
		outSlice, errs := toSliceAndErrors(out)

		th.ExpectSlice(t, outSlice, []int{0, 1, 2, 3, 4})
		th.ExpectSlice(t, errs, []string{"err001", "err002"})
	})

	t.Run("unclosed values", func(t *testing.T) {
		th.ExpectLeak(t, func(t *testing.T) {
			out := FromChans(
				th.DontClose(th.FromSlice([]int{0, 1, 2, 3, 4})),
				th.FromSlice([]error{fmt.Errorf("err001"), fmt.Errorf("err002")}),
			)

			Discard(out)
		})
	})

	t.Run("unclosed errors", func(t *testing.T) {
		th.ExpectLeak(t, func(t *testing.T) {
			out := FromChans(
				th.FromSlice([]int{0, 1, 2, 3, 4}),
				th.DontClose(th.FromSlice([]error{fmt.Errorf("err001"), fmt.Errorf("err002")})),
			)

			Discard(out)
		})
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
		var errSlice []string
		th.DoConcurrently(
			func() { outSlice = th.ToSlice(out) },
			func() {
				for err := range errs {
					if err != nil {
						errSlice = append(errSlice, err.Error())
					}
				}
			},
		)

		th.ExpectSlice(t, outSlice, []int{0, 1, 2, 4, 5, 6, 8, 9})
		th.ExpectSlice(t, errSlice, []string{"err003", "err007"})
	})

	t.Run("unclosed", func(t *testing.T) {
		th.ExpectLeak(t, func(t *testing.T) {
			in := FromSlice([]int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}, nil)
			in = th.DontClose(in)

			out, errs := ToChans(in)
			Discard(out)
			Discard(errs)
		})
	})

	t.Run("non-concurrent consumption", func(t *testing.T) {
		th.ExpectBlock(t, func(t *testing.T) {
			in := FromSlice([]int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}, fmt.Errorf("err"))

			out, errs := ToChans(in)
			Drain(out)
			Drain(errs)
		})
	})
}

func TestGenerate(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		in := Generate(func(send func(int), sendError func(error)) {
			send(1)
			sendError(fmt.Errorf("err1"))
			send(2)
			sendError(nil) // skipped: produces no item
			send(3)
		})

		outSlice := toItemSlice(in)

		var expectedSlice []Item[int]
		expectedSlice = appendVal(expectedSlice, 1)
		expectedSlice = appendErr(expectedSlice, fmt.Errorf("err1"))
		expectedSlice = appendVal(expectedSlice, 2)
		expectedSlice = appendVal(expectedSlice, 3)

		th.ExpectSlice(t, outSlice, expectedSlice)
	})
}
