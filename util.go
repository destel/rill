package rill

import "github.com/destel/rill/internal/core"

// Drain consumes and discards all items from an input channel, blocking until the channel is closed.
func Drain[A any](in <-chan A) {
	core.Drain(in)
}

// DrainNB is a non-blocking version of [Drain]. Is does draining in a separate goroutine.
func DrainNB[A any](in <-chan A) {
	core.DrainNB(in)
}

// Buffer takes a channel of items and returns a buffered channel of exact same items in the same order.
// This can be useful for preventing write operations on the input channel from blocking, especially if subsequent stages
// in the processing pipeline are slow.
// Buffering allows up to n items to be held in memory before back pressure is applied to the upstream producer.
//
// Typical usage of Buffer might look like this:
//
//	users := getUsers(ctx, companyID)
//	users = rill.Buffer(users, 100)
//	// Now work with the users channel as usual.
//	// Up to 100 users can be buffered if subsequent stages of the pipeline are slow.
func Buffer[A any](in <-chan A, n int) <-chan A {
	return core.Buffer(in, n)
}
