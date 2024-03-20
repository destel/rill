package rill

import "github.com/destel/rill/internal/core"

// Drain consumes and discards all items from an input channel, blocking until the channel is closed
func Drain[A any](in <-chan A) {
	core.Drain(in)
}

// DrainNB is a non-blocking version of [Drain].
func DrainNB[A any](in <-chan A) {
	core.DrainNB(in)
}

// Buffer takes a channel of items and returns a buffered channel of exact same items in the same order.
// This is useful when you want to write to the input channel without blocking the writer.
//
// Typical use case would look like
//
//	ids = Buffer(ids, 100)
//	// Now up to 100 ids can be buffered if subsequent stages of the pipeline are slow
func Buffer[A any](in <-chan A, n int) <-chan A {
	return core.Buffer(in, n)
}
