//go:build go1.23
// +build go1.23

package rill

import (
	"iter"
)

// FromSeq converts a iterator into a stream.
// If err is not nil function returns a stream with a single error.
//
// Such function signature allows concise wrapping of functions that return an
// iterator and an error:
//
//	stream := rill.FromSeq(someFunc())
func FromSeq[A any](seq iter.Seq[A], err error) <-chan Try[A] {
	if seq == nil && err == nil {
		return nil
	}
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

// FromSeq2 converts a value-error pairs sequence into a stream.
func FromSeq2[A any](seq iter.Seq2[A, error]) <-chan Try[A] {
	if seq == nil {
		return nil
	}

	out := make(chan Try[A])
	go func() {
		for val, err := range seq {
			out <- Wrap(val, err)
		}
		close(out)
	}()
	return out
}

// ToSeq2 converts an input stream into a sequence of value-error paris.
//
// This is a blocking ordered function that processes items sequentially. For
// error handling, ToSeq2 is different from ToSlice; it does not simply return
// the first encountered error. Instead, ToIterSeq will iterate all value-error
// paris, allowing the client to decide when to stop.
func ToSeq2[A any](in <-chan Try[A]) iter.Seq2[A, error] {
	return func(yield func(A, error) bool) {
		defer DrainNB(in)
		for x := range in {
			if !yield(x.Value, x.Error) {
				return
			}
		}
	}
}
