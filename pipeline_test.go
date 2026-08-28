package rill

import (
	"context"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/destel/rill/internal/th"
)

func isPalindrome(s string) bool {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		if s[i] != s[j] {
			return false
		}
	}
	return true
}

func TestPipelines(t *testing.T) {

	// Count the palindromes among the first 10000 numbers,
	// grouped by digit count.
	th.RunSynctest(t, "basic", func(t *testing.T) {
		numbers := Generate(func(send func(int), sendError func(error)) {
			for i := range 10000 {
				send(i)
			}
		})

		strs := Map(numbers, 10, func(x int) (string, error) {
			return strconv.Itoa(x), nil
		})

		palindromes := Filter(strs, 10, func(x string) (bool, error) {
			return isPalindrome(x), nil
		})

		counts, err := MapReduce(palindromes,
			2, func(s string) (int, int, error) {
				return len(s), 1, nil
			},
			10, func(x, y int) (int, error) {
				return x + y, nil
			},
		)

		th.ExpectNoError(t, err)
		th.ExpectMap(t, counts, map[int]int{
			1: 10,
			2: 9,
			3: 90,
			4: 90,
		})
	})

	// Find the first palindromic number greater than 123456.
	th.RunSynctest(t, "settlement", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// shared state, mutated with atomics by the stages
		var totalCalls int64

		// an infinite source: generates numbers until the context is canceled
		numbers := Generate(func(send func(int), sendError func(error)) {
			for i := 123456; ctx.Err() == nil; i++ {
				send(i)
			}
		})

		strs := OrderedMap(numbers, 10, func(x int) (string, error) {
			atomic.AddInt64(&totalCalls, 1)
			return strconv.Itoa(x), nil
		})

		palindromes := OrderedFilter(strs, 10, func(x string) (bool, error) {
			atomic.AddInt64(&totalCalls, 1)
			return isPalindrome(x), nil
		})

		settled, opt := Settlement()
		res, _, err := First(palindromes, opt)

		// stop the source from generating more numbers
		cancel()

		th.ExpectNoError(t, err)
		th.ExpectValue(t, res, "124421")

		// wait for the pipeline to settle (no more callbacks)
		<-settled

		// shared state is now safe to read without atomics
		th.ExpectNoRace(totalCalls)
	})
}
