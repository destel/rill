//go:build go1.23

package rill_test

import (
	"fmt"
	"slices"

	"github.com/destel/rill"
)

func ExampleFromSeq() {
	numbersSeq := slices.Values([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

	numbers := rill.FromSeq(numbersSeq, nil)

	squares := rill.Map(numbers, 1, func(x int) (int, error) {
		return x * x, nil
	})

	printStream(squares)
}

func ExampleFromSeq2() {
	numberSeq := func(yield func(int, error) bool) {
		for i := 1; i <= 10; i++ {
			if !yield(i, nil) {
				return
			}
		}
	}

	numbers := rill.FromSeq2(numberSeq)

	squares := rill.Map(numbers, 1, func(x int) (int, error) {
		return x * x, nil
	})

	printStream(squares)
}

func ExampleToSeq2() {
	numbers := rill.FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, nil)

	squares := rill.Map(numbers, 1, func(x int) (int, error) {
		return x * x, nil
	})

	for val, err := range rill.ToSeq2(squares) {
		if err != nil {
			fmt.Println("Error:", err)
			break // cleanup and early exit
		}
		fmt.Printf("%+v\n", val)
	}
}
