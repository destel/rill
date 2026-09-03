package rill

import (
	"iter"
	"sync"
)

// FromSeq converts an iterator into a stream.
// If err is not nil, the function ignores the passed seq and returns a stream with a single error.
//
// Such function signature allows concise wrapping of functions that return an
// iterator and an error:
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
// For pairs with a non-nil error, FromSeq2 emits an error item and ignores the value.
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

// ToSeq2 converts an input stream into an iterator of value-error pairs.
// Errors are yielded as ordinary pairs and do not stop the iteration;
// handle them inside the loop.
//
// The returned iterator is single-use. If the caller stops iteration early
// using break or return, ToSeq2 drains the remaining input in the background
// to settle the pipeline.
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
