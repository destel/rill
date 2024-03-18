package chans

// Drain consumes and discards all items from an input channel, blocking until the channel is closed
func Drain[A any](in <-chan A) {
	for range in {
	}
}

// DrainNB is a non-blocking version of Drain.
func DrainNB[A any](in <-chan A) {
	go Drain(in)
}

// Buffer takes a channel of items and returns a buffered channel of exact same items in the same order.
// This is useful when you want to write to the input channel without blocking the writer.
func Buffer[A any](in <-chan A, n int) <-chan A {
	// we use n+1 since 1 additional is held on the stack (x variable)
	out := make(chan A, n-1)

	go func() {
		defer close(out)
		for x := range in {
			out <- x
		}
	}()

	return out
}

// FromSlice converts a slice into a channel.
func FromSlice[A any](slice []A) <-chan A {
	out := make(chan A, len(slice))
	for _, a := range slice {
		out <- a
	}
	close(out)
	return out
}

// ToSlice converts a channel into a slice.
func ToSlice[A any](in <-chan A) []A {
	var res []A
	for x := range in {
		res = append(res, x)
	}
	return res
}
