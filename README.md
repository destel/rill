# Rill
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
- **Efficient Resource Use**: the number of goroutines and allocations does not depend on data size
- **Generic**: all operations are type-safe and can be used with any data type
- **Functional Programming**: based on functional programming concepts, making operations like map, filter, flatMap and others available for channel-based workflows

## Installation
```bash
go get github.com/destel/rill
```

## Example
A function that fetches keys from multiple URLs, retrieves their values from a Redis database, and prints them. 
This example demonstrates the library's strengths in handling concurrent tasks, error propagation, batching and data streaming, 
all while maintaining simplicity and efficiency.
See full runnable example at examples/redis-read/main.go

```go
type KV struct {
    Key   string
    Value string
}


func printValuesFromRedis(ctx context.Context, urls []string) error {
    ctx, cancel := context.WithCancel(ctx)
    defer cancel() // In case of error, this ensures all http and redis operations are canceled

    // Convert urls into a channel
    urlsChan := rill.WrapSlice(urls)
    
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
    // ...
}




```


## Design philosophy
At the heart of rill lies a simple yet powerful concept: operating on channels of wrapped values, encapsulated by the Try structure.
Such channels can be created manually or through utilities like **WrapSlice** or **WrapChan**, and then transformed via operations 
such as **Map**, **Filter**, **FlatMap** and others. Finally when all processing stages are completed, the data can be consumed by 
**ForEach**, **UnwrapToSlice** or manually by iterating over the resulting channel.



## Batching
Batching is a common pattern in concurrent processing, especially when dealing with external services or databases.
Rill provides a Batch function that organizes a stream of items into batches of a specified size. It's also possible 
to specify a timeout, after which the batch is emitted even if it's not full. This is useful for keeping an app reactive
when input stream is slow or sparse.





## Error handling
In the examples above errors are handled using **ForEach**, which is good for most use cases. 
**ForEach** stops processing on the first error and returns it. If you need to handle error in the middle of pipeline,
and continue processing, there is a **Catch** function that can be used for that.

```go
results := echans.Map(input, 10, func(item int) (int, error) {
    // do some processing
})

results = echans.Catch(results, 5, func(err error) {
    if errors.Is(err, sql.ErrNoRows) {
        return nil // ignore this error
    } else {
        return fmt.Errorf("error processing item: %w", err) // wrap error and continue processing
    }
})

err := echans.ForEach(results, 1, func(item int) error {
    // process results as usual
})
```


## Order preservation
There are use cases where it's necessary to preserve the original order of data, while still allowing for concurrent processing.
Below is an example function that fetches temperature measurements for each day in a specified range
and prints temperature movements for each day. OrderedMap function fetches measurements in parallel, but returns them in chronological order.
This allows the next stage of processing to calculate temperature differences between consecutive days.
See full runnable example at examples/weather/main.go

```go
type Measurement struct {
	Date     time.Time
	Temp     float64
	Movement float64
}

func printTemperatureMovements(ctx context.Context, city string, startDate, endDate time.Time) error {
    ctx, cancel := context.WithCancel(ctx)
    defer cancel() // In case of error, this ensures all pending operations are canceled
    
    // Make a channel that emits all the days between startDate and endDate
    days := make(chan rill.Try[time.Time])
    go func() {
        defer close(days)
        for date := startDate; date.Before(endDate); date = date.AddDate(0, 0, 1) {
            days <- rill.Wrap(date, nil)
        }
    }()
    
    // Download the temperature for each day in parallel and in order
    measurements := rill.OrderedMap(days, 10, func(date time.Time) (Measurement, error) {
        temp, err := getTemperature(ctx, city, date)
        return Measurement{Date: date, Temp: temp}, err
    })
    
    // Calculate the temperature movements. Use a single goroutine
    prev := Measurement{Temp: math.NaN()}
    measurements = rill.OrderedMap(measurements, 1, func(m Measurement) (Measurement, error) {
        m.Movement = m.Temp - prev.Temp
        prev = m
        return m, nil
    })
    
    // Iterate over the measurements and print the movements
    err := rill.ForEach(measurements, 1, func(m Measurement) error {
        fmt.Printf("%s: %.1f°C (movement %+.1f°C)\n", m.Date.Format("2006-01-02"), m.Temp, m.Movement)
        prev = m
        return nil
    })
    
    return err
}

// getTemperature does a network request to fetch the temperature for a given city and date.
func getTemperature(ctx context.Context, city string, date time.Time) (float64, error) {
    // ...
}
```