package rill

// Try is a container for a value or an error
type Try[A any] struct {
	Value A
	Error error
}

// Wrap converts a regular channel of items into a channel of items wrapped in a [Try] container.
// Additionally, this function can also take an error, which will be added to the output channel.
// Either the input channel or the error can be nil, but not both simultaneously.
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
			out <- Try[A]{Value: x}
		}
	}()

	return out
}

// WrapAsync is similar to [Wrap], but instead of single error, it can take a channel of errors,
// that all will be added to the output channel.
func WrapAsync[A any](values <-chan A, errs <-chan error) <-chan Try[A] {
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

// Unwrap converts a channel of [Try] containers into a channel of values and a channel of errors.
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
				out <- x.Value
			}
		}
	}()

	return out, errs
}
