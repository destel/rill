package rill

import (
	"fmt"

	"github.com/destel/rill/internal/core"
)

// Drain consumes and discards all items from an input channel, blocking until the channel is closed.
func Drain[A any](in <-chan A) {
	core.Drain(in)
}

// Discard is a non-blocking function that discards all items from an input channel.
func Discard[A any](in <-chan A) {
	core.Discard(in)
}

// DrainNB is a non-blocking version of [Drain]. It does draining in a separate goroutine.
//
// Deprecated: use [Discard] instead. DrainNB will be removed in v1.0.
func DrainNB[A any](in <-chan A) {
	core.Discard(in)
}

// Buffer takes a channel of items and returns a buffered channel of exact same items in the same order.
// This can be useful for preventing write operations on the input channel from blocking, especially if subsequent stages
// in the processing pipeline are slow.
// Up to size items can be buffered before back pressure is applied to the upstream producer.
//
// Typical usage of Buffer might look like this:
//
//	users := getUsers(ctx, companyID)
//	users = rill.Buffer(users, 100)
//	// Now work with the users channel as usual.
//	// Up to 100 users can be buffered if subsequent stages of the pipeline are slow.
func Buffer[A any](in <-chan A, size int) <-chan A {
	return core.Buffer(in, size)
}

func validateN(n int) {
	if n < 1 {
		panic(fmt.Sprintf("rill: n must be at least 1, got %d", n))
	}
}

func validateMinSize(size int, minSize int) {
	if size < minSize {
		panic(fmt.Sprintf("rill: size must be at least %d, got %d", minSize, size))
	}
}

func validateNilFunc(fIsNil bool) {
	if fIsNil {
		panic("rill: function must not be nil")
	}
}
