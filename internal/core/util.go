package core

func Drain[A any](in <-chan A) {
	for range in {
	}
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

func SendNB[A any](out chan<- A, x A) bool {
	select {
	case out <- x:
		return true
	default:
		return false
	}
}
