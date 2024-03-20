package rill

import (
	"github.com/destel/rill/chans"
)

// Drain consumes and discards all items from an input channel, blocking until the channel is closed
func Drain[A any](in <-chan A) {
	chans.Drain(in)
}

// DrainNB is a non-blocking version of [Drain].
func DrainNB[A any](in <-chan A) {
	chans.DrainNB(in)
}

// Buffer takes a channel of items and returns a buffered channel of exact same items in the same order.
// This is useful when you want to write to the input channel without blocking the writer.
//
// Typical use case would look like
//
//	ids = Buffer(ids, 100)
//	// Now up to 100 ids can be buffered if subsequent stages of the pipeline are slow
func Buffer[A any](in <-chan A, n int) <-chan A {
	return chans.Buffer(in, n)
}

// FromSlice converts a slice into a channel.
func FromSlice[A any](slice []A) <-chan Try[A] {
	out := make(chan Try[A], len(slice))
	for _, a := range slice {
		out <- Try[A]{V: a}
	}
	close(out)
	return out
}

// ToSlice converts a channel into a slice.
// Conversion stops at the first error encountered.
// In case of an error, ToSlice ensures the input channel is drained to avoid goroutine leaks,
func ToSlice[A any](in <-chan Try[A]) ([]A, error) {
	var res []A

	for x := range in {
		if err := x.Error; err != nil {
			chans.DrainNB(in)
			return res, err
		}
		res = append(res, x.V)
	}

	return res, nil
}
