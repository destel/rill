//go:build go1.23
// +build go1.23

package rill_test

import (
	"fmt"
	"iter"

	"github.com/destel/rill"
)

func ExampleToSeq2() {
	nums := rill.FromSeq2(genPositive(40))
	squares := rill.Map(nums, 4, func(x int) (int, error) {
		return x * x, nil
	})
	for val, err := range rill.ToSeq2(squares) {
		fmt.Println(val, err)
	}
}

func genPositive(to int) iter.Seq2[int, error] {
	return func(yield func(i int, err error) bool) {
		for i := 1; i <= to; i++ {
			if !yield(i, nil) {
				return
			}
		}
	}
}
