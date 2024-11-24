package rill_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/destel/rill"
	"github.com/destel/rill/mockapi"
)

// --- Package examples ---

// This example demonstrates a Rill pipeline that fetches users from an API,
// updates their status to active and saves them back.
// Both operations are performed concurrently
func Example() {
	ctx := context.Background()

	// Convert a slice of user IDs into a stream
	ids := rill.FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, nil)

	// Read users from the API.
	// Concurrency = 3
	users := rill.Map(ids, 3, func(id int) (*mockapi.User, error) {
		return mockapi.GetUser(ctx, id)
	})

	// Activate users.
	// Concurrency = 2
	err := rill.ForEach(users, 2, func(u *mockapi.User) error {
		if u.IsActive {
			fmt.Printf("User %d is already active\n", u.ID)
			return nil
		}

		u.IsActive = true
		err := mockapi.SaveUser(ctx, u)
		if err != nil {
			return err
		}

		fmt.Printf("User saved: %+v\n", u)
		return nil
	})

	// Handle errors
	fmt.Println("Error:", err)
}

// This example demonstrates a Rill pipeline that fetches users from an API,
// and updates their status to active and saves them back.
// Users are fetched concurrently and in batches to reduce the number of API calls.
func Example_batching() {
	ctx := context.Background()

	// Convert a slice of user IDs into a stream
	ids := rill.FromSlice([]int{
		1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20,
		21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40,
	}, nil)

	// Group IDs into batches of 5
	idBatches := rill.Batch(ids, 5, -1)

	// Bulk fetch users from the API
	// Concurrency = 3
	userBatches := rill.Map(idBatches, 3, func(ids []int) ([]*mockapi.User, error) {
		return mockapi.GetUsers(ctx, ids)
	})

	// Transform the stream of batches back into a flat stream of users
	users := rill.Unbatch(userBatches)

	// Activate users.
	// Concurrency = 2
	err := rill.ForEach(users, 2, func(u *mockapi.User) error {
		if u.IsActive {
			fmt.Printf("User %d is already active\n", u.ID)
			return nil
		}

		u.IsActive = true
		err := mockapi.SaveUser(ctx, u)
		if err != nil {
			return err
		}

		fmt.Printf("User saved: %+v\n", u)
		return nil
	})

	// Handle errors
	fmt.Println("Error:", err)
}

// This example demonstrates how batching can be used to group similar concurrent database updates into a single query.
// The UpdateUserTimestamp function is used to update the last_active_at column in the users table. Updates are not
// executed immediately, but are rather queued and then sent to the database in batches of up to 5.
//
// When updates are sparse, it can take some time to collect a full batch. In this case the [Batch] function
// emits partial batches, ensuring that updates are delayed by at most 100ms.
//
// For simplicity, this example does not have retries, error handling and synchronization
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

// UpdateUserTimestamp is the public API for updating the last_active_at column in the users table
func UpdateUserTimestamp(userID int) {
	userIDsToUpdate <- userID
}

// This is a background worker that sends queued updates to the database in batches.
// For simplicity, there are no retries, error handling and synchronization
func updateUserTimestampWorker() {
	// convert channel of userIDsStream into a stream
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
// them one-by-one would take too long. And traditional concurrency patterns do not preserve the order of files,
// and would make it challenging to find the first match.
//
// The combination of [OrderedFilter] and [First] functions solves the problem,
// while downloading and holding in memory at most 5 files at the same time.
func Example_ordering() {
	ctx := context.Background()

	// The string to search for in the downloaded files
	needle := []byte("26")

	// Generate a stream of URLs from https://example.com/file-0.txt to https://example.com/file-999.txt
	// New URLs will stop being generated if the context is canceled
	urls := rill.Generate(func(send func(string), sendErr func(error)) {
		for i := 0; i < 1000 && ctx.Err() == nil; i++ {
			send(fmt.Sprintf("https://example.com/file-%d.txt", i))
		}
	})

	// Download and process the files
	// At most 5 files are downloaded and held in memory at the same time
	matchedUrls := rill.OrderedFilter(urls, 5, func(url string) (bool, error) {
		fmt.Println("Downloading:", url)

		content, err := mockapi.DownloadFile(ctx, url)
		if err != nil {
			return false, err
		}

		// keep only URLs of files that contain the needle
		return bytes.Contains(content, needle), nil
	})

	// Find the first matched URL
	firstMatchedUrl, found, err := rill.First(matchedUrls)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	// Print the result
	if found {
		fmt.Println("Found in:", firstMatchedUrl)
	} else {
		fmt.Println("Not found")
	}
}

// This example demonstrates how to use the Fan-in and Fan-out patterns
// to send messages through multiple servers concurrently.
func Example_fanIn_FanOut() {
	// Convert a slice of messages into a stream
	messages := rill.FromSlice([]string{
		"message1", "message2", "message3", "message4", "message5",
		"message6", "message7", "message8", "message9", "message10",
	}, nil)

	// Fan-out the messages to three servers
	results1 := rill.Map(messages, 2, func(message string) (string, error) {
		return message, sendMessage(message, "server1")
	})

	results2 := rill.Map(messages, 2, func(message string) (string, error) {
		return message, sendMessage(message, "server2")
	})

	results3 := rill.Map(messages, 2, func(message string) (string, error) {
		return message, sendMessage(message, "server3")
	})

	// Fan-in the results from all servers into a single stream
	results := rill.Merge(results1, results2, results3)

	// Handle errors
	err := rill.Err(results)
	fmt.Println("Error:", err)
}

// Helper function that simulates sending a message through a server
func sendMessage(message string, server string) error {
	randomSleep(500 * time.Millisecond) // simulate some additional work
	fmt.Printf("Sent through %s: %s\n", server, message)
	return nil
}

// This example demonstrates using [FlatMap] to fetch users from multiple departments concurrently.
// Additionally, it demonstrates how to write a reusable streaming wrapper over paginated API calls - the StreamUsers function
func Example_flatMap() {
	ctx := context.Background()

	// Start with a stream of department names
	departments := rill.FromSlice([]string{"IT", "Finance", "Marketing", "Support", "Engineering"}, nil)

	// Stream users from all departments concurrently.
	// At most 3 departments at the same time.
	users := rill.FlatMap(departments, 3, func(department string) <-chan rill.Try[*mockapi.User] {
		return StreamUsers(ctx, &mockapi.UserQuery{Department: department})
	})

	// Print the users from the combined stream
	err := rill.ForEach(users, 1, func(user *mockapi.User) error {
		fmt.Printf("%+v\n", user)
		return nil
	})
	fmt.Println("Error:", err)
}

// StreamUsers is a reusable streaming wrapper around the mockapi.ListUsers function.
// It iterates through all listing pages and returns a stream of users.
// This function is useful both on its own and as part of larger pipelines.
func StreamUsers(ctx context.Context, query *mockapi.UserQuery) <-chan rill.Try[*mockapi.User] {
	res := make(chan rill.Try[*mockapi.User])

	if query == nil {
		query = &mockapi.UserQuery{}
	}

	go func() {
		defer close(res)

		for page := 0; ; page++ {
			query.Page = page

			users, err := mockapi.ListUsers(ctx, query)
			if err != nil {
				res <- rill.Wrap[*mockapi.User](nil, err)
				return
			}

			if len(users) == 0 {
				break
			}

			for _, user := range users {
				res <- rill.Wrap(user, nil)
			}
		}
	}()

	return res
}

// This example demonstrates how to gracefully stop a pipeline on the first error.
// The CheckAllUsersExist uses several concurrent workers and returns an error as soon as it encounters a non-existent user.
// Such early return triggers the context cancellation, which in turn stops all remaining users fetches.
func Example_context() {
	ctx := context.Background()

	// ID 999 doesn't exist, so fetching will stop after hitting it.
	err := CheckAllUsersExist(ctx, 3, []int{1, 2, 3, 4, 5, 999, 7, 8, 9, 10})
	fmt.Printf("Check result: %v\n", err)
}

// CheckAllUsersExist uses several concurrent workers to checks if all users with given IDs exist.
func CheckAllUsersExist(ctx context.Context, concurrency int, ids []int) error {
	// Create new context that will be canceled when this function returns
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Convert the slice into a stream
	idsStream := rill.FromSlice(ids, nil)

	// Fetch users concurrently.
	users := rill.Map(idsStream, concurrency, func(id int) (*mockapi.User, error) {
		u, err := mockapi.GetUser(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch user %d: %w", id, err)
		}

		fmt.Printf("Fetched user %d\n", id)
		return u, nil
	})

	// Return the first error (if any) and cancel remaining fetches via context
	return rill.Err(users)
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

// Also check out the package level examples to see Batch in action
func ExampleBatch() {
	// Generate a stream of numbers 0 to 49, where a new number is emitted every 50ms
	numbers := make(chan rill.Try[int])
	go func() {
		defer close(numbers)
		for i := 0; i < 50; i++ {
			numbers <- rill.Wrap(i, nil)
			time.Sleep(50 * time.Millisecond)
		}
	}()

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
		randomSleep(500 * time.Millisecond) // simulate some additional work
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

// The same example as for the [Catch], but using ordered versions of functions.
func ExampleOrderedCatch() {
	// Convert a slice of strings into a stream
	strs := rill.FromSlice([]string{"1", "2", "3", "4", "5", "not a number 6", "7", "8", "9", "10"}, nil)

	// Convert strings to ints
	// Concurrency = 3; Ordered
	ids := rill.OrderedMap(strs, 3, func(s string) (int, error) {
		randomSleep(500 * time.Millisecond) // simulate some additional work
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
	users := rill.FromSlice([]*mockapi.User{
		{ID: 1, Name: "foo", Age: 25},
		{ID: 2, Name: "bar", Age: 30},
		{ID: 3}, // empty username is invalid
		{ID: 4, Name: "baz", Age: 35},
		{ID: 5, Name: "qux", Age: 26},
		{ID: 6, Name: "quux", Age: 27},
	}, nil)

	// Save users. Use struct{} as a result type
	// Concurrency = 2
	results := rill.Map(users, 2, func(user *mockapi.User) (struct{}, error) {
		return struct{}{}, mockapi.SaveUser(ctx, user)
	})

	// We're only need to know if all users were saved successfully
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

// The same example as for the [Filter], but using ordered versions of functions.
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

// The same example as for the [FilterMap], but using ordered versions of functions.
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
	dvisibleBy4 := rill.OrderedFilter(numbers, 3, func(x int) (bool, error) {
		return x%4 == 0, nil
	})

	// Get the first number divisible by 4
	first, ok, err := rill.First(dvisibleBy4)

	fmt.Println("Result:", first, ok)
	fmt.Println("Error:", err)
}

func ExampleFlatMap() {
	// Convert a slice of numbers into a stream
	numbers := rill.FromSlice([]int{1, 2, 3, 4, 5}, nil)

	// Replace each number in the input stream with three strings
	// Concurrency = 2
	result := rill.FlatMap(numbers, 2, func(x int) <-chan rill.Try[string] {
		randomSleep(500 * time.Millisecond) // simulate some additional work

		return rill.FromSlice([]string{
			fmt.Sprintf("foo%d", x),
			fmt.Sprintf("bar%d", x),
			fmt.Sprintf("baz%d", x),
		}, nil)
	})

	printStream(result)
}

// The same example as for the [FlatMap], but using ordered versions of functions.
func ExampleOrderedFlatMap() {
	// Convert a slice of numbers into a stream
	numbers := rill.FromSlice([]int{1, 2, 3, 4, 5}, nil)

	// Replace each number in the input stream with three strings
	// Concurrency = 2; Ordered
	result := rill.OrderedFlatMap(numbers, 2, func(x int) <-chan rill.Try[string] {
		randomSleep(500 * time.Millisecond) // simulate some additional work

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

// There is no ordered version of the ForEach function. To achieve ordered processing, use concurrency set to 1.
// If you need a concurrent and ordered ForEach, then do all processing with the [OrderedMap],
// and then use ForEach with concurrency set to 1 at the final stage.
func ExampleForEach_ordered() {
	// Convert a slice of numbers into a stream
	numbers := rill.FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, nil)

	// Square each number
	// Concurrency = 3; Ordered
	squares := rill.OrderedMap(numbers, 3, func(x int) (int, error) {
		return square(x), nil
	})

	// Print results.
	// Concurrency = 1; Ordered
	err := rill.ForEach(squares, 1, func(y int) error {
		fmt.Println(y)
		return nil
	})

	// Handle errors
	fmt.Println("Error:", err)
}

// Generate a stream of URLs from http://example.com/file-0.txt to http://example.com/file-9.txt
func ExampleGenerate() {
	urls := rill.Generate(func(send func(string), sendErr func(error)) {
		for i := 0; i < 10; i++ {
			send(fmt.Sprintf("http://example.com/file-%d.txt", i))
		}
	})

	printStream(urls)
}

// Generate an infinite stream of natural numbers (1, 2, 3, ...).
// New numbers are sent to the stream every 500ms until the context is canceled
func ExampleGenerate_context() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	numbers := rill.Generate(func(send func(int), sendErr func(error)) {
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

// The same example as for the [Map], but using ordered versions of functions.
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
	// Convert a slice of numbers into a stream
	numbers := rill.FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, nil)

	// Sum all numbers
	sum, ok, err := rill.Reduce(numbers, 3, func(a, b int) (int, error) {
		return a + b, nil
	})

	fmt.Println("Result:", sum, ok)
	fmt.Println("Error:", err)
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

// --- Helpers ---

// helper function that checks if a number is prime
// and simulates some additional work using sleep
func isPrime(n int) bool {
	randomSleep(500 * time.Millisecond) // simulate some additional work

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

// helper function that squares the number
// and simulates some additional work using sleep
func square(x int) int {
	randomSleep(500 * time.Millisecond) // simulate some additional work
	return x * x
}

// printStream prints all items from a stream (one per line) and an error if any.
func printStream[A any](stream <-chan rill.Try[A]) {
	fmt.Println("Result:")
	err := rill.ForEach(stream, 1, func(x A) error {
		fmt.Printf("%+v\n", x)
		return nil
	})
	fmt.Println("Error:", err)
}

func randomSleep(max time.Duration) {
	time.Sleep(time.Duration(rand.Intn(int(max))))
}
