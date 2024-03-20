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

	"github.com/destel/rill/echans"
)

type KV struct {
	Key   string
	Value string
}

func main() {
	err := printValues(context.Background(), []string{
		"https://raw.githubusercontent.com/destel/rill/f/docs/examples/redis-read/ids1.txt",
		"https://raw.githubusercontent.com/destel/rill/f/docs/examples/redis-read/ids2.txt",
		"https://raw.githubusercontent.com/destel/rill/f/docs/examples/redis-read/ids3.txt",
	})

	if err != nil {
		fmt.Println("Error:", err)
	}
}

// printValues orchestrates a pipeline that fetches keys from URLs, retrieves their values from Redis, and prints them.
// The pipeline leverages concurrency for fetching and processing, utilizes batching for efficient Redis lookups,
// and employs context for graceful cancellation and timeout handling. Batching not only improves performance by
// reducing the number of Redis calls but also demonstrates the package's ability to group stream elements effectively.
func printValues(ctx context.Context, urls []string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel() // In case of error or early exit, this ensures all http and redis operations are canceled

	// Convert URLs into a channel
	urlsChan := echans.FromSlice(urls)

	// Fetch and stream keys from each URL concurrently
	keys := echans.FlatMap(urlsChan, 10, func(url string) <-chan echans.Try[string] {
		return streamKeys(ctx, url)
	})

	// Exclude any empty keys from the stream
	keys = echans.Filter(keys, 5, func(key string) (bool, error) {
		return key != "", nil
	})

	// Organize keys into manageable batches of 10 for bulk operations
	keyBatches := echans.Batch(keys, 10, 1*time.Second)

	// Fetch values from Redis for each batch of keys
	resultBatches := echans.Map(keyBatches, 5, func(keys []string) ([]KV, error) {
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
	results := echans.Unbatch(resultBatches)

	// Exclude any empty values from the stream
	results = echans.Filter(results, 5, func(kv KV) (bool, error) {
		return kv.Value != "<nil>", nil
	})

	// Iterate over each key-value pair and print
	cnt := 0
	err := echans.ForEach(results, 1, func(kv KV) error {
		fmt.Println(kv.Key, "=>", kv.Value)
		cnt++
		return nil
	})
	fmt.Println("Total keys:", cnt)

	return err
}

// streamKeys reads a file from the given URL line by line and returns a channel of lines/keys
func streamKeys(ctx context.Context, url string) <-chan echans.Try[string] {
	out := make(chan echans.Try[string], 1)

	go func() {
		defer close(out)

		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			out <- echans.Try[string]{Error: err}
			return
		}

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			out <- echans.Try[string]{Error: err}
			return
		}
		defer res.Body.Close()

		r := bufio.NewReader(res.Body)

		for {
			line, err := r.ReadString('\n')
			line = strings.TrimSuffix(line, "\n")

			if errors.Is(err, io.EOF) {
				out <- echans.Try[string]{V: line}
				return
			}
			if err != nil {
				out <- echans.Try[string]{Error: err}
				return
			}

			out <- echans.Try[string]{V: line}
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
