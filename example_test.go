package rill_test

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"math/rand"
	"path/filepath"
	"strings"
	"time"

	"github.com/destel/rill"
)

type KV struct {
	Key   string
	Value string
}

type Measurement struct {
	Date time.Time
	Temp float64
}

// A basic demonstrating how [ForEach] can be used to process a list of items concurrently.
func Example_basic() {
	items := rill.FromSlice([]string{"item1", "item2", "item3", "item4", "item5", "item6", "item7", "item8", "item9", "item10"}, nil)

	err := rill.ForEach(items, 3, func(item string) error {
		randomSleep(1000 * time.Millisecond) // simulate some additional work
		res := strings.ToUpper(item)
		fmt.Println(res)
		return nil
	})
	if err != nil {
		fmt.Println("Error:", err)
	}
}

// This example fetches keys from a list of URLs, retrieves their values from a key-value database, and prints them.
// The pipeline leverages concurrency for fetching and processing and uses batching to reduce the number of database calls.
func Example_batching() {
	startedAt := time.Now()
	defer func() { fmt.Println("Elapsed:", time.Since(startedAt)) }()

	urls := rill.FromSlice([]string{
		"https://example.com/file1.txt",
		"https://example.com/file2.txt",
		"https://example.com/file3.txt",
		"https://example.com/file4.txt",
	}, nil)

	// Fetch keys from each URL and flatten them into a single stream
	keys := rill.FlatMap(urls, 3, func(url string) <-chan rill.Try[string] {
		return streamFileLines(url)
	})

	// Exclude any empty keys from the stream
	keys = rill.Filter(keys, 3, func(key string) (bool, error) {
		return key != "", nil
	})

	// Organize keys into manageable batches of 10 for bulk operations
	keyBatches := rill.Batch(keys, 10, 1*time.Second)

	// Fetch values from DB for each batch of keys
	resultBatches := rill.Map(keyBatches, 3, func(keys []string) ([]KV, error) {
		values, err := kvMultiGet(keys...)
		if err != nil {
			return nil, err
		}

		results := make([]KV, len(keys))
		for i, key := range keys {
			results[i] = KV{Key: key, Value: values[i]}
		}

		return results, nil
	})

	// Convert batches back to a single items for final processing
	results := rill.Unbatch(resultBatches)

	// Exclude any empty values from the stream
	results = rill.Filter(results, 3, func(kv KV) (bool, error) {
		return kv.Value != "<nil>", nil
	})

	// Iterate over each key-value pair and print
	cnt := 0
	err := rill.ForEach(results, 1, func(kv KV) error {
		fmt.Println(kv.Key, "=>", kv.Value)
		cnt++
		return nil
	})
	if err != nil {
		fmt.Println("Error:", err)
	}

	fmt.Println("Total keys:", cnt)
}

// This example demonstrates how [OrderedMap] can be used to enforce ordering of processing results.
// Pipeline below fetches temperature measurements for a city and calculates daily temperature changes.
// Measurements are fetched concurrently, but ordered processing is used to calculate the changes.
func Example_ordering() {
	startedAt := time.Now()
	defer func() { fmt.Println("Elapsed:", time.Since(startedAt)) }()

	city := "New York"
	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -30)

	// Make a channel that emits all the days between startDate and endDate
	days := make(chan rill.Try[time.Time])
	go func() {
		defer close(days)
		for date := startDate; date.Before(endDate); date = date.AddDate(0, 0, 1) {
			days <- rill.Wrap(date, nil)
		}
	}()

	// Download the temperature for each day concurrently
	measurements := rill.OrderedMap(days, 10, func(date time.Time) (Measurement, error) {
		temp, err := getTemperature(city, date)
		return Measurement{Date: date, Temp: temp}, err
	})

	// Iterate over the measurements, calculate and print changes. Use a single goroutine
	prev := Measurement{Temp: math.NaN()}
	err := rill.ForEach(measurements, 1, func(m Measurement) error {
		change := m.Temp - prev.Temp
		prev = m

		fmt.Printf("%s: %.1f°C (change %+.1f°C)\n", m.Date.Format("2006-01-02"), m.Temp, change)
		return nil
	})
	if err != nil {
		fmt.Println("Error:", err)
	}
}

// This example demonstrates how [Reduce] can be used to calculate a sum of numbers from a channel.
func ExampleReduce() {
	// Create a channel with some values
	numbers := rill.FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, nil)

	// Reduce the channel to a single value by summing all numbers
	sum, ok, err := rill.Reduce(numbers, 3, func(a, b int) (int, error) {
		return a + b, nil
	})
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Println("Sum:", sum)
	fmt.Println("OK:", ok)
}

// This example demonstrates how MapReduce can be used to count how many times each word appears in a stream.
// Mappers emit a count of '1' for each word, and reducers sum these counts to calculate the total occurrences of each word.
func ExampleMapReduce() {
	stream := strings.NewReader(`Early morning brings early birds to the early market. Birds sing, the market buzzes, and the morning shines.`)

	words := streamWords(stream)

	mr, err := rill.MapReduce(words,
		3, func(word string) (string, int, error) {
			return strings.ToLower(word), 1, nil
		},
		2, func(x, y int) (int, error) {
			return x + y, nil
		},
	)

	fmt.Println("Err:", err)
	fmt.Println("MapReduce:", mr)
}

// streamFileLines simulates line-by-line streaming of a file from a URL,
// introducing a randomized delay to simulate network latency.
// It's a simplified placeholder for actual network-based file streaming.
func streamFileLines(url string) <-chan rill.Try[string] {
	out := make(chan rill.Try[string])
	go func() {
		defer close(out)

		base := filepath.Base(url)
		base = strings.TrimSuffix(base, filepath.Ext(base))

		for i := 0; i < 10; i++ {
			randomSleep(20 * time.Millisecond) // Simulate a network delay
			out <- rill.Wrap(fmt.Sprintf("%s:key:%d", base, i), nil)
		}
	}()
	return out
}

// streamWords is helper function that reads words from a reader and streams them as strings.
func streamWords(r io.Reader) <-chan rill.Try[string] {
	raw := make(chan rill.Try[string], 1)

	go func() {
		defer close(raw)

		scanner := bufio.NewScanner(r)
		scanner.Split(bufio.ScanWords)

		for scanner.Scan() {
			word := scanner.Text()
			word = strings.Trim(word, ".,;:!?&()") // it's basic and just for demonstration
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

// kvGet simulates fetching a value form a key-value database,
// introducing a randomized delay to simulate network latency.
// It's a simplified placeholder for actual database operation.
func kvGet(key string) (string, error) {
	randomSleep(1000 * time.Millisecond) // Simulate a network delay

	// Simulates that some keys are missing
	if strings.HasSuffix(key, "2") || strings.HasSuffix(key, "3") {
		return "<nil>", nil
	}

	return strings.Replace(key, "key:", "val:", 1), nil
}

// kvMultiGet simulates a batch read from a key-value database,
// introducing a randomized delay to simulate network latency.
// It's a simplified placeholder for actual database operation.
func kvMultiGet(keys ...string) ([]string, error) {
	randomSleep(1000 * time.Millisecond) // Simulate a network delay

	values := make([]string, len(keys))
	for i, key := range keys {
		// Simulates that some keys are missing
		if strings.HasSuffix(key, "2") || strings.HasSuffix(key, "3") {
			values[i] = "<nil>"
			continue
		}

		values[i] = strings.Replace(key, "key:", "val:", 1)
	}

	return values, nil
}

// getTemperature simulates fetching a temperature reading for a city and date,
func getTemperature(city string, date time.Time) (float64, error) {
	randomSleep(1000 * time.Millisecond) // Simulate a network delay

	// Basic city hash, to make measurements unique for each city
	var h float64
	for _, c := range city {
		h += float64(c)
	}

	// Simulate a temperature reading, by retuning a pseudo-random, but deterministic value
	temp := 15 - 10*math.Sin(h+float64(date.Unix()))

	return temp, nil
}

func randomSleep(max time.Duration) {
	time.Sleep(time.Duration(rand.Intn(int(max))))
}
