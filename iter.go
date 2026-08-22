package rill

import (
	"iter"
	"sync"
)

// FromSeq converts an iterator into a stream. If err is not nil,
// FromSeq returns a stream with only that error and ignores seq.
// Otherwise, the values of seq are forwarded to the output, and the
// output is closed once seq ends.
//
// This signature allows concise wrapping of functions that return an
// iterator and an error. FromSeq assumes a non-nil error means
// someFunc() could not construct the iterator.
//
//	stream := rill.FromSeq(someFunc())
func FromSeq[A any](seq iter.Seq[A], err error) <-chan Try[A] {
	if err != nil {
		out := make(chan Try[A], 1)
		out <- Try[A]{Error: err}
		close(out)
		return out
	}

	validateNilFunc(seq == nil)

	out := make(chan Try[A])
	go func() {
		for val := range seq {
			out <- Wrap(val, nil)
		}
		close(out)
	}()
	return out
}

// FromSeq2 converts an iterator of value-error pairs into a stream.
// Each pair becomes one item. For pairs with a non-nil error, FromSeq2
// emits an error item and ignores the value. The output is closed once
// seq ends.
func FromSeq2[A any](seq iter.Seq2[A, error]) <-chan Try[A] {
	validateNilFunc(seq == nil)

	out := make(chan Try[A])
	go func() {
		for val, err := range seq {
			out <- Wrap(val, err)
		}
		close(out)
	}()
	return out
}

// ToSeq2 converts the stream into an iterator of value-error pairs,
// typically consumed with a for-range loop. Pairs are yielded until the
// stream is exhausted or the loop exits with break or return. Error
// items do not stop the iteration: they are yielded as ordinary pairs.
//
// On an early exit, ToSeq2 discards the rest of the stream in the
// background, the same way sinks do.
//
// The returned iterator is single-use and must be ranged for the
// pipeline to settle. If it is never ranged, the input is never drained.
func ToSeq2[A any](in <-chan Try[A], options ...SinkOption) iter.Seq2[A, error] {
	// Unlike other sinks, ToSeq2 opens the options at the call site: its work
	// happens while the iterator is ranged, which can be arbitrarily far from
	// here.
	opts := collectSinkOptions(options)

	// The options are opened once, so they must also be settled once.
	var once sync.Once

	return func(yield func(A, error) bool) {
		endReached := false

		defer once.Do(func() {
			if endReached {
				opts.settle()
				return
			}

			go func() {
				Drain(in)
				opts.settle()
			}()
		})

		for x := range in {
			if !yield(x.Value, x.Error) {
				return
			}
		}

		endReached = true
	}
}
