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

// FromSlice converts a slice into a channel of [Try] containers.
// If err is not nil function returns a single [Try] container with the error.
func FromSlice[A any](slice []A, err error) <-chan Try[A] {
	if err != nil {
		out := make(chan Try[A], 1)
		out <- Try[A]{Error: err}
		close(out)
		return out
	}

	out := make(chan Try[A], len(slice))
	for _, a := range slice {
		out <- Try[A]{Value: a}
	}
	close(out)
	return out
}

// ToSlice converts a channel of [Try] containers into a slice of values and an error.
// It's an inverse of [FromSlice]. The function blocks until the whole channel is processed or an error is encountered.
// In case of an error leading to early termination, ToSlice ensures the input channel is drained to avoid goroutine leaks.
func ToSlice[A any](in <-chan Try[A]) ([]A, error) {
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

// FromChan converts a regular channel into a channel of values wrapped in a [Try] container.
// Additionally, this function can take an error, that will be added to the output channel alongside the values.
// If both values and error are nil, the function returns nil.
func FromChan[A any](values <-chan A, err error) <-chan Try[A] {
	if values == nil && err == nil {
		return nil
	}

	out := make(chan Try[A])
	go func() {
		defer close(out)

		// error goes first
		if err != nil {
			out <- Try[A]{Error: err}
		}

		for x := range values {
			out <- Try[A]{Value: x}
		}
	}()

	return out
}

// FromChans converts a regular channel into a channel of values wrapped in a [Try] container.
// Additionally, this function can take a channel of errors, which will be added to
// the output channel alongside the values.
// If both values and errors are nil, the function returns nil.
func FromChans[A any](values <-chan A, errs <-chan error) <-chan Try[A] {
	if values == nil && errs == nil {
		return nil
	}

	out := make(chan Try[A])

	go func() {
		defer close(out)
		for {
			select {
			case err, ok := <-errs:
				if ok {
					if err != nil {
						out <- Try[A]{Error: err}
					}
				} else {
					errs = nil
					if values == nil && errs == nil {
						return
					}
				}

			case v, ok := <-values:
				if ok {
					out <- Try[A]{Value: v}
				} else {
					values = nil
					if values == nil && errs == nil {
						return
					}
				}
			}
		}
	}()

	return out
}

// ToChans splits a channel of [Try] containers into a channel of values and a channel of errors.
// It's an inverse of [FromChans]. Returns two nil channels if the input is nil.
func ToChans[A any](in <-chan Try[A]) (<-chan A, <-chan error) {
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
