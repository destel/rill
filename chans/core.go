package chans

func MapAndFilter[A, B any](in <-chan A, n int, f func(A) (B, bool)) <-chan B {
	if in == nil {
		return nil
	}

	out := make(chan B)

	loop(in, out, n, func(x A) {
		y, keep := f(x)
		if keep {
			out <- y
		}
	})

	return out
}

func OrderedMapAndFilter[A, B any](in <-chan A, n int, f func(A) (B, bool)) <-chan B {
	if in == nil {
		return nil
	}

	out := make(chan B)
	orderedLoop(in, out, n, func(a A, canWrite <-chan struct{}) {
		y, keep := f(a)
		<-canWrite
		if keep {
			out <- y
		}
	})

	return out
}

func Map[A, B any](in <-chan A, n int, f func(A) B) <-chan B {
	return MapAndFilter(in, n, func(a A) (B, bool) {
		return f(a), true
	})
}

func OrderedMap[A, B any](in <-chan A, n int, f func(A) B) <-chan B {
	return OrderedMapAndFilter(in, n, func(a A) (B, bool) {
		return f(a), true
	})
}

func Filter[A any](in <-chan A, n int, f func(A) bool) <-chan A {
	return MapAndFilter(in, n, func(a A) (A, bool) {
		return a, f(a)
	})
}

func OrderedFilter[A any](in <-chan A, n int, f func(A) bool) <-chan A {
	return OrderedMapAndFilter(in, n, func(a A) (A, bool) {
		return a, f(a)
	})
}

func FlatMap[A, B any](in <-chan A, n int, f func(A) <-chan B) <-chan B {
	if in == nil {
		return nil
	}

	out := make(chan B)

	loop(in, out, n, func(a A) {
		for b := range f(a) {
			out <- b
		}
	})

	return out
}

func OrderedFlatMap[A, B any](in <-chan A, n int, f func(A) <-chan B) <-chan B {
	if in == nil {
		return nil
	}

	out := make(chan B)
	orderedLoop(in, out, n, func(a A, canWrite <-chan struct{}) {
		bb := f(a)
		<-canWrite
		for b := range bb {
			out <- b
		}
	})

	return out
}

// blocking
// todo: explain that if false has been returned for item[i] that it's guranteed that function would have been called for all previous items
func ForEach[A any](in <-chan A, n int, f func(A) bool) {
	if n == 1 {
		for a := range in {
			if !f(a) {
				break
			}
		}

		return
	}

	in, doBreak := breakable(in)
	done := make(chan struct{})

	loop(in, done, n, func(a A) {
		if !f(a) {
			doBreak()
		}
	})

	<-done
}
