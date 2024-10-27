//go:build go1.23

package rill_test

import (
	"fmt"
	"slices"

	"github.com/destel/rill"
)

func ExampleFromSeq() {
	// Start with an iterator that yields numbers from 1 to 10
	numbersSeq := slices.Values([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

	// Convert the iterator into a stream
	numbers := rill.FromSeq(numbersSeq, nil)

	// Transform each number
	// Concurrency = 3
	results := rill.Map(numbers, 3, func(x int) (int, error) {
		return doSomethingWithNumber(x), nil
	})

	printStream(results)
}

func ExampleFromSeq2() {
	// Create an iter.Seq2 iterator that yields numbers from 1 to 10
	numberSeq := func(yield func(int, error) bool) {
		for i := 1; i <= 10; i++ {
			if !yield(i, nil) {
				return
			}
		}
	}

	// Convert the iterator into a stream
	numbers := rill.FromSeq2(numberSeq)

	// Transform each number
	// Concurrency = 3
	results := rill.Map(numbers, 3, func(x int) (int, error) {
		return doSomethingWithNumber(x), nil
	})

	printStream(results)
}

func ExampleToSeq2() {
	// Convert a slice of numbers into a stream
	numbers := rill.FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, nil)

	// Transform each number
	// Concurrency = 3
	results := rill.Map(numbers, 3, func(x int) (int, error) {
		return doSomethingWithNumber(x), nil
	})

	// Convert the stream into an iterator and use for-range to print the results
	for val, err := range rill.ToSeq2(results) {
		if err != nil {
			fmt.Println("Error:", err)
			break // cleanup is done regardless of early exit
		}
		fmt.Printf("%+v\n", val)
	}
}
