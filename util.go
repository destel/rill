package rill

import (
	"fmt"

	"github.com/destel/rill/internal/core"
)

// Drain consumes and discards all items of the channel, blocking until
// it is exhausted.
func Drain[A any](in <-chan A) {
	core.Drain(in)
}

// Discard returns immediately, then consumes and discards all items of
// the channel in the background.
func Discard[A any](in <-chan A) {
	core.Discard(in)
}

// DrainNB is a non-blocking version of [Drain].
//
// Deprecated: use [Discard] instead, which is identical. DrainNB will
// be removed in v1.0.
func DrainNB[A any](in <-chan A) {
	core.Discard(in)
}

// Buffer forwards all input items to a new channel with a capacity of
// size. It returns immediately and closes the output once the input is
// exhausted.
//
//	users := getUsers(ctx, companyID)
//	users = rill.Buffer(users, 100)
//	// Up to 100 users can be buffered if subsequent stages of the
//	// pipeline are slow.
func Buffer[A any](in <-chan A, size int) <-chan A {
	validateMinSize(size, 0)
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
