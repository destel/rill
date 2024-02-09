package echans

import "github.com/destel/rill/chans"

type tuple2[A, B any] struct {
	V1 A
	V2 B
}

func makeTuple2[A, B any](v1 A, v2 B) tuple2[A, B] {
	return tuple2[A, B]{V1: v1, V2: v2}
}

// takes chan of tuples and returns chan of first elements of each tuple
func tuple2chanExtract1[A, B any](in <-chan tuple2[A, B]) <-chan A {
	return chans.Map(in, 1, func(a tuple2[A, B]) A {
		return a.V1
	})
}
