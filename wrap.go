package rill

// Try holds either a value of type A or an error. When Error is
// non-nil, Value is meaningless.
type Try[A any] struct {
	Value A
	Error error
}

// Stream is a type alias for a receive-only channel of [Try] structs.
// Using it is optional, but improves readability.
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

// Wrap converts a value-error pair into a [Try]. If err is not nil,
// Wrap returns an error item and ignores value.
//
// This signature allows concise wrapping of functions that return a
// value and an error:
//
//	item := rill.Wrap(strconv.Atoi("42"))
func Wrap[A any](value A, err error) Try[A] {
	if err != nil {
		return Try[A]{Error: err}
	}
	return Try[A]{Value: value}
}

// FromSlice converts a slice into a stream.
// If err is not nil, it is added to the end of the stream.
//
// Modifying the slice before the stream is fully consumed is a data
// race.
//
// This signature allows concise wrapping of functions that return a
// slice and an error. FromSlice assumes that a non-empty slice along
// with an error is a partial result, and preserves both.
//
//	stream := rill.FromSlice(someFunc())
func FromSlice[A any](slice []A, err error) <-chan Try[A] {
	const maxBufferSize = 64

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

// ToSlice collects the stream's values into a slice. When ToSlice
// encounters an error, it immediately returns that error along with the
// partial slice. Otherwise, it consumes the stream to the end and
// returns a slice of all values.
//
// See the package documentation for the behaviors that all sinks share.
func ToSlice[A any](in <-chan Try[A], options ...SinkOption) ([]A, error) {
	defer Discard(in, options...)

	var res []A
	for x := range in {
		if err := x.Error; err != nil {
			return res, err
		}
		res = append(res, x.Value)
	}
	return res, nil
}

// FromChan converts a regular channel into a stream. If err is not nil,
// FromChan returns a stream with only that error and ignores values.
// Otherwise, values are forwarded to the output as they arrive, and the
// output is closed once the input is exhausted.
//
// A nil input is never exhausted, so the output never closes.
//
// This signature allows concise wrapping of functions that return a
// channel and an error. FromChan assumes a non-nil error means
// someFunc() could not construct the channel.
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

// FromChans converts separate value and error channels into a single
// stream. Values and errors are forwarded to the output as they arrive,
// and nil errors are skipped. The output is closed only when both
// inputs are exhausted.
//
// A nil input is never exhausted, so the output never closes.
//
// This signature allows concise wrapping of functions that return two
// channels. FromChans assumes someFunc() returns two channels that
// eventually close.
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

// ToChans splits the stream into two channels: one for values and one
// for errors. It returns immediately, forwards each item to the
// appropriate channel as it arrives, and closes both channels once the
// input is exhausted.
//
// The channels must be consumed concurrently to avoid a deadlock.
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

// Generate is a shorthand for creating streams: it manages the
// goroutine and channel lifecycle. Inside f, send writes a value to the
// stream, and sendError writes an error unless it is nil.
//
//	stream := rill.Generate(func(send func(int), sendError func(error)) {
//		for i := 0; i < 100; i++ {
//			send(i)
//		}
//		sendError(someError)
//	})
//
// The same stream without Generate:
//
//	stream := make(chan rill.Try[int])
//	go func() {
//		defer close(stream)
//		for i := 0; i < 100; i++ {
//			stream <- rill.Try[int]{Value: i}
//		}
//		stream <- rill.Try[int]{Error: someError}
//	}()
func Generate[A any](f func(send func(A), sendError func(error))) <-chan Try[A] {
	validateNilFunc(f == nil)

	out := make(chan Try[A])
	go func() {
		defer close(out)

		send := func(a A) {
			out <- Try[A]{Value: a}
		}
		sendError := func(err error) {
			if err != nil {
				out <- Try[A]{Error: err}
			}
		}

		f(send, sendError)
	}()
	return out
}
