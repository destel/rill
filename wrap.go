package rill

// Try is a container for a value or an error
type Try[A any] struct {
	Value A
	Error error
}

// Wrap converts a value and/or error into a [Try] container.
// It's a convenience function to avoid creating a [Try] container manually and benefit from type inference.
func Wrap[A any](value A, err error) Try[A] {
	return Try[A]{Value: value, Error: err}
}

// WrapSlice converts a slice into a channel of [Try] containers.
func WrapSlice[A any](slice []A) <-chan Try[A] {
	out := make(chan Try[A], len(slice))
	for _, a := range slice {
		out <- Try[A]{Value: a}
	}
	close(out)
	return out
}

// UnwrapToSlice converts a channel of [Try] containers into a slice of values and an error.
// In a way it's an inverse of [WrapSlice], but it stops on the first error and returns it
// Also in case of an error, UnwrapToSlice ensures the input channel is drained to avoid goroutine leaks.
func UnwrapToSlice[A any](in <-chan Try[A]) ([]A, error) {
	var res []A

	for x := range in {
		if err := x.Error; err != nil {
			DrainNB(in)
			return res, err
		}
		res = append(res, x.Value)
	}

	return res, nil
}

// WrapChan converts a regular channel into a channel of values wrapped in a [Try] container.
// Additionally, this function can take an error, which will be added to the output channel alongside the values.
// Either the input channel or the error can be nil, but not both simultaneously.
func WrapChan[A any](values <-chan A, err error) <-chan Try[A] {
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
			out <- Try[A]{Value: x}
		}
	}()

	return out
}

// WrapChanAndErrs takes channel of values and a channel of errors and merges them into a single channel of [Try] containers.
// Either of the input channels can be nil, but not both simultaneously.
func WrapChanAndErrs[A any](values <-chan A, errs <-chan error) <-chan Try[A] {
	if values == nil && errs == nil {
		return nil
	}

	out := make(chan Try[A])

	go func() {
		defer close(out)
		for {
			select {
			case err, ok := <-errs:
				if !ok {
					errs = nil
					if values == nil && errs == nil {
						return
					}
					continue
				}

				if err != nil {
					out <- Try[A]{Error: err}
				}

			case x, ok := <-values:
				if !ok {
					values = nil
					if values == nil && errs == nil {
						return
					}
					continue
				}

				out <- Try[A]{Value: x}
			}
		}
	}()

	return out
}

// UnwrapToChanAndErrs converts a channel of [Try] containers into a channel of values and a channel of errors.
// It's an inverse of [WrapChanAndErrs].
func UnwrapToChanAndErrs[A any](in <-chan Try[A]) (<-chan A, <-chan error) {
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
				out <- x.Value
			}
		}
	}()

	return out, errs
}
