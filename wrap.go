package rill

// Try is a container holding a value of type A or an error
type Try[A any] struct {
	Value A
	Error error
}

// Wrap converts a value and/or error into a [Try] container.
// It's a convenience function to avoid creating a [Try] container manually and benefit from type inference.
//
// Such function signature also allows concise wrapping of functions that return a value and an error:
//
//	item := rill.Wrap(strconv.ParseInt("42"))
func Wrap[A any](value A, err error) Try[A] {
	return Try[A]{Value: value, Error: err}
}

// FromSlice converts a slice into a stream.
// If err is not nil function returns a stream with a single error.
//
// Such function signature allows concise wrapping of functions that return a slice and an error:
//
//	stream := rill.FromSlice(someFunc())
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

// ToSlice converts an input stream into a slice.
//
// This is a blocking ordered function that processes items sequentially.
// See the package documentation for more information on blocking ordered functions and error handling.
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

// FromChan converts a regular channel into a stream.
// Additionally, this function can take an error, that will be added to the output stream alongside the values.
// Either argument can be nil, in which case it is ignored. If both arguments are nil, the function returns nil.
//
// Such function signature allows concise wrapping of functions that return a channel and an error:
//
//	stream := rill.FromChan(someFunc())
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

// FromChans converts a regular channel into a stream.
// Additionally, this function can take a channel of errors, which will be added to
// the output stream alongside the values.
// Either argument can be nil, in which case it is ignored. If both arguments are nil, the function returns nil.
//
// Such function signature allows concise wrapping of functions that return two channels:
//
//	stream := rill.FromChans(someFunc())
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

// ToChans splits an input stream into two channels: one for values and one for errors.
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
