package echans

import "github.com/destel/rill/chans"

type Try[A any] struct {
	V     A
	Error error
}

func Wrap[A any](values <-chan A, err error) <-chan Try[A] {
	if values == nil && err == nil {
		return nil
	}

	wrappedValues := chans.Map(values, 1, func(x A) Try[A] {
		return Try[A]{V: x}
	})

	var wrappedErrs chan Try[A]
	if err != nil {
		wrappedErrs = make(chan Try[A], 1)
		wrappedErrs <- Try[A]{Error: err}
		close(wrappedErrs)
	}

	if wrappedValues == nil {
		return wrappedErrs
	}
	if wrappedErrs == nil {
		return wrappedValues
	}
	return chans.Merge(wrappedErrs, wrappedValues)
}

func WrapAsync[A any](values <-chan A, errs <-chan error) <-chan Try[A] {
	if values == nil && errs == nil {
		return nil
	}

	wrappedValues := chans.Map(values, 1, func(x A) Try[A] {
		return Try[A]{V: x}
	})

	wrappedErrs := chans.Map(errs, 1, func(err error) Try[A] {
		return Try[A]{Error: err}
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
