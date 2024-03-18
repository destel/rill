package chans

func Drain[A any](in <-chan A) {
	for range in {
	}
}

func DrainNB[A any](in <-chan A) {
	go Drain(in)
}

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

func FromSlice[A any](slice []A) <-chan A {
	out := make(chan A, len(slice))
	for _, a := range slice {
		out <- a
	}
	close(out)
	return out
}

func ToSlice[A any](in <-chan A) []A {
	var res []A
	for x := range in {
		res = append(res, x)
	}
	return res
}
