package rill

import (
	"fmt"
	"testing"
	"time"

	"github.com/destel/rill/internal/th"
)

func TestWrapUnwrapSlice(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		in := WrapSlice[int](nil)
		outSlice, err := ToSlice(in)

		th.ExpectSlice(t, outSlice, nil)
		th.ExpectNoError(t, err)
	})

	t.Run("no errors", func(t *testing.T) {
		inSlice := make([]int, 20)
		for i := 0; i < 20; i++ {
			inSlice[i] = i
		}

		in := WrapSlice(inSlice)
		outSlice, err := ToSlice(in)

		th.ExpectSlice(t, outSlice, inSlice)
		th.ExpectNoError(t, err)
	})

	t.Run("errors", func(t *testing.T) {
		inSlice := make([]int, 20)
		for i := 0; i < 20; i++ {
			inSlice[i] = i
		}

		in := WrapSlice(inSlice)
		in = replaceWithError(in, 15, fmt.Errorf("err15"))
		in = replaceWithError(in, 18, fmt.Errorf("err18"))

		outSlice, err := ToSlice(in)

		th.ExpectSlice(t, outSlice, inSlice[:15])
		th.ExpectError(t, err, "err15")

		time.Sleep(1 * time.Second)
		th.ExpectDrainedChan(t, in)
	})
}

func TestWrap(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		res := Wrap[int](nil, nil)
		th.ExpectValue(t, res, nil)
	})

	t.Run("no error", func(t *testing.T) {
		var inSlice []int
		var expectedOutSlice []Try[int]

		for i := 0; i < 20000; i++ {
			inSlice = append(inSlice, i)
			expectedOutSlice = append(expectedOutSlice, Try[int]{Value: i})
		}

		wrapped := Wrap(th.FromSlice(inSlice), nil)
		outSlice := th.ToSlice(wrapped)

		th.ExpectSlice(t, outSlice, expectedOutSlice)
	})

	t.Run("with error", func(t *testing.T) {
		var inSlice []int
		var expectedOutSlice []Try[int]

		err := fmt.Errorf("err")
		expectedOutSlice = append(expectedOutSlice, Try[int]{Error: err})

		for i := 0; i < 20000; i++ {
			inSlice = append(inSlice, i)
			expectedOutSlice = append(expectedOutSlice, Try[int]{Value: i})
		}

		wrapped := Wrap(th.FromSlice(inSlice), err)
		outSlice := th.ToSlice(wrapped)

		th.ExpectSlice(t, outSlice, expectedOutSlice)
	})
}

func TestWrapUnwrapAsync(t *testing.T) {
	// slices -> WrapSlice -> WrapAsync -> Unwrap -> ToSlice -> compare
	runTest := func(name string, valsIn []int, errsIn []error) {
		t.Run(name, func(t *testing.T) {
			var valsInChan <-chan int
			if len(valsIn) > 0 {
				valsInChan = th.FromSlice(valsIn)
			}

			var errsInChan <-chan error
			if len(errsIn) > 0 {
				errsInChan = th.FromSlice(errsIn)
			}

			valsOutChan, errsOutChan := Unwrap(WrapAsync(valsInChan, errsInChan))

			if valsInChan == nil && errsInChan == nil {
				th.ExpectValue(t, valsOutChan, nil)
				th.ExpectValue(t, errsOutChan, nil)
				return
			}

			var valsOut []int
			var errsOut []error

			th.DoConcurrently(
				func() { valsOut = th.ToSlice(valsOutChan) },
				func() { errsOut = th.ToSlice(errsOutChan) },
			)

			// nil errors are not expected in the output
			var expectedErrors []error
			for _, err := range errsIn {
				if err != nil {
					expectedErrors = append(expectedErrors, err)
				}
			}

			th.ExpectSlice(t, valsOut, valsIn)
			th.ExpectSlice(t, errsOut, expectedErrors)
		})
	}

	makeSlice := func(n int) []int {
		out := make([]int, n)
		for i := 0; i < n; i++ {
			out[i] = i
		}
		return out
	}

	makeErrSlice := func(n int) []error {
		out := make([]error, n)
		for i := 0; i < n; i++ {
			out[i] = fmt.Errorf("err%06d", i)
		}
		return out
	}

	runTest("nil", nil, nil)
	runTest("no errors", makeSlice(10000), nil)
	runTest("only errors", nil, makeErrSlice(10000))
	runTest("values and errors", makeSlice(10000), makeErrSlice(10000))
	runTest("values and nil errors", makeSlice(10), []error{nil, nil, fmt.Errorf("err"), nil})
}
