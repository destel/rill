package core

func Drain[A any](in <-chan A) {
	for range in {
	}
}

func Discard[A any](in <-chan A) {
	go Drain(in)
}

func Buffer[A any](in <-chan A, size int) <-chan A {
	if in == nil {
		return nil
	}

	out := make(chan A, size)

	go func() {
		defer close(out)
		for x := range in {
			out <- x
		}
	}()

	return out
}
