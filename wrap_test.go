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
		out := FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9}, fmt.Errorf("some error"))

		outSlice := toItemSlice(out)

		var expectedSlice []Item[int]
		expectedSlice = appendErr(expectedSlice, fmt.Errorf("some error"))

		th.ExpectSlice(t, outSlice, expectedSlice)
	})

	for _, inputSize := range []int{0, 20, 4000} {
		th.RunSynctest(t, th.Name("no error", inputSize), func(t *testing.T) {
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
	}
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

		synctest.Wait()
		th.ExpectDrainedChan(t, in)

		th.ExpectSlice(t, outSlice, []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})
		th.ExpectNoError(t, err)
	})

	th.RunSynctest(t, "errors", func(t *testing.T) {
		in := FromSlice([]int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}, nil)
		in = replaceWithError(in, 5, fmt.Errorf("err005"))
		in = replaceWithError(in, 7, fmt.Errorf("err007"))

		outSlice, err := ToSlice(in)

		synctest.Wait()
		th.ExpectDrainedChan(t, in)

		th.ExpectSlice(t, outSlice, []int{0, 1, 2, 3, 4})
		th.ExpectError(t, err, "err005")
	})

	t.Run("unclosed", func(t *testing.T) {
		th.ExpectLeak(t, func(t *testing.T) {
			in := FromSlice([]int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}, nil)
			in = replaceWithError(in, 5, fmt.Errorf("err005"))
			in = th.DontClose(in)

			_, _ = ToSlice(in)
		})
	})
}

func TestFromChan(t *testing.T) {
	t.Run("nil no errors", func(t *testing.T) {
		th.ExpectBlock(t, func(t *testing.T) {
			out := FromChan[int](nil, nil)
			toItemSlice(out)
		})

	})

	t.Run("nil with error", func(t *testing.T) {
		var outSlice []Item[int]

		th.ExpectBlock(t, func(t *testing.T) {
			out := FromChan[int](nil, fmt.Errorf("err"))
			for item := range out {
				outSlice = appendTry(outSlice, item)
			}
		})

		var expectedSlice []Item[int]
		expectedSlice = appendErr(expectedSlice, fmt.Errorf("err"))

		th.ExpectSlice(t, outSlice, expectedSlice)
	})

	th.RunSynctest(t, "no error", func(t *testing.T) {
		inSlice := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}

		out := FromChan(th.FromSlice(inSlice), nil)
		outSlice := toItemSlice(out)

		var expectedSlice []Item[int]
		expectedSlice = appendVal(expectedSlice, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9)

		th.ExpectSlice(t, outSlice, expectedSlice)
	})

	th.RunSynctest(t, "error", func(t *testing.T) {
		inSlice := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}

		out := FromChan(th.FromSlice(inSlice), fmt.Errorf("some error"))

		outSlice := toItemSlice(out)

		var expectedSlice []Item[int]
		expectedSlice = appendErr(expectedSlice, fmt.Errorf("some error"))
		expectedSlice = appendVal(expectedSlice, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9)

		th.ExpectSlice(t, outSlice, expectedSlice)
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

		outSlice := toItemSlice(in)

		var expectedSlice []Item[int]
		for i := range 10 {
			if i%2 == 0 {
				expectedSlice = appendVal(expectedSlice, i)
			} else {
				expectedSlice = appendErr(expectedSlice, fmt.Errorf("err%d", i))
			}
		}

		th.ExpectSlice(t, outSlice, expectedSlice)
	})
}
