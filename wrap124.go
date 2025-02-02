//go:build go1.24

package rill

// Stream is a type alias for a channel of [Try] containers.
// This alias is optional, but it can make the code more readable.
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
