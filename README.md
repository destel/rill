# Rill [![GoDoc](https://pkg.go.dev/badge/github.com/destel/rill)](https://pkg.go.dev/github.com/destel/rill) [![Go Report Card](https://goreportcard.com/badge/github.com/destel/rill)](https://goreportcard.com/report/github.com/destel/rill) [![codecov](https://codecov.io/gh/destel/rill/graph/badge.svg?token=252K8OQ7E1)](https://codecov.io/gh/destel/rill) 
Rill (noun: a small stream) is a comprehensive Go toolkit for streaming, parallel processing, and pipeline construction. 
Designed to reduce boilerplate and simplify usage, it empowers developers to focus on core logic 
without getting bogged down by the complexity of concurrency.



## Key features
- **Lightweight**: fast and modular, can be easily integrated into existing projects
- **Easy to use**: the complexity of managing goroutines, wait groups, and error handling is abstracted away
- **Concurrent**: control the level of concurrency for all operations
- **Batching**: provides a simple way to organize and process data in batches
- **Error Handling**: provides a structured way to handle errors in concurrent apps
- **Streaming**: handles real-time data streams or large datasets with a minimal memory footprint
- **Order Preservation**: offers functions that preserve the original order of data, while still allowing for concurrent processing
- **Efficient Resource Use**: the number of goroutines and allocations does not depend on the data size
- **Generic**: all operations are type-safe and can be used with any data type
- **Functional Programming**: based on functional programming concepts, making operations like map, filter, flatMap and others available for channel-based workflows



## Installation
```bash
go get github.com/destel/rill
```



## Example usage
Consider an application that fetches keys from multiple URLs, retrieves their values from a key-value database in batches, and prints them. 
This example demonstrates the library's strengths in handling concurrent tasks, error propagation, batching and data streaming, 
all while maintaining simplicity and efficiency.

[Full runnable example](https://pkg.go.dev/github.com/destel/rill#example-package-Batching)

```go
func main() {
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

// streamFileLines does line-by-line streaming of a file from a URL,
func streamFileLines(url string) <-chan rill.Try[string] {
	// ...
}

// kvMultiGet does a batch read from a key-value database,
func kvMultiGet(keys ...string) ([]string, error) {
	// ...
}
```



## Testing strategy
Rill has a test coverage of over 95%, with testing focused on:
- **Correctness**: ensuring that functions produce accurate results at different levels of concurrency
- **Concurrency**: confirming that correct number of goroutines is spawned and utilized
- **Ordering**: ensuring that ordered versions of functions preserve the order, while basic versions do not






## Design philosophy
At the heart of rill lies a simple yet powerful concept: operating on channels of wrapped values, encapsulated by the Try structure.
Such channels can be created manually or through utilities like **FromSlice** or **FromChan**, and then transformed via operations 
such as **Map**, **Filter**, **FlatMap** and others. Finally, when all processing stages are completed, the data can be consumed by 
**ForEach**, **ToSlice** or manually by iterating over the resulting channel.




## Batching
Batching is a common pattern in concurrent processing, especially when dealing with external services or databases.
Rill provides a **Batch** function that organizes a stream of items into batches of a specified size. It's also possible 
to specify a timeout, after which the batch is emitted even if it's not full. This is useful for keeping an application reactive
when input stream is slow or sparse.



## Fan-In and Fan-Out
The library offers mechanisms for fanning in and out data streams. Fan-in is done with the **Merge** function,
which consolidates multiple data streams into a single unified channel.
Fan-out is done with the **Split2** function, that divides a single input stream into two distinct output channels. 
This division is based on a discriminator function, allowing parallel processing paths based on data characteristics.



## Error handling
In the examples above errors are handled using **ForEach**, which is good for most use cases. 
**ForEach** stops processing on the first error and returns it. If you need to handle errors in the middle of a pipeline,
and/or continue processing after an error, there is a **Catch** function that can be used for that.

```go
results := rill.Map(input, 10, func(item int) (int, error) {
    // do some processing
})

results = rill.Catch(results, 5, func(err error) {
    if errors.Is(err, sql.ErrNoRows) {
        return nil // ignore this error
    } else {
        return fmt.Errorf("error processing item: %w", err) // wrap other errors
    }
})

err := rill.ForEach(results, 1, func(item int) error {
    // process results as usual
})
```





## Termination and resource leaks
In Go concurrent applications, if there are no readers for a channel, writers can become stuck, 
leading to potential goroutine and memory leaks. This issue extends to rill pipelines, which are built on Go channels; 
if any stage in a pipeline lacks a consumer, the whole chain of producers upstream may become blocked. 
Therefore, it's vital to ensure that pipelines are fully consumed, especially in cases where errors lead to early termination. 
The example below demonstrates a situation where the final processing stage exits upon the first encountered error, 
risking a blocked pipeline state.

```go
func doWork(ctx context.Context) error {
    // Initialize the first stage of the pipeline
    ids := streamIDs(ctx)
    
    // Define other pipeline stages...
	
    // Final stage processing
    for value := range results {
        // Process value...
        if someCondition {
            return fmt.Errorf("some error") // Early exit on error
        }
    }
    return nil
}
```

To prevent such issues, it's advisable to ensure the results channel is drained in the event of an error. 
A straightforward approach is to use defer to invoke **DrainNB**:

```go
func doWork(ctx context.Context) error {
    // Initialize the first stage of the pipeline
    ids := streamIDs(ctx)
    
    // Define other pipeline stages...
	
    // Ensure pipeline is drained in case of failure
    defer rill.DrainNB(results)
	
    // Final stage processing
    for value := range results {
        // Process value...
        if someCondition {
            return fmt.Errorf("some error") // Early exit on error
        }
    }
    return nil
}
```

Utilizing functions like **ForEach** or **ToSlice**, which incorporate built-in draining mechanisms, can simplify 
the code and enhance readability:

```go
func doWork(ctx context.Context) error {
    // Initialize the first stage of the pipeline
    ids := streamIDs(ctx)
    
    // Define other pipeline stages...

    // Final stage processing
    return rill.ForEach(results, 5, func(value string) error {
        // Process value...
        if someCondition {
            return fmt.Errorf("some error") // Early exit on error, with automatic draining
        }
        return nil
    })
}
```

While these measures are effective in preventing leaks, the pipeline may continue draining values in the background as long 
as the initial stage produces values. A best practice is to manage the first stage (and potentially others) with a context, 
allowing for a controlled shutdown:

```go
func doWork(ctx context.Context) error {
    ctx, cancel := context.WithCancel(ctx)
    defer cancel() // Ensures first stage is cancelled upon function exit

    // Initialize the first stage of the pipeline
    ids := streamIDs(ctx)

    // Define other pipeline stages...

    // Final stage processing
    return rill.ForEach(results, 5, func(value string) error {
        // Process value
        if someCondition {
            return fmt.Errorf("some error") // Early exit on error, with automatic draining
        }
        return nil
    })
}
```





## Order preservation
In concurrent environments, maintaining the original sequence of processed items is challenging due to the nature of parallel execution. 
When values are read from an input channel, processed through a function **f**, and written to an output channel, their order might not 
mirror the input sequence. To address this, rill provides ordered versions of its core functions, such as **OrderedMap**, **OrderedFilter**, 
and others. These ensure that if value **x** precedes value **y** in the input channel, then **f(x)** will precede **f(y)** in the output, 
preserving the original order. It's important to note that these ordered functions incur a small overhead compared to their unordered counterparts, 
due to the additional logic required to maintain order.

Order preservation is vital in scenarios where the sequence of data impacts the outcome. Take, for instance, an application that retrieves 
daily temperature measurements over a specific period and calculates the change in temperature from one day to the next. 
Although fetching the data in parallel boosts efficiency, processing it in the original order is crucial for 
accurate computation of temperature variations.

[Full runnable example](https://pkg.go.dev/github.com/destel/rill#example-package-Ordering)

```go
type Measurement struct {
	Date time.Time
	Temp float64
}

func main() {
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

// getTemperature fetches a temperature reading for a city and date,
func getTemperature(city string, date time.Time) (float64, error) {
	// ...
}

```