package rill_test

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"math"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/destel/rill"
)

type Measurement struct {
	Date time.Time
	Temp float64
}

type User struct {
	ID       int
	Username string
	IsActive bool
}

// --- Package examples ---

// This example showcases the use of Rill for building a multi-stage data processing pipeline,
// with a focus on batch processing. It streams user ids from a remote file and then fetches users from an API in batches.
// At the end the pipeline filters out inactive users and prints the results.
func Example_batching() {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)

	// In case of early exit this will cancel the file streaming,
	// which in turn will terminate the entire pipeline.
	defer cancel()

	// Stream a file with user ids as an io.Reader
	reader, err := downloadFile(ctx, "http://example.com/user_ids1.txt")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	// Transform the reader into a stream of words
	lines := streamLines(reader)

	// Parse words as integers
	// Concurrency = 3
	ids := rill.Map(lines, 3, func(line string) (int, error) {
		return strconv.Atoi(line)
	})

	// Group IDs into batches of 5 for bulk processing
	idBatches := rill.Batch(ids, 5, 1*time.Second)

	// Fetch users for each batch of IDs
	// Concurrency = 3
	userBatches := rill.Map(idBatches, 3, func(ids []int) ([]*User, error) {
		return getUsers(ctx, ids...)
	})

	// Transform batches back into a stream of users
	users := rill.Unbatch(userBatches)

	// Exclude inactive users
	// Concurrency = 1
	users = rill.Filter(users, 1, func(u *User) (bool, error) {
		return u.IsActive, nil
	})

	// Print results and errors
	err = rill.ForEach(users, 1, func(u *User) error {
		fmt.Printf("%+v\n", u)
		return nil
	})
	if err != nil {
		fmt.Println("Error:", err)
	}
}

// This example demonstrates a Rill pipeline that fetches users from an API concurrently,
// and updates their status to active. Both operations are done concurrently.
func Example_basic() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start with a stream of user ids
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

	fmt.Println("Error:", err)
}

// This example demonstrates how [OrderedMap] can be used to enforce ordering of processing results.
// Pipeline below fetches temperature measurements for a city and calculates daily temperature changes.
// Measurements are fetched concurrently, but ordered processing is used to calculate the changes.
func Example_ordering() {
	city := "New York"
	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -30)

	// Create a stream of all days between startDate and endDate
	days := make(chan rill.Try[time.Time])
	go func() {
		defer close(days)
		for date := startDate; date.Before(endDate); date = date.AddDate(0, 0, 1) {
			days <- rill.Wrap(date, nil)
		}
	}()

	// Fetch the temperature for each day from the API
	// Concurrency = 10; Ordered
	measurements := rill.OrderedMap(days, 10, func(date time.Time) (Measurement, error) {
		temp, err := getTemperature(city, date)
		return Measurement{Date: date, Temp: temp}, err
	})

	// Iterate over the measurements, calculate and print changes.
	// Concurrency = 1; Ordered
	prev := Measurement{Temp: math.NaN()}
	err := rill.ForEach(measurements, 1, func(m Measurement) error {
		change := m.Temp - prev.Temp
		prev = m

		fmt.Printf("%s: %.1f°C (change %+.1f°C)\n", m.Date.Format("2006-01-02"), m.Temp, change)
		return nil
	})

	fmt.Println("Error:", err)
}

// This example demonstrates a concurrent [MapReduce] performed on a set of remote files.
// It downloads them and calculates how many times each word appears in all the files.
func Example_mapReduce() {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)

	defer cancel()

	// Start with a stream of file URLs
	urls := rill.FromSlice([]string{
		"http://example.com/text1.txt",
		"http://example.com/text2.txt",
		"http://example.com/text3.txt",
	}, nil)

	// Download files concurrently, and get a stream of all words from all files
	// Concurrency = 2
	words := rill.FlatMap(urls, 2, func(url string) <-chan rill.Try[string] {
		reader, err := downloadFile(ctx, url)
		if err != nil {
			return rill.FromSlice[string](nil, err) // Wrap the error in a stream
		}

		return streamWords(reader)
	})

	// Count the number of occurrences of each word
	counts, err := rill.MapReduce(words,
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

	fmt.Println("Result:", counts)
	fmt.Println("Error:", err)
}

// --- Function examples ---

func ExampleAll() {
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
	numbers := rill.FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, nil)

	// Is there at least one even number?
	// Concurrency = 3
	ok, err := rill.Any(numbers, 3, func(x int) (bool, error) {
		return x%2 == 0, nil
	})

	fmt.Println("Result: ", ok)
	fmt.Println("Error: ", err)
}

func ExampleBatch() {
	// New number is emitted every 50ms
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
	lines := rill.FromSlice([]string{"1", "2", "3", "4", "5", "not a number 6", "7", "8", "9", "10"}, nil)

	// Convert strings to ints
	// Concurrency = 3; Unordered
	ids := rill.Map(lines, 3, func(a string) (int, error) {
		randomSleep(1000 * time.Millisecond) // simulate some additional work
		return strconv.Atoi(a)
	})

	// Catch and ignore number parsing errors
	// Concurrency = 2; Unordered
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
	lines := rill.FromSlice([]string{"1", "2", "3", "4", "5", "not a number 6", "7", "8", "9", "10"}, nil)

	// Convert strings to ints
	// Concurrency = 3; Ordered
	ids := rill.OrderedMap(lines, 3, func(a string) (int, error) {
		randomSleep(1000 * time.Millisecond) // simulate some additional work
		return strconv.Atoi(a)
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

	users := rill.FromSlice([]*User{
		{ID: 1, Username: "foo"},
		{ID: 2, Username: "bar"},
		{ID: 3},
		{ID: 4, Username: "baz"},
		{ID: 5, Username: "qux"},
		{ID: 6, Username: "quux"},
	}, nil)

	// Save users. Use struct{} as a result type
	// Concurrency = 2; Unordered
	results := rill.Map(users, 2, func(user *User) (struct{}, error) {
		return struct{}{}, saveUser(ctx, user)
	})

	// We're interested only in side effects and errors from
	// the pipeline above
	err := rill.Err(results)
	fmt.Println("Error:", err)
}

func ExampleFilter() {
	numbers := rill.FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, nil)

	// Keep only even numbers
	// Concurrency = 3; Unordered
	evens := rill.Filter(numbers, 3, func(x int) (bool, error) {
		randomSleep(1000 * time.Millisecond) // simulate some additional work
		return x%2 == 0, nil
	})

	printStream(evens)
}

// The same example as for the [Filter], but using ordered versions of functions.
func ExampleOrderedFilter() {
	numbers := rill.FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, nil)

	// Keep only even numbers
	// Concurrency = 3; Ordered
	evens := rill.OrderedFilter(numbers, 3, func(x int) (bool, error) {
		randomSleep(1000 * time.Millisecond) // simulate some additional work
		return x%2 == 0, nil
	})

	printStream(evens)
}

func ExampleFirst() {
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
	numbers := rill.FromSlice([]int{1, 2, 3, 4, 5}, nil)

	// Replace each number with three strings
	// Concurrency = 3; Unordered
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
	numbers := rill.FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, nil)

	// Square and print each number
	// Concurrency = 3; Unordered
	err := rill.ForEach(numbers, 3, func(x int) error {
		randomSleep(1000 * time.Millisecond) // simulate some additional work

		y := x * x
		fmt.Println(y)
		return nil
	})

	fmt.Println("Error:", err)
}

// There is no ordered version of the ForEach function. To achieve ordered processing,
// move code to [OrderedMap] and then use ForEach with concurrency set to 1.
func ExampleForEach_ordered() {
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
	numbers := rill.FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, nil)

	// Square each number.
	// Concurrency = 3; Unordered
	squares := rill.Map(numbers, 3, func(x int) (int, error) {
		randomSleep(1000 * time.Millisecond) // simulate some additional work
		return x * x, nil
	})

	printStream(squares)
}

// The same example as for the [Map], but using ordered versions of functions.
func ExampleOrderedMap() {
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

	// Start with a stream of words
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
	numbers1 := rill.FromSlice([]int{1, 2, 3, 4, 5}, nil)
	numbers2 := rill.FromSlice([]int{6, 7, 8, 9, 10}, nil)
	numbers3 := rill.FromSlice([]int{11, 12}, nil)

	numbers := rill.Merge(numbers1, numbers2, numbers3)

	printStream(numbers)
}

func ExampleReduce() {
	numbers := rill.FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, nil)

	// Sum all numbers
	sum, ok, err := rill.Reduce(numbers, 3, func(a, b int) (int, error) {
		return a + b, nil
	})

	fmt.Println("Result:", sum, ok)
	fmt.Println("Error:", err)
}

func ExampleToSlice() {
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

// streamLines converts an io.Reader into a stream of lines
func streamLines(r io.ReadCloser) <-chan rill.Try[string] {
	out := make(chan rill.Try[string])
	go func() {
		defer r.Close()
		defer close(out)

		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			out <- rill.Wrap(scanner.Text(), nil)
		}
		if err := scanner.Err(); err != nil {
			out <- rill.Wrap("", err)
		}
	}()
	return out
}

// streamWords is helper function that converts an io.Reader into a stream of words.
func streamWords(r io.ReadCloser) <-chan rill.Try[string] {
	raw := make(chan rill.Try[string], 1)

	go func() {
		defer r.Close()
		defer close(raw)

		scanner := bufio.NewScanner(r)
		scanner.Split(bufio.ScanWords)

		for scanner.Scan() {
			word := scanner.Text()
			word = strings.Trim(word, ".,;:!?&()") // strip all punctuation. it's basic and just for demonstration
			if len(word) > 0 {
				raw <- rill.Wrap(word, nil)
			}
		}
		if err := scanner.Err(); err != nil {
			raw <- rill.Wrap("", err)
		}
	}()

	return raw
}

var ErrFileNotFound = errors.New("file not found")

var files = map[string]string{
	"http://example.com/user_ids1.txt": strings.ReplaceAll("1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20", " ", "\n"),
	"http://example.com/user_ids2.txt": strings.ReplaceAll("21 22 23 24 25 26 27 28 29 30 31 32 33 34 35 36 37 38 39 40", " ", "\n"),
	"http://example.com/user_ids3.txt": strings.ReplaceAll("41 42 43 44 45", " ", "\n"),
	"http://example.com/text1.txt":     "Early morning brings early birds to the early market. Birds sing, the market buzzes, and the morning shines.",
	"http://example.com/text2.txt":     "The birds often sing at the market",
	"http://example.com/text3.txt":     "The market closes, the birds rest, and the night brings peace to the town.",
}

// downloadFile simulates downloading a file from a URL.
// Returns a reader for the file content.
func downloadFile(ctx context.Context, url string) (io.ReadCloser, error) {
	content, ok := files[url]
	if !ok {
		return nil, ErrFileNotFound
	}

	// In a real-world scenario, this would be an HTTP request depending on the ctx.
	return io.NopCloser(strings.NewReader(content)), nil
}

// getTemperature simulates fetching a temperature reading for a city and date,
func getTemperature(city string, date time.Time) (float64, error) {
	randomSleep(1000 * time.Millisecond) // Simulate a network delay

	// Basic city hash, to make measurements unique for each city
	cityHash := float64(hash(city))

	// Simulate a temperature reading, by retuning a pseudo-random, but deterministic value
	temp := 15 - 10*math.Sin(cityHash+float64(date.Unix()))

	return temp, nil
}

var adjs = []string{"big", "small", "fast", "slow", "smart", "happy", "sad", "funny", "serious", "angry"}
var nouns = []string{"dog", "cat", "bird", "fish", "mouse", "elephant", "lion", "tiger", "bear", "wolf"}

// getUsers simulates fetching multiple users from an API.
// User fields are pseudo-random, but deterministic based on the user ID.
func getUsers(ctx context.Context, ids ...int) ([]*User, error) {
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
	users, err := getUsers(ctx, id)
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
