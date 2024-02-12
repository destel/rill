package echans

import (
	"github.com/destel/rill/chans"
	"github.com/destel/rill/internal/common"
)

type Try[A any] struct {
	V     A
	Error error
}

func Wrap[A any](values <-chan A, err error) <-chan Try[A] {
	if values == nil && err == nil {
		return nil
	}

	out := make(chan Try[A])
	go func() {
		defer close(out)

		if err != nil {
			out <- Try[A]{Error: err} // error goes first
		}

		for x := range values {
			out <- Try[A]{V: x}
		}
	}()

	return out
}

func WrapAsync[A any](values <-chan A, errs <-chan error) <-chan Try[A] {
	wrappedValues := chans.Map(values, 1, func(a A) Try[A] {
		return Try[A]{V: a}
	})

	wrappedErrs := common.MapOrFilter(errs, 1, func(err error) (Try[A], bool) {
		if err == nil {
			return Try[A]{}, false
		}
		return Try[A]{Error: err}, true
	})

	if wrappedValues == nil {
		return wrappedErrs
	}
	if wrappedErrs == nil {
		return wrappedValues
	}
	return chans.Merge(wrappedErrs, wrappedValues)
}

func Unwrap[A any](in <-chan Try[A]) (<-chan A, <-chan error) {
	if in == nil {
		return nil, nil
	}

	out := make(chan A)
	errs := make(chan error)

	go func() {
		defer close(out)
		defer close(errs)

		for x := range in {
			if x.Error != nil {
				errs <- x.Error
			} else {
				out <- x.V
			}
		}
	}()

	return out, errs
}
