package rill

// Try is a container holding a value of type A or an error
type Try[A any] struct {
	Value A
	Error error
}

// Stream is a type alias for a channel of [Try] containers.
// This alias is optional, but it can make the code more readable.
//
// Before:
//
//	func StreamUsers() <-chan rill.Try[*User] {
//		...
//	}
//
// After:
//
//	func StreamUsers() rill.Stream[*User] {
//		...
//	}
type Stream[T any] = <-chan Try[T]

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
// If err is not nil, it is added to the end of the stream.
//
// Such function signature allows concise wrapping of functions that return a slice and an error:
//
//	stream := rill.FromSlice(someFunc())
func FromSlice[A any](slice []A, err error) <-chan Try[A] {
	const maxBufferSize = 512

	sendAll := func(out chan Try[A]) {
		for _, a := range slice {
			out <- Try[A]{Value: a}
		}
		if err != nil {
			out <- Try[A]{Error: err}
		}
		close(out)
	}

	size := len(slice)
	if err != nil {
		size++
	}

	if size <= maxBufferSize {
		out := make(chan Try[A], size)
		sendAll(out)
		return out
	}

	out := make(chan Try[A], maxBufferSize)
	go sendAll(out)
	return out
}

// ToSlice converts an input stream into a slice.
// If the stream contains errors, ToSlice returns the values that precede
// the first error, along with that error.
//
// This is a blocking ordered function that processes items sequentially.
// See the package documentation for more information on blocking ordered functions and error handling.
func ToSlice[A any](in <-chan Try[A]) ([]A, error) {
	var res []A

	for x := range in {
		if err := x.Error; err != nil {
			Discard(in)
			return res, err
		}
		res = append(res, x.Value)
	}

	return res, nil
}

// FromChan converts a regular channel into a stream.
// If err is not nil, the function ignores the passed values and returns a stream with a single error.
//
// Such function signature allows concise wrapping of functions that return a channel and an error:
//
//	stream := rill.FromChan(someFunc())
func FromChan[A any](values <-chan A, err error) <-chan Try[A] {
	if err != nil {
		out := make(chan Try[A], 1)
		out <- Try[A]{Error: err}
		close(out)
		return out
	}
	if values == nil {
		return nil
	}

	out := make(chan Try[A])
	go func() {
		defer close(out)
		for x := range values {
			out <- Try[A]{Value: x}
		}
	}()

	return out
}

// FromChans creates a stream from independent value and error channels.
// Items from both inputs are added to the output stream as they arrive, and nil
// errors are skipped.
// The output stream is closed only when both input channels are exhausted.
// In particular, if at least one input is nil, the output stream never closes.
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

		cntClosed := 0
		for cntClosed < 2 {
			select {
			case err, ok := <-errs:
				if !ok {
					cntClosed++
					errs = nil
					continue
				}
				if err != nil {
					out <- Try[A]{Error: err}
				}

			case v, ok := <-values:
				if !ok {
					cntClosed++
					values = nil
					continue
				}
				out <- Try[A]{Value: v}
			}
		}
	}()

	return out
}

// ToChans splits an input stream into two channels: one for values and one for errors.
// Both output channels are closed when the input stream is exhausted.
// They must be consumed concurrently to avoid deadlocks.
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

// Generate is a shorthand for creating streams.
// It provides a more ergonomic way of sending both values and errors to a stream, manages goroutine and channel lifecycle.
//
//	stream := rill.Generate(func(send func(int), sendErr func(error)) {
//		for i := 0; i < 100; i++ {
//			send(i)
//		}
//		sendErr(someError)
//	})
//
// Here's how the same code would look without Generate:
//
//	stream := make(chan rill.Try[int])
//	go func() {
//		defer close(stream)
//		for i := 0; i < 100; i++ {
//			stream <- rill.Try[int]{Value: i}
//		}
//		stream <- rill.Try[int]{Error: someError}
//	}()
func Generate[A any](f func(send func(A), sendErr func(error))) <-chan Try[A] {
	out := make(chan Try[A])
	go func() {
		defer close(out)

		send := func(a A) {
			out <- Try[A]{Value: a}
		}
		sendErr := func(err error) {
			out <- Try[A]{Error: err}
		}

		f(send, sendErr)
	}()
	return out
}
