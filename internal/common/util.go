package common

func Drain[A any](in <-chan A) {
	for range in {
	}
}

func DrainNB[A any](in <-chan A) {
	go Drain(in)
}

func Buffer[A any](in <-chan A, n int) <-chan A {
	// we use n-1 since 1 additional item is held on the stack (x variable)
	out := make(chan A, n-1)

	go func() {
		defer close(out)
		for x := range in {
			out <- x
		}
	}()

	return out
}
