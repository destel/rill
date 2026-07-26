package rill

import (
	"iter"
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
func FromSeq2[A any](seq iter.Seq2[A, error]) <-chan Try[A] {
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
// If the caller stops iteration early using break or return, ToSeq2 drains the
// remaining input in the background, like blocking functions such as [ForEach].
func ToSeq2[A any](in <-chan Try[A]) iter.Seq2[A, error] {
	return func(yield func(A, error) bool) {
		defer Discard(in)
		for x := range in {
			if !yield(x.Value, x.Error) {
				return
			}
		}
	}
}
