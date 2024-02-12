package echans

import (
	"fmt"
	"testing"

	"github.com/destel/rill/chans"
	"github.com/destel/rill/internal/th"
)

func TestWrap(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		res := Wrap[int](nil, nil)
		th.ExpectValue(t, res, nil)
	})

	t.Run("no error", func(t *testing.T) {
		in := th.FromRange(0, 20)

		out := Wrap(in, nil)

		outSlice, errSlice := toSliceAndErrors(out)

		expectedSlice := make([]int, 0, 20)
		for i := 0; i < 20; i++ {
			expectedSlice = append(expectedSlice, i)
		}

		th.ExpectSlice(t, outSlice, expectedSlice)
		th.ExpectSlice(t, errSlice, nil)
	})

	t.Run("with error", func(t *testing.T) {
		in := th.FromRange(0, 20)

		out := Wrap(in, fmt.Errorf("err0"))

		if wErr := <-out; wErr.Error == nil || wErr.Error.Error() != "err0" {
			t.Errorf("expected first output to be error, but got %v", wErr)
		}

		outSlice, errSlice := toSliceAndErrors(out)

		expectedSlice := make([]int, 0, 20)
		for i := 0; i < 20; i++ {
			expectedSlice = append(expectedSlice, i)
		}

		th.ExpectSlice(t, outSlice, expectedSlice)
		th.ExpectSlice(t, errSlice, nil) // no more errors
	})

	t.Run("ordering", func(t *testing.T) {
		in := th.FromRange(0, 20000)

		out := Wrap(in, nil)
		outSLice, _ := toSliceAndErrors(out)

		th.ExpectSorted(t, outSLice)
	})
}

func doWrapUnwrapAsync[A any](values []A, errs []error) ([]A, []error) {
	var valuesChan <-chan A
	var errsChan <-chan error

	if values != nil {
		valuesChan = chans.FromSlice(values)
	}
	if errs != nil {
		errsChan = chans.FromSlice(errs)
	}

	outValuesChan, outErrsChan := Unwrap(WrapAsync(valuesChan, errsChan))

	outValues := make([]A, 0, len(values))
	outErrs := make([]error, 0, len(errs))

	th.DoConcurrently(
		func() { outValues = chans.ToSlice(outValuesChan) },
		func() { outErrs = chans.ToSlice(outErrsChan) },
	)

	return outValues, outErrs
}

func TestWrapUnwrapAsync(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		wrapped := WrapAsync[int](nil, nil)
		th.ExpectValue(t, wrapped, nil)

		vals, errs := Unwrap[int](nil)
		th.ExpectValue(t, vals, nil)
		th.ExpectValue(t, errs, nil)
	})

	t.Run("no error", func(t *testing.T) {
		values := make([]int, 20)
		for i := 0; i < 20; i++ {
			values[i] = i
		}

		outValues, outErrs := doWrapUnwrapAsync(values, nil)

		th.ExpectSlice(t, outValues, values)
		th.ExpectSlice(t, outErrs, nil)
	})

	t.Run("only errors", func(t *testing.T) {
		errs := make([]error, 20)
		for i := 0; i < 20; i++ {
			errs[i] = fmt.Errorf("err%03d", i)
		}

		outValues, outErrs := doWrapUnwrapAsync[int](nil, errs)

		th.ExpectSlice(t, outValues, nil)
		th.ExpectSlice(t, outErrs, errs)
	})

	t.Run("values and errors", func(t *testing.T) {
		values := make([]int, 20)
		errs := make([]error, 20)
		for i := 0; i < 20; i++ {
			values[i] = i
			errs[i] = fmt.Errorf("err%03d", i)
		}

		outValues, outErrs := doWrapUnwrapAsync(values, errs)

		th.ExpectSlice(t, outValues, values)
		th.ExpectSlice(t, outErrs, errs)
	})

	t.Run("ordering", func(t *testing.T) {
		values := make([]int, 20000)
		for i := 0; i < 20000; i++ {
			values[i] = i
		}

		outValues, _ := doWrapUnwrapAsync(values, nil)

		th.ExpectSorted(t, outValues)
	})
}
