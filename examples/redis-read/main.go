package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/destel/rill"
)

type KV struct {
	Key   string
	Value string
}

func main() {
	err := printValuesFromRedis(context.Background(), []string{
		"https://raw.githubusercontent.com/destel/rill/main/examples/redis-read/ids1.txt",
		"https://raw.githubusercontent.com/destel/rill/main/examples/redis-read/ids2.txt",
		"https://raw.githubusercontent.com/destel/rill/main/examples/redis-read/ids3.txt",
	})

	if err != nil {
		fmt.Println("Error:", err)
	}
}

// printValues orchestrates a pipeline that fetches keys from URLs, retrieves their values from Redis, and prints them.
// The pipeline leverages concurrency for fetching and processing and uses batching to reduce the number of Redis calls.
func printValuesFromRedis(ctx context.Context, urls []string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel() // In case of error, this ensures all http and redis operations are canceled

	// Convert urls into a channel
	urlsChan := rill.FromSlice(urls, nil)

	// Fetch and stream keys from each URL concurrently
	keys := rill.FlatMap(urlsChan, 10, func(url string) <-chan rill.Try[string] {
		return streamLines(ctx, url)
	})

	// Exclude any empty keys from the stream
	keys = rill.Filter(keys, 5, func(key string) (bool, error) {
		return key != "", nil
	})

	// Organize keys into manageable batches of 10 for bulk operations
	keyBatches := rill.Batch(keys, 10, 1*time.Second)

	// Fetch values from Redis for each batch of keys
	resultBatches := rill.Map(keyBatches, 5, func(keys []string) ([]KV, error) {
		values, err := redisMGet(ctx, keys...)
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
	results = rill.Filter(results, 5, func(kv KV) (bool, error) {
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
		return err
	}

	fmt.Println("Total keys:", cnt)
	return nil
}

// streamLines reads a file from the given URL line by line and returns a channel of lines
func streamLines(ctx context.Context, url string) <-chan rill.Try[string] {
	out := make(chan rill.Try[string], 1)

	go func() {
		defer close(out)

		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			out <- rill.Try[string]{Error: err}
			return
		}

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			out <- rill.Try[string]{Error: err}
			return
		}
		defer res.Body.Close()

		if res.StatusCode != http.StatusOK {
			out <- rill.Try[string]{Error: fmt.Errorf("got %d code for %s", res.StatusCode, url)}
			return
		}

		r := bufio.NewReader(res.Body)

		for {
			line, err := r.ReadString('\n')
			line = strings.TrimSuffix(line, "\n")

			if errors.Is(err, io.EOF) {
				out <- rill.Try[string]{Value: line}
				return
			}
			if err != nil {
				out <- rill.Try[string]{Error: err}
				return
			}

			out <- rill.Try[string]{Value: line}
		}
	}()

	return out
}

// redisMGet emulates a batch Redis read operation. It returns the values for the given keys.
func redisMGet(ctx context.Context, keys ...string) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Emulate a network delay
	randomSleep(1000 * time.Millisecond)

	values := make([]string, len(keys))
	for i, key := range keys {
		// Emulate that some keys are missing
		if strings.HasSuffix(key, "0") || strings.HasSuffix(key, "5") {
			values[i] = "<nil>"
			continue
		}

		values[i] = "val" + strings.TrimPrefix(key, "id")
	}

	return values, nil
}

func randomSleep(max time.Duration) {
	time.Sleep(time.Duration(rand.Intn(int(max))))
}
