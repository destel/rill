package rill_test

import (
	"fmt"
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

// This example demonstrates how [ForEach] can be used for concurrent processing and error handling.
func Example_forEach() {
	startedAt := time.Now()
	defer func() { fmt.Println("Elapsed:", time.Since(startedAt)) }()

	items := rill.FromSlice([]string{"item1", "item2", "item3", "item4", "item5", "item6", "item7", "item8", "item9", "item10"}, nil)

	items = rill.Map(items, 3, func(item string) (string, error) {
		randomSleep(1000 * time.Millisecond) // simulate some work

		if item == "item6" {
			return "", fmt.Errorf("invalid item")
		}

		return strings.ToUpper(item), nil
	})

	// For each will stop on the first error
	err := rill.ForEach(items, 3, func(item string) error {
		fmt.Println(item)
		return nil
	})
	if err != nil {
		fmt.Println("Error:", err)
	}
}

// This example fetches keys from a list of URLs, retrieves their values from a key-value database, and prints them.
// The pipeline leverages concurrency for fetching and processing and uses batching to reduce the number of database calls.
func Example_keyValueBatchRead() {
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
		return downloadFile(url)
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
// Pipeline below takes a list of keys and queries their values from a key-value database.
// Values are fetched concurrently, but results are printed in order.
func Example_ordering() {
	startedAt := time.Now()
	defer func() { fmt.Println("Elapsed:", time.Since(startedAt)) }()

	keys := rill.FromSlice([]string{"key:1", "key:2", "key:3", "key:4", "key:5", "key:6", "key:7", "key:8", "key:9", "key:10"}, nil)

	// Get values for each key concurrently, but stream them in order
	values := rill.OrderedMap(keys, 3, func(key string) (string, error) {
		return kvGet(key)
	})

	// Iterate over each value and print
	err := rill.ForEach(values, 1, func(value string) error {
		fmt.Println(value)
		return nil
	})
	if err != nil {
		fmt.Println("Error:", err)
	}
}

// downloadFile simulates line-by-line streaming of a file from a URL,
// introducing a randomized delay to simulate network latency.
// It's a simplified placeholder for actual network-based file streaming.
func downloadFile(url string) <-chan rill.Try[string] {
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

func randomSleep(max time.Duration) {
	time.Sleep(time.Duration(rand.Intn(int(max))))
}
