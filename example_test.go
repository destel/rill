package rill_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/rand"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/destel/rill"
	api "github.com/destel/rill/mockapi"
)

// --- Package examples ---

// This example demonstrates a rill pipeline that fetches users from an API, activates them, and saves
// the changes back. Each step runs concurrently with its own concurrency limit, and errors from both
// are handled in one place. On the first error, [ForEach] cancels the context, waits until nothing is
// running anymore, and returns that error.
func Example() {
	ctx, scope := rill.WithContext(context.Background())

	// Convert a slice into a channel
	ids := rill.FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, nil)

	// Read users from the API. Concurrency = 3
	users := rill.Map(ids, 3, func(id int) (*api.User, error) {
		return api.GetUser(ctx, id)
	})

	// Process users. Concurrency = 2
	err := rill.ForEach(users, 2, func(u *api.User) error {
		if u.IsActive {
			fmt.Printf("User %d is already active\n", u.ID)
			return nil
		}

		u.IsActive = true
		err := api.SaveUser(ctx, u)
		if err != nil {
			return err
		}

		fmt.Printf("User saved: %+v\n", u)
		return nil
	}, scope) // scope is a functional option

	// Nothing is running anymore; the context is canceled.
	// Handle the error (if any).
	fmt.Println("Error:", err)
}

// This example demonstrates a rill pipeline that fetches users from an API in batches, activates
// them, and saves the changes back. [Batch] groups individual IDs into slices, so users are fetched
// with one bulk API call per batch instead of one call per user.
func Example_batching() {
	ctx, scope := rill.WithContext(context.Background())

	// Convert a slice of user IDs into a channel
	ids := rill.FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}, nil)

	// Group IDs into batches of 5
	idBatches := rill.Batch(ids, 5, -1)

	// Bulk fetch users from the API. Concurrency = 3
	userBatches := rill.Map(idBatches, 3, func(ids []int) ([]*api.User, error) {
		return api.GetUsers(ctx, ids)
	})

	// Transform the stream of batches back into a flat stream of users
	users := rill.Unbatch(userBatches)

	// Process users. Concurrency = 2
	err := rill.ForEach(users, 2, func(u *api.User) error {
		if u.IsActive {
			fmt.Printf("User %d is already active\n", u.ID)
			return nil
		}

		u.IsActive = true
		err := api.SaveUser(ctx, u)
		if err != nil {
			return err
		}

		fmt.Printf("User saved: %+v\n", u)
		return nil
	}, scope)

	// Nothing is running anymore; the context is canceled.
	// Handle the error.
	fmt.Println("Error:", err)
}

// This example demonstrates how [Batch] can group independent operations happening in real time.
// The main function makes 100 concurrent calls to the UpdateUserTimestamp function, which looks
// normal at the call site: it takes a user ID, waits for the database to respond, and returns an
// error. Under the hood, a background worker uses rill to combine concurrent calls into bulk updates
// and send results back to the corresponding callers.
//
// Since calls happen at unpredictable times, waiting for a full batch can take arbitrarily long.
// To avoid this, [Batch] takes a timeout argument that limits how long each batch waits to fill.
// When the timeout expires, a partial batch is emitted.
func Example_realTimeBatching() {
	// Start the background worker
	go updateUserTimestampWorker()

	// Make 100 concurrent calls
	var wg sync.WaitGroup
	for id := 1; id <= 100; id++ {
		wg.Go(func() {
			if err := UpdateUserTimestamp(id); err != nil {
				fmt.Println("Error:", err)
			}
		})
	}
	wg.Wait()
}

func UpdateUserTimestamp(userID int) error {
	// Prepare a request to the worker.
	req := request{userID: userID, replyTo: make(chan error, 1)}

	// Send the request and wait for a reply.
	queue <- req
	err := <-req.replyTo
	return err
}

func updateUserTimestampWorker() {
	// Start with a stream of update requests
	requests := rill.FromChan(queue, nil)

	// Group up to 10 requests; when requests are sparse, add at most 20ms of latency
	requestBatches := rill.Batch(requests, 10, 20*time.Millisecond)

	// Send bulk updates to DB. At most 2 concurrent DB queries
	_ = rill.ForEach(requestBatches, 2, func(batch []request) error {
		// Create a slice of user IDs
		ids := make([]int, len(batch))
		for i, req := range batch {
			ids[i] = req.userID
		}

		// Do bulk update
		err := executeQuery("UPDATE users SET last_active_at = NOW() WHERE id IN (?)", ids)

		// Send result back to all callers in this batch
		for _, req := range batch {
			req.replyTo <- err
		}

		// Keep the pipeline running
		return nil
	})
}

// This type represents a single request to the worker
type request struct {
	userID  int
	replyTo chan error
}

// Queue of update requests
var queue = make(chan request)

// executeQuery simulates a database query
func executeQuery(query string, args ...any) error {
	simulateWork(100 * time.Millisecond)
	for _, arg := range args {
		query = strings.Replace(query, "?", fmt.Sprint(arg), 1)
	}
	fmt.Println("Executed:", query)
	return nil
}

// This example demonstrates how to check 1000 large files and find the first file containing a
// given string. Downloading files one by one is slow, while traditional concurrency patterns find
// the fastest match instead of the first one. [OrderedFilter] and [First] solve this while
// downloading and keeping in memory at most 5 files at a time. On the first match or error, [First]
// cancels the context, waits until nothing is running anymore, and returns the result.
func Example_orderPreservation() {
	ctx, scope := rill.WithContext(context.Background())

	// The string to search for in the downloaded files
	needle := []byte("26")

	// Generate a stream of URLs from file-0.txt to file-999.txt.
	// Stop generating URLs when the context is canceled
	urls := rill.Generate(func(send func(string), sendError func(error)) {
		for i := 0; i < 1000 && ctx.Err() == nil; i++ {
			send(fmt.Sprintf("https://example.com/file-%d.txt", i))
		}
	})

	// Download and process the files. Concurrency = 5
	matchedUrls := rill.OrderedFilter(urls, 5, func(url string) (bool, error) {
		fmt.Println("Downloading:", url)

		content, err := api.DownloadFile(ctx, url)
		if err != nil {
			return false, err
		}

		// Keep only URLs of files that contain the needle
		return bytes.Contains(content, needle), nil
	})

	// Get the first matched URL or error
	firstMatchedUrl, found, err := rill.First(matchedUrls, scope)

	// Nothing is running anymore; the context is canceled.
	// Handle the result.
	fmt.Println("Result:", firstMatchedUrl, found, err)
}

// This example demonstrates how [FlatMap] can remove a slow-source bottleneck. The API is slow and
// paginated, so users are streamed from several departments concurrently and merged into a single
// stream. There can be any number of departments, while FlatMap streams at most 3 at a time.
func Example_parallelStreaming() {
	ctx, scope := rill.WithContext(context.Background())

	// Start with a stream of department names
	departments := rill.FromSlice([]string{"IT", "Finance", "Marketing", "Support", "Engineering"}, nil)

	// Stream users from all departments concurrently.
	// At most 3 departments at the same time.
	users := rill.FlatMap(departments, 3, func(department string) <-chan rill.Try[*api.User] {
		return StreamUsers(ctx, api.UserQuery{Department: department})
	})

	// Print the users from the combined stream
	err := rill.ForEach(users, 1, func(user *api.User) error {
		fmt.Printf("%+v\n", user)
		return nil
	}, scope)

	// Nothing is running anymore; the context is canceled.
	// Handle the error.
	fmt.Println("Error:", err)
}

// StreamUsers streams users from a paginated API. It's a reusable streaming wrapper, useful both
// on its own and as part of larger pipelines.
func StreamUsers(ctx context.Context, query api.UserQuery) <-chan rill.Try[*api.User] {
	return rill.Generate(func(send func(*api.User), sendError func(error)) {
		for page := 0; ; page++ {
			query.Page = page

			users, err := api.ListUsers(ctx, query)
			if err != nil {
				sendError(err)
				return
			}

			if len(users) == 0 {
				break
			}

			for _, user := range users {
				send(user)
			}
		}
	})
}

// --- Function examples ---

func ExampleAll() {
	// Convert a slice of numbers into a stream
	numbers := rill.FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, nil)

	// Are all numbers prime?
	// Concurrency = 3
	ok, err := rill.All(numbers, 3, func(x int) (bool, error) {
		return isPrime(x), nil
	})

	fmt.Println("Result:", ok)
	fmt.Println("Error:", err)
}

func ExampleAny() {
	// Convert a slice of numbers into a stream
	numbers := rill.FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, nil)

	// Is there at least one prime number?
	// Concurrency = 3
	ok, err := rill.Any(numbers, 3, func(x int) (bool, error) {
		return isPrime(x), nil
	})

	fmt.Println("Result: ", ok)
	fmt.Println("Error: ", err)
}

// See the package-level examples for more realistic uses of Batch.
func ExampleBatch() {
	// Generate a stream of numbers 0 to 49, where a new number is emitted every 50ms
	numbers := rill.Generate(func(send func(int), sendError func(error)) {
		for i := range 50 {
			send(i)
			time.Sleep(50 * time.Millisecond)
		}
	})

	// Group numbers into batches of up to 5
	batches := rill.Batch(numbers, 5, 1*time.Second)

	printStream(batches)
}

func ExampleCatch() {
	// Convert a slice of strings into a stream
	strs := rill.FromSlice([]string{"1", "2", "3", "4", "5", "not a number 6", "7", "8", "9", "10"}, nil)

	// Convert strings to ints
	// Concurrency = 3
	ids := rill.Map(strs, 3, func(s string) (int, error) {
		simulateWork(500 * time.Millisecond)
		return strconv.Atoi(s)
	})

	// Catch and ignore number parsing errors
	// Concurrency = 2
	ids = rill.Catch(ids, 2, func(err error) error {
		if errors.Is(err, strconv.ErrSyntax) {
			return nil // Ignore this error
		}
		return err
	})

	// No error will be printed
	printStream(ids)
}

func ExampleOrderedCatch() {
	// Convert a slice of strings into a stream
	strs := rill.FromSlice([]string{"1", "2", "3", "4", "5", "not a number 6", "7", "8", "9", "10"}, nil)

	// Convert strings to ints
	// Concurrency = 3; Ordered
	ids := rill.OrderedMap(strs, 3, func(s string) (int, error) {
		simulateWork(500 * time.Millisecond)
		return strconv.Atoi(s)
	})

	// Catch and ignore number parsing errors
	// Concurrency = 2; Ordered
	ids = rill.OrderedCatch(ids, 2, func(err error) error {
		if errors.Is(err, strconv.ErrSyntax) {
			return nil // Ignore this error
		}
		return err
	})

	// No error will be printed
	printStream(ids)
}

func ExampleErr() {
	ctx := context.Background()

	// Convert a slice of users into a stream
	users := rill.FromSlice([]*api.User{
		{ID: 1, Name: "foo", Age: 25},
		{ID: 2, Name: "bar", Age: 30},
		{ID: 3}, // empty username is invalid
		{ID: 4, Name: "baz", Age: 35},
		{ID: 5, Name: "qux", Age: 26},
		{ID: 6, Name: "quux", Age: 27},
	}, nil)

	// Save users. Use struct{} as a result type
	// Concurrency = 2
	results := rill.Map(users, 2, func(user *api.User) (struct{}, error) {
		return struct{}{}, api.SaveUser(ctx, user)
	})

	// We only need to know if all users were saved successfully
	err := rill.Err(results)
	fmt.Println("Error:", err)
}

func ExampleFilter() {
	// Convert a slice of numbers into a stream
	numbers := rill.FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, nil)

	// Keep only prime numbers
	// Concurrency = 3
	primes := rill.Filter(numbers, 3, func(x int) (bool, error) {
		return isPrime(x), nil
	})

	printStream(primes)
}

func ExampleOrderedFilter() {
	// Convert a slice of numbers into a stream
	numbers := rill.FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, nil)

	// Keep only prime numbers
	// Concurrency = 3; Ordered
	primes := rill.OrderedFilter(numbers, 3, func(x int) (bool, error) {
		return isPrime(x), nil
	})

	printStream(primes)
}

func ExampleFilterMap() {
	// Convert a slice of numbers into a stream
	numbers := rill.FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, nil)

	// Keep only prime numbers and square them
	// Concurrency = 3
	squares := rill.FilterMap(numbers, 3, func(x int) (int, bool, error) {
		if !isPrime(x) {
			return 0, false, nil
		}

		return x * x, true, nil
	})

	printStream(squares)
}

func ExampleOrderedFilterMap() {
	// Convert a slice of numbers into a stream
	numbers := rill.FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, nil)

	// Keep only prime numbers and square them
	// Concurrency = 3
	squares := rill.OrderedFilterMap(numbers, 3, func(x int) (int, bool, error) {
		if !isPrime(x) {
			return 0, false, nil
		}

		return x * x, true, nil
	})

	printStream(squares)
}

func ExampleFirst() {
	// Convert a slice of numbers into a stream
	numbers := rill.FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, nil)

	// Keep only the numbers divisible by 4
	// Concurrency = 3; Ordered
	divisibleBy4 := rill.OrderedFilter(numbers, 3, func(x int) (bool, error) {
		return x%4 == 0, nil
	})

	// Get the first number divisible by 4
	first, ok, err := rill.First(divisibleBy4)

	fmt.Println("Result:", first, ok)
	fmt.Println("Error:", err)
}

func ExampleFlatMap() {
	// Convert a slice of numbers into a stream
	numbers := rill.FromSlice([]int{1, 2, 3, 4, 5}, nil)

	// Replace each number in the input stream with three strings
	// Concurrency = 2
	result := rill.FlatMap(numbers, 2, func(x int) <-chan rill.Try[string] {
		simulateWork(500 * time.Millisecond)

		return rill.FromSlice([]string{
			fmt.Sprintf("foo%d", x),
			fmt.Sprintf("bar%d", x),
			fmt.Sprintf("baz%d", x),
		}, nil)
	})

	printStream(result)
}

func ExampleOrderedFlatMap() {
	// Convert a slice of numbers into a stream
	numbers := rill.FromSlice([]int{1, 2, 3, 4, 5}, nil)

	// Replace each number in the input stream with three strings
	// Concurrency = 2; Ordered
	result := rill.OrderedFlatMap(numbers, 2, func(x int) <-chan rill.Try[string] {
		simulateWork(500 * time.Millisecond)

		return rill.FromSlice([]string{
			fmt.Sprintf("foo%d", x),
			fmt.Sprintf("bar%d", x),
			fmt.Sprintf("baz%d", x),
		}, nil)
	})

	printStream(result)
}

func ExampleForEach() {
	// Convert a slice of numbers into a stream
	numbers := rill.FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, nil)

	// Square each number and print the result
	// Concurrency = 3
	err := rill.ForEach(numbers, 3, func(x int) error {
		y := square(x)
		fmt.Println(y)
		return nil
	})

	// Handle errors
	fmt.Println("Error:", err)
}

func ExampleGenerate() {
	urls := rill.Generate(func(send func(string), sendError func(error)) {
		for i := range 10 {
			send(fmt.Sprintf("https://example.com/file-%d.txt", i))
		}
	})

	printStream(urls)
}

func ExampleGenerate_context() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Keep generating numbers until the context is canceled after 5s.
	numbers := rill.Generate(func(send func(int), sendError func(error)) {
		for i := 1; ctx.Err() == nil; i++ {
			send(i)
			time.Sleep(500 * time.Millisecond)
		}
	})

	printStream(numbers)
}

func ExampleMap() {
	// Convert a slice of numbers into a stream
	numbers := rill.FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, nil)

	// Transform each number
	// Concurrency = 3
	squares := rill.Map(numbers, 3, func(x int) (int, error) {
		return square(x), nil
	})

	printStream(squares)
}

func ExampleOrderedMap() {
	// Convert a slice of numbers into a stream
	numbers := rill.FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, nil)

	// Transform each number
	// Concurrency = 3; Ordered
	squares := rill.OrderedMap(numbers, 3, func(x int) (int, error) {
		return square(x), nil
	})

	printStream(squares)
}

func ExampleMapReduce() {
	var re = regexp.MustCompile(`\w+`)
	text := "Early morning brings early birds to the early market. Birds sing, the market buzzes, and the morning shines."

	// Convert a text into a stream of words
	words := rill.FromSlice(re.FindAllString(text, -1), nil)

	// Count the number of occurrences of each word
	mr, err := rill.MapReduce(words,
		// Map phase: Use the word as key and "1" as value
		// Concurrency = 3
		3, func(word string) (string, int, error) {
			return strings.ToLower(word), 1, nil
		},
		// Reduce phase: Sum all "1" values for the same key
		// Concurrency = 2
		2, func(x, y int) (int, error) {
			return x + y, nil
		},
	)

	fmt.Println("Result:", mr)
	fmt.Println("Error:", err)
}

func ExampleMerge() {
	// Convert slices of numbers into streams
	numbers1 := rill.FromSlice([]int{1, 2, 3, 4, 5}, nil)
	numbers2 := rill.FromSlice([]int{6, 7, 8, 9, 10}, nil)
	numbers3 := rill.FromSlice([]int{11, 12}, nil)

	numbers := rill.Merge(numbers1, numbers2, numbers3)

	printStream(numbers)
}

func ExampleReduce() {
	// A stream of 62 single-character strings
	str := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	letters := rill.FromSlice(strings.Split(str, ""), nil)

	// Reassemble the original string. Concurrency = 4
	// String concatenation is a simple non-commutative operation
	// and is used here for demonstration only.
	res, ok, err := rill.Reduce(letters, 4, func(x, y string) (string, error) {
		return x + y, nil
	})

	fmt.Println("Result:", res, ok)
	fmt.Println("Error:", err)
}

func ExampleTee() {
	// Convert a slice of numbers into a stream
	numbers := rill.FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, nil)

	// Create two identical copies of the stream
	// Each copy can be transformed separately
	numbers1, numbers2 := rill.Tee(numbers)

	// Keep only the odd numbers in the first copy
	// Concurrency = 3
	odd := rill.Filter(numbers1, 3, func(x int) (bool, error) {
		return x%2 != 0, nil
	})

	// Double the numbers in the second copy
	// Concurrency = 3
	doubled := rill.Map(numbers2, 3, func(x int) (int, error) {
		return x * 2, nil
	})

	// Both copies must be consumed concurrently, otherwise Tee deadlocks
	var wg sync.WaitGroup

	// First consumer prints the odd numbers
	wg.Go(func() {
		printStream(odd)
	})

	// Second consumer sums the doubled numbers
	wg.Go(func() {
		sum, _, err := rill.Reduce(doubled, 3, func(a, b int) (int, error) {
			return a + b, nil
		})

		fmt.Println("Sum:", sum)
		fmt.Println("Sum error:", err)
	})

	wg.Wait()
}

func ExampleToSlice() {
	// Convert a slice of numbers into a stream
	numbers := rill.FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, nil)

	// Transform each number
	// Concurrency = 3; Ordered
	squares := rill.OrderedMap(numbers, 3, func(x int) (int, error) {
		return square(x), nil
	})

	resultsSlice, err := rill.ToSlice(squares)

	fmt.Println("Result:", resultsSlice)
	fmt.Println("Error:", err)
}

func ExampleUnbatch() {
	// Create a stream of batches
	batches := rill.FromSlice([][]int{
		{1, 2, 3},
		{4, 5},
		{6, 7, 8, 9},
		{10},
	}, nil)

	numbers := rill.Unbatch(batches)

	printStream(numbers)
}

func ExampleFromSeq() {
	// Start with an iterator that yields numbers from 1 to 10
	numbersSeq := slices.Values([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

	// Convert the iterator into a stream
	numbers := rill.FromSeq(numbersSeq, nil)

	// Transform each number
	// Concurrency = 3
	squares := rill.Map(numbers, 3, func(x int) (int, error) {
		return square(x), nil
	})

	printStream(squares)
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
	squares := rill.Map(numbers, 3, func(x int) (int, error) {
		return square(x), nil
	})

	printStream(squares)
}

func ExampleToSeq2() {
	ctx, scope := rill.WithContext(context.Background())

	ids := rill.FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, nil)

	// Read users from the API.
	users := rill.Map(ids, 1, func(id int) (*api.User, error) {
		return api.GetUser(ctx, id)
	})

	for user, err := range rill.ToSeq2(users, scope) {
		if err != nil {
			fmt.Println("Error:", err)
			break
		}

		fmt.Println("Seen:", user.ID)
		if user.ID == 5 {
			break // blocks until the pipeline has finished
		}
	}

	// The context is canceled and nothing is running anymore
}

func ExampleWithContext() {
	// ctx is captured by the callbacks below.
	// scope is a functional option passed to ForEach.
	ctx, scope := rill.WithContext(context.Background())

	// Convert a slice into a channel
	ids := rill.FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, nil)

	// Read users from the API. Concurrency = 3
	users := rill.Map(ids, 3, func(id int) (*api.User, error) {
		return api.GetUser(ctx, id)
	})

	// Process users. Concurrency = 2
	err := rill.ForEach(users, 2, func(u *api.User) error {
		if u.IsActive {
			return nil
		}
		u.IsActive = true
		return api.SaveUser(ctx, u)
	}, scope)

	// Nothing is running anymore; the context is canceled.
	// Handle the error (if any).
	fmt.Println("Error:", err)
}

// --- Helpers ---

// isPrime checks if a number is prime
// and simulates some additional work by sleeping.
func isPrime(n int) bool {
	simulateWork(500 * time.Millisecond)

	if n < 2 {
		return false
	}
	for i := 2; i*i <= n; i++ {
		if n%i == 0 {
			return false
		}
	}
	return true
}

// square returns the square of x and simulates some additional work by sleeping.
func square(x int) int {
	simulateWork(500 * time.Millisecond)
	return x * x
}

// printStream prints all items from a stream (one per line) and an error, if any.
func printStream[A any](stream <-chan rill.Try[A]) {
	fmt.Println("Result:")
	err := rill.ForEach(stream, 1, func(x A) error {
		fmt.Printf("%+v\n", x)
		return nil
	})
	fmt.Println("Error:", err)
}

func simulateWork(max time.Duration) {
	time.Sleep(time.Duration(rand.Intn(int(max))))
}
