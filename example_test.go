package rill_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/destel/rill"
)

type User struct {
	ID       int
	Username string
	IsActive bool
}

// --- Package examples ---

// This example demonstrates a Rill pipeline that fetches users from an API,
// and updates their status to active and saves them back. Both operations are done concurrently.
func Example() {
	ctx := context.Background()

	// Convert a slice of user IDs into a stream
	ids := rill.FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, nil)

	// Read users from the API.
	// Concurrency = 3
	users := rill.Map(ids, 3, func(id int) (*User, error) {
		return getUser(ctx, id)
	})

	// Activate users.
	// Concurrency = 2
	err := rill.ForEach(users, 2, func(u *User) error {
		if u.IsActive {
			fmt.Printf("User %d is already active\n", u.ID)
			return nil
		}

		u.IsActive = true
		return saveUser(ctx, u)
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

	// Group IDs into batches of 5 for bulk processing
	idBatches := rill.Batch(ids, 5, 1*time.Second)

	// Bulk read user for each batch of IDs
	// Concurrency = 3
	userBatches := rill.Map(idBatches, 3, func(ids []int) ([]*User, error) {
		return getUsers(ctx, ids)
	})

	// Transform the stream of batches back into a stream of users
	users := rill.Unbatch(userBatches)

	// Activate users.
	// Concurrency = 2
	err := rill.ForEach(users, 2, func(u *User) error {
		if u.IsActive {
			fmt.Printf("User %d is already active\n", u.ID)
			return nil
		}

		u.IsActive = true
		return saveUser(ctx, u)
	})

	// Handle errors
	fmt.Println("Error:", err)
}

// This example demonstrates how batching can be used to group similar concurrent database updates into a single query.
// The UpdateUserTimestamp function is used to update the last_active_at column in the users table. Updates are not
// executed immediately, but are rather queued and then sent to the database in batches of up to 5.
// When updates are sparse, it can take some time to collect a full batch. In this case the [Batch] function
// emits partial batches, ensuring that updates are delayed by at most 100ms.
//
// For simplicity, this example does not have retries, error handling and synchronization
func Example_batching_updatesGrouping() {
	// Start the background worker that processes the updates
	go updateUserTimestampWorker()

	// Do some updates. They'll be grouped and sent in the background.
	UpdateUserTimestamp(1)
	UpdateUserTimestamp(2)
	UpdateUserTimestamp(3)
	UpdateUserTimestamp(4)
	UpdateUserTimestamp(5)
	UpdateUserTimestamp(6)
	UpdateUserTimestamp(7)

	// Wait for the updates to be processed
	// In real-world application, different synchronization mechanisms would be used.
	time.Sleep(2 * time.Second)
}

// This is the queue of user IDs to update.
var userIDsToUpdate = make(chan int)

// UpdateUserTimestamp is the public API to update the last_active_at column in the users table.
func UpdateUserTimestamp(userID int) {
	userIDsToUpdate <- userID
}

func updateUserTimestampWorker() {
	// convert channel of userIDsStream into a stream
	ids := rill.FromChan(userIDsToUpdate, nil)

	// Group IDs into batches of 5 for bulk processing
	// In case of sparse updates, we want to send them to the database no later than 100ms after they were queued.
	idBatches := rill.Batch(ids, 5, 100*time.Millisecond)

	// Send updates to the database
	// Concurrency = 1 (this controls max number of concurrent updates)
	_ = rill.ForEach(idBatches, 1, func(batch []int) error {
		randomSleep(1 * time.Second)
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// The string to search for in the downloaded files
	needle := []byte("26")

	// Generate a stream of URLs from http://example.com/file-0.txt to http://example.com/file-999.txt
	urls := make(chan rill.Try[string])
	go func() {
		defer close(urls)
		for i := 0; i < 1000; i++ {
			// Stop generating URLs after the context is canceled (when file is found)
			// This can be rewritten as a select statement, but it's not necessary
			if err := ctx.Err(); err != nil {
				return
			}

			urls <- rill.Wrap(fmt.Sprintf("https://example.com/file-%d.txt", i), nil)
		}
	}()

	// Download and process the files
	// At most 5 files are downloaded and held in memory at the same time
	matchedUrls := rill.OrderedFilter(urls, 5, func(url string) (bool, error) {
		fmt.Println("Downloading:", url)

		content, err := downloadFile(ctx, url)
		if err != nil {
			return false, nil
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

// This example demonstrates how to use a context for pipeline termination.
// The findFirstPrime function uses several concurrent workers to find the first prime number after a given number.
// Internally it creates a pipeline that starts from an infinite stream of numbers. When the first prime number is found,
// in that stream, the context gets canceled, and the pipeline terminates gracefully.
func Example_context() {
	p := findFirstPrime(10000, 3) // Concurrency = 3
	fmt.Println("First prime after 10000:", p)
}

// findFirstPrime finds the first prime number after the given number, using several concurrent workers.
func findFirstPrime(after int, concurrency int) int {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Generate an infinite stream of numbers starting from the given number
	numbers := make(chan rill.Try[int])
	go func() {
		defer close(numbers)
		for i := after + 1; ; i++ {
			select {
			case <-ctx.Done():
				return // Stop generating numbers when the context is canceled
			case numbers <- rill.Wrap(i, nil):
			}
		}
	}()

	// Filter out non-prime numbers, preserve the order
	primes := rill.OrderedFilter(numbers, concurrency, func(x int) (bool, error) {
		return isPrime(x), nil
	})

	// Find the first prime number
	result, _, _ := rill.First(primes)
	return result
}

// --- Function examples ---

func ExampleAll() {
	// Convert a slice of numbers into a stream
	numbers := rill.FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, nil)

	// Are all numbers even?
	// Concurrency = 3
	ok, err := rill.All(numbers, 3, func(x int) (bool, error) {
		return x%2 == 0, nil
	})

	fmt.Println("Result:", ok)
	fmt.Println("Error:", err)
}

func ExampleAny() {
	// Convert a slice of numbers into a stream
	numbers := rill.FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, nil)

	// Is there at least one even number?
	// Concurrency = 3
	ok, err := rill.Any(numbers, 3, func(x int) (bool, error) {
		return x%2 == 0, nil
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
		randomSleep(1000 * time.Millisecond) // simulate some additional work
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
		randomSleep(1000 * time.Millisecond) // simulate some additional work
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
	users := rill.FromSlice([]*User{
		{ID: 1, Username: "foo"},
		{ID: 2, Username: "bar"},
		{ID: 3},
		{ID: 4, Username: "baz"},
		{ID: 5, Username: "qux"},
		{ID: 6, Username: "quux"},
	}, nil)

	// Save users. Use struct{} as a result type
	// Concurrency = 2
	results := rill.Map(users, 2, func(user *User) (struct{}, error) {
		return struct{}{}, saveUser(ctx, user)
	})

	// We're interested only in side effects and errors from
	// the pipeline above
	err := rill.Err(results)
	fmt.Println("Error:", err)
}

func ExampleFilter() {
	// Convert a slice of numbers into a stream
	numbers := rill.FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, nil)

	// Keep only even numbers
	// Concurrency = 3
	evens := rill.Filter(numbers, 3, func(x int) (bool, error) {
		randomSleep(1000 * time.Millisecond) // simulate some additional work
		return x%2 == 0, nil
	})

	printStream(evens)
}

// The same example as for the [Filter], but using ordered versions of functions.
func ExampleOrderedFilter() {
	// Convert a slice of numbers into a stream
	numbers := rill.FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, nil)

	// Keep only even numbers
	// Concurrency = 3; Ordered
	evens := rill.OrderedFilter(numbers, 3, func(x int) (bool, error) {
		randomSleep(1000 * time.Millisecond) // simulate some additional work
		return x%2 == 0, nil
	})

	printStream(evens)
}

func ExampleFilterMap() {
	// Convert a slice of numbers into a stream
	numbers := rill.FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, nil)

	// Keep only odd numbers and square them
	// Concurrency = 3
	squares := rill.FilterMap(numbers, 3, func(x int) (int, bool, error) {
		if x%2 == 0 {
			return 0, false, nil
		}

		randomSleep(1000 * time.Millisecond) // simulate some additional work
		return x * x, true, nil
	})

	printStream(squares)
}

// The same example as for the [FilterMap], but using ordered versions of functions.
func ExampleOrderedFilterMap() {
	// Convert a slice of numbers into a stream
	numbers := rill.FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, nil)

	// Keep only odd numbers and square them
	// Concurrency = 3; Ordered
	squares := rill.OrderedFilterMap(numbers, 3, func(x int) (int, bool, error) {
		if x%2 == 0 {
			return 0, false, nil
		}

		randomSleep(1000 * time.Millisecond) // simulate some additional work
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

	// Replace each number with three strings
	// Concurrency = 3
	result := rill.FlatMap(numbers, 3, func(x int) <-chan rill.Try[string] {
		randomSleep(1000 * time.Millisecond) // simulate some additional work

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

	// Replace each number with three strings
	// Concurrency = 3; Ordered
	result := rill.OrderedFlatMap(numbers, 3, func(x int) <-chan rill.Try[string] {
		randomSleep(1000 * time.Millisecond) // simulate some additional work

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

	// Square and print each number
	// Concurrency = 3
	err := rill.ForEach(numbers, 3, func(x int) error {
		randomSleep(1000 * time.Millisecond) // simulate some additional work

		y := x * x
		fmt.Println(y)
		return nil
	})

	fmt.Println("Error:", err)
}

// There is no ordered version of the ForEach function. To achieve ordered processing, use concurrency set to 1.
// If you need a concurrent and ordered ForEach, then do all processing with the [OrderedMap],
// and then use ForEach with concurrency set to 1 at the final stage.
func ExampleForEach_ordered() {
	// Convert a slice of numbers into a stream
	numbers := rill.FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, nil)

	// Square each number.
	// Concurrency = 3; Ordered
	squares := rill.OrderedMap(numbers, 3, func(x int) (int, error) {
		randomSleep(1000 * time.Millisecond) // simulate some additional work
		return x * x, nil
	})

	// Print results.
	// Concurrency = 1; Ordered
	err := rill.ForEach(squares, 1, func(y int) error {
		fmt.Println(y)
		return nil
	})
	fmt.Println("Error:", err)
}

func ExampleMap() {
	// Convert a slice of numbers into a stream
	numbers := rill.FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, nil)

	// Square each number.
	// Concurrency = 3
	squares := rill.Map(numbers, 3, func(x int) (int, error) {
		randomSleep(1000 * time.Millisecond) // simulate some additional work
		return x * x, nil
	})

	printStream(squares)
}

// The same example as for the [Map], but using ordered versions of functions.
func ExampleOrderedMap() {
	// Convert a slice of numbers into a stream
	numbers := rill.FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, nil)

	// Square each number.
	// Concurrency = 3; Ordered
	squares := rill.OrderedMap(numbers, 3, func(x int) (int, error) {
		randomSleep(1000 * time.Millisecond) // simulate some additional work
		return x * x, nil
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

	// Square each number
	// Concurrency = 3; Ordered
	squares := rill.OrderedMap(numbers, 3, func(x int) (int, error) {
		return x * x, nil
	})

	squaresSlice, err := rill.ToSlice(squares)

	fmt.Println("Result:", squaresSlice)
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

// Helper function that simulates sending a message through a server
func sendMessage(message string, server string) error {
	randomSleep(1000 * time.Millisecond) // simulate some additional work
	fmt.Printf("Sent through %s: %s\n", server, message)
	return nil
}

var adjs = []string{"big", "small", "fast", "slow", "smart", "happy", "sad", "funny", "serious", "angry"}
var nouns = []string{"dog", "cat", "bird", "fish", "mouse", "elephant", "lion", "tiger", "bear", "wolf"}

// getUsers simulates fetching multiple users from an API.
// User fields are pseudo-random, but deterministic based on the user ID.
func getUsers(ctx context.Context, ids []int) ([]*User, error) {
	randomSleep(1000 * time.Millisecond) // Simulate a network delay

	users := make([]*User, 0, len(ids))
	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		user := User{
			ID:       id,
			Username: adjs[hash(id, "adj")%len(adjs)] + "_" + nouns[hash(id, "noun")%len(nouns)], // adj + noun
			IsActive: hash(id, "active")%100 < 60,                                                // 60%
		}

		users = append(users, &user)
	}
	return users, nil
}

var ErrUserNotFound = errors.New("user not found")

// getUser simulates fetching a user from an API.
func getUser(ctx context.Context, id int) (*User, error) {
	users, err := getUsers(ctx, []int{id})
	if err != nil {
		return nil, err
	}

	if len(users) == 0 {
		return nil, ErrUserNotFound
	}

	return users[0], nil
}

// saveUser simulates saving a user through an API.
func saveUser(ctx context.Context, user *User) error {
	randomSleep(1000 * time.Millisecond) // Simulate a network delay

	if err := ctx.Err(); err != nil {
		return err
	}

	if user.Username == "" {
		return fmt.Errorf("empty username")
	}

	fmt.Printf("User saved: %+v\n", user)
	return nil
}

// downloadFile simulates downloading a file from a URL.
func downloadFile(ctx context.Context, url string) ([]byte, error) {
	randomSleep(1 * time.Second)
	return []byte("Content of the file " + url), nil
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

// hash is a simple hash function that returns an integer hash for a given input.
func hash(input ...any) int {
	hasher := fnv.New32()
	fmt.Fprintln(hasher, input...)
	return int(hasher.Sum32())
}

// naive prime number check
func isPrime(n int) bool {
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
