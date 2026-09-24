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

// This example demonstrates a rill pipeline that fetches users from an API,
// updates their status to active, and saves them back.
// Both operations are performed concurrently, and errors are handled in one place at the end.
func Example() {
	// The context is canceled on the first error or when ForEach returns,
	// whichever occurs first.
	ctx, scope := rill.WithContext(context.Background())

	// Convert a slice of user IDs into a stream
	ids := rill.FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, nil)

	// Read users from the API.
	// Concurrency = 3
	users := rill.Map(ids, 3, func(id int) (*api.User, error) {
		return api.GetUser(ctx, id)
	})

	// Activate users.
	// Concurrency = 2
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

	// Nothing is running anymore. Handle errors:
	fmt.Println("Error:", err)
}

// This example demonstrates a rill pipeline that fetches users from an API,
// updates their status to active, and saves them back.
// Users are fetched concurrently and in batches to reduce the number of API calls.
func Example_batching() {
	ctx, scope := rill.WithContext(context.Background())

	// Convert a slice of user IDs into a stream
	ids := rill.FromSlice([]int{
		1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20,
		21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40,
	}, nil)

	// Group IDs into batches of 5
	idBatches := rill.Batch(ids, 5, -1)

	// Bulk fetch users from the API
	// Concurrency = 3
	userBatches := rill.Map(idBatches, 3, func(ids []int) ([]*api.User, error) {
		return api.GetUsers(ctx, ids)
	})

	// Transform the stream of batches back into a flat stream of users
	users := rill.Unbatch(userBatches)

	// Activate users.
	// Concurrency = 2
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

	// Handle errors
	fmt.Println("Error:", err)
}

// This example demonstrates how batching can be used to group similar concurrent database updates into a single query.
// The UpdateUserTimestamp function is used to update the last_active_at column in the users table. Updates are not
// executed immediately but are instead queued and then sent to the database in batches of up to 5.
//
// When updates are sparse, it can take some time to collect a full batch. In this case, the [Batch] function
// emits partial batches, ensuring that updates are delayed by at most 100ms.
//
// For simplicity, this example does not include retries, error handling, or synchronization.
// A more complete version of this pattern, with context support, error handling, and
// synchronization, is described at https://destel.dev/blog/real-time-batching-in-go.
func Example_batchingRealTime() {
	// Start the background worker that processes the updates
	go updateUserTimestampWorker()

	// Do some updates. They'll be automatically grouped into
	// batches: [1,2,3,4,5], [6,7], [8]
	UpdateUserTimestamp(1)
	UpdateUserTimestamp(2)
	UpdateUserTimestamp(3)
	UpdateUserTimestamp(4)
	UpdateUserTimestamp(5)
	UpdateUserTimestamp(6)
	UpdateUserTimestamp(7)
	time.Sleep(500 * time.Millisecond) // simulate sparse updates
	UpdateUserTimestamp(8)

	// Wait for the updates to be processed
	// In real-world application, different synchronization mechanisms would be used.
	time.Sleep(1 * time.Second)
}

// This is the queue of user IDs to update.
var userIDsToUpdate = make(chan int)

// UpdateUserTimestamp is the public API for updating the last_active_at column in the users table.
func UpdateUserTimestamp(userID int) {
	userIDsToUpdate <- userID
}

// This is a background worker that sends queued updates to the database in batches.
// For simplicity, this worker does not include retries, error handling, or synchronization.
func updateUserTimestampWorker() {
	// convert the channel of user IDs into a stream
	ids := rill.FromChan(userIDsToUpdate, nil)

	// Group IDs into batches of 5 for bulk processing
	// In case of sparse updates, we want to send them to the database no later than 100ms after they were queued.
	idBatches := rill.Batch(ids, 5, 100*time.Millisecond)

	// Send updates to the database
	// Concurrency = 1 (this controls max number of concurrent updates)
	_ = rill.ForEach(idBatches, 1, func(batch []int) error {
		fmt.Printf("Executed: UPDATE users SET last_active_at = NOW() WHERE id IN (%v)\n", batch)
		return nil
	})
}

// This example demonstrates how to find the first file containing a specific string among 1000 large files
// hosted online.
//
// Downloading all files at once would consume too much memory, while processing
// them one by one would take too long. Traditional concurrency patterns do not preserve the order of files
// and would make it challenging to find the first match.
//
// The combination of the [OrderedFilter] and [First] functions solves the problem
// while downloading and holding in memory at most 5 files at the same time.
func Example_ordering() {
	ctx, scope := rill.WithContext(context.Background())

	// The string to search for in the downloaded files
	needle := []byte("26")

	// Generate a stream of URLs from https://example.com/file-0.txt
	// to https://example.com/file-999.txt
	// Stop generating URLs if the context is canceled
	urls := rill.Generate(func(send func(string), sendError func(error)) {
		for i := 0; i < 1000 && ctx.Err() == nil; i++ {
			send(fmt.Sprintf("https://example.com/file-%d.txt", i))
		}
	})

	// Download and process the files
	// At most 5 files are downloaded and held in memory at the same time
	matchedUrls := rill.OrderedFilter(urls, 5, func(url string) (bool, error) {
		fmt.Println("Downloading:", url)

		content, err := api.DownloadFile(ctx, url)
		if err != nil {
			return false, err
		}

		// keep only URLs of files that contain the needle
		return bytes.Contains(content, needle), nil
	})

	// Find the first matched URL.
	// The match cancels the context, which stops the URL generation and the
	// downloads in flight; First returns once they have.
	firstMatchedUrl, found, err := rill.First(matchedUrls, scope)
	fmt.Println("First matched URL:", firstMatchedUrl, found, err)
}

// This example demonstrates the parallel streaming pattern: [FlatMap] turns each
// department into its own stream of users and merges these streams into one,
// fetching from several departments concurrently.
// Additionally, it demonstrates how to write a reusable streaming wrapper over paginated API calls -
// the StreamUsers function.
func Example_parallelStreaming() {
	ctx, scope := rill.WithContext(context.Background())

	// Start with a stream of department names
	departments := rill.FromSlice([]string{"IT", "Finance", "Marketing", "Support", "Engineering"}, nil)

	// Stream users from all departments concurrently.
	// At most 3 departments at the same time.
	users := rill.FlatMap(departments, 3, func(department string) <-chan rill.Try[*api.User] {
		return StreamUsers(ctx, &api.UserQuery{Department: department})
	})

	// Print the users from the combined stream
	err := rill.ForEach(users, 1, func(user *api.User) error {
		fmt.Printf("%+v\n", user)
		return nil
	}, scope)
	fmt.Println("Error:", err)
}

// StreamUsers is a reusable streaming wrapper around the api.ListUsers function.
// It iterates through all listing pages and uses [Generate] to simplify sending users and errors to the resulting stream.
// This function is useful both on its own and as part of larger pipelines.
func StreamUsers(ctx context.Context, query *api.UserQuery) <-chan rill.Try[*api.User] {
	return rill.Generate(func(send func(*api.User), sendError func(error)) {
		var currentQuery api.UserQuery
		if query != nil {
			currentQuery = *query
		}

		for page := 0; ; page++ {
			currentQuery.Page = page

			users, err := api.ListUsers(ctx, &currentQuery)
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
	ctx, scope := rill.WithContext(context.Background())

	// The source is an infinite, context-aware stream of natural numbers starting from 114
	numbers := rill.Generate(func(send func(int), sendError func(error)) {
		for i := 114; ctx.Err() == nil; i++ {
			send(i)
		}
	})

	// Keep only the primes. Concurrency = 3; Ordered
	// The check is context-aware: it gives up as soon as the context is canceled
	primes := rill.OrderedFilter(numbers, 3, func(x int) (bool, error) {
		if err := simulateWorkContext(ctx, 500*time.Millisecond); err != nil {
			return false, err
		}
		fmt.Println("Checked:", x)
		return isPrime(x), nil
	})

	// Finding the first prime cancels the context, which stops the source and the remaining checks.
	// First returns once nothing is running anymore.
	first, ok, err := rill.First(primes, scope)
	fmt.Println("First prime:", first, ok, err) // prints 127
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

func simulateWorkContext(ctx context.Context, max time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(time.Duration(rand.Intn(int(max)))):
		return nil
	}
}
