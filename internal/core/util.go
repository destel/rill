package core

func Drain[A any](in <-chan A) {
	for range in {
	}
}

func Discard[A any](in <-chan A) {
	go Drain(in)
}

func Buffer[A any](in <-chan A, size int) <-chan A {
	// we use size-1 since 1 additional item is held on the stack (x variable)
	out := make(chan A, size-1)

	go func() {
		defer close(out)
		for x := range in {
			out <- x
		}
	}()

	return out
}
