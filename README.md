# Rill [![GoDoc](https://pkg.go.dev/badge/github.com/destel/rill)](https://pkg.go.dev/github.com/destel/rill) [![Go Report Card](https://goreportcard.com/badge/github.com/destel/rill)](https://goreportcard.com/report/github.com/destel/rill) [![codecov](https://codecov.io/gh/destel/rill/graph/badge.svg?token=252K8OQ7E1)](https://codecov.io/gh/destel/rill) 
Rill (noun: a small stream) is a Go concurrency toolkit that offers a collection of easy-to-use functions for streaming, 
parallel processing and pipeline construction. It abstracts away the complexities of concurrency and removes boilerplate, 
enabling developers to focus on core logic. Whether you need to perform a basic concurrent ForEach or 
construct a complex multi-stage processing pipeline, Rill has got you covered.


## Key Features
- **Easy to Use**: the complexity of managing goroutines, channels, wait groups, and atomics is abstracted away
- **Easy to Integrate**: seamlessly integrates into existing projects without any setup or configuration
- **Concurrent**: provides control over the level of concurrency for all operations
- **Error Handling**: provides a structured way to handle errors in concurrent applications
- **Streaming**: handles real-time data streams or large datasets with a minimal memory footprint
- **Modular**: allows composing functions to create custom pipelines and higher-order operations
- **Batching**: simplifies organizing and processing data in batches
- **Order Preservation**: provides functions that maintain the original order of data during concurrent processing
- **Efficient Resource Use**: ensures goroutine pool sizes and memory allocations are independent of input size
- **Generic**: all operations are type-safe and can be used with any data type


## Installation
```bash
go get github.com/destel/rill
```


## Example Usage
Consider an example application that loads users from an API concurrently,
updates their status to active and saves them back, 
while controlling the level of concurrency for each operation.

[Full runnable example](https://pkg.go.dev/github.com/destel/rill#example-package)

```go
func main() {
	// In case of early exit this will cancel the file streaming,
	// which in turn will terminate the entire pipeline.	
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
```




## Testing Strategy
Rill has a test coverage of over 95%, with testing focused on:
- **Correctness**: ensuring that functions produce accurate results at different levels of concurrency
- **Concurrency**: confirming that correct number of goroutines is spawned and utilized
- **Ordering**: ensuring that ordered versions of functions preserve the order, while basic versions do not






## Design Philosophy
At the heart of rill lies a simple yet powerful concept: operating on channels of wrapped values, encapsulated by the **Try** structure.
This allows to propagate both values and errors through the pipeline, ensuring that errors are handled correctly at each stage.
Such wrapped channels can be created manually or through utilities like **FromSlice** or **FromChan**, and then transformed via non-blocking 
functions like **Map** or **Filter**. Finally, the transformed stream can be consumed by a blocking function such as 
**ForEach**, **Reduce** or **MapReduce**

One of the key features of Rill is the ability to control the level of concurrency for almost all operations through the **n** parameter.
This is possible due to the channel and goroutine orchestration that library does under the hood. Rill's built-in functions manage
worker pools internally, making the number of goroutines and allocations independent of the input size.

Finally, rill is designed to be modular and extensible. Most functions take streams as input and return transformed streams as output, 
It's easy to create custom reusable higher-order operations and pipelines by combining existing ones.




## Batching
Batching is a common pattern in concurrent processing, especially when dealing with external services or databases.
Rill provides a **Batch** function that transforms a stream of items into a stream of batches of a specified size. It's also possible 
to specify a timeout, after which the batch is emitted even if it's not full. This is useful for keeping an application reactive
when input stream is slow or sparse.

Consider a modification of the example above, with list of user ids streamed from a remote file,
and users fetched from the API in batches.

[Full runnable example](https://pkg.go.dev/github.com/destel/rill#example-package-Batching)

```go
func main() {
	// In case of early exit this will cancel the file streaming,
	// which in turn will terminate the entire pipeline.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Stream a file with user ids as an io.Reader
	reader, err := downloadFile(ctx, "http://example.com/user_ids1.txt")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	// Transform the reader into a stream of words
	lines := streamLines(reader)

	// Parse lines as integers
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

	// Activate users.
	// Concurrency = 2
	err = rill.ForEach(users, 2, func(u *User) error {
		if u.IsActive {
			fmt.Printf("User %d is already active\n", u.ID)
			return nil
		}

		u.IsActive = true
		return saveUser(ctx, u)
	})

	fmt.Println("Error:", err)	
}
```



## Fan-in and Fan-out
Go channels support both Fan-in and Fan-out patterns, meaning that multiple goroutines can write to a single channel (fan-in)
or read from a single channel (fan-out). On top of that Rill adds a Merge function that can be used to combine multiple streams into a single one.

Consider a basic example application that concurrently sends messages through multiple servers, then collects the results
into a single stream and handles errors.

[Full runnable example](https://pkg.go.dev/github.com/destel/rill#example-package-FanIn_FanOut)

```go
func main() {
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
```

## Errors, Termination and Contexts
Usually rill pipelines consist of zero or more non-blocking stages that transform the input stream, 
and one blocking stage that consumes the results. General rule is: any error happening anywhere in the pipeline is 
propagated down the pipeline, where it is caught and returned to the caller by some blocking function.

Rill provides several blocking functions out of the box:

- **ForEach:** Concurrently applies a user function to each item in the stream. 
  [Example](https://pkg.go.dev/github.com/destel/rill#example-ForEach)
- **ToSlice:** Collects all stream items into a slice.
  [Example](https://pkg.go.dev/github.com/destel/rill#example-ToSlice)
- **Reduce:** Concurrently reduces the stream to a single value using a user provided reducer function.
  [Example](https://pkg.go.dev/github.com/destel/rill#example-Reduce)
- **MapReduce:** Performs a concurrent MapReduce operation one the stream, reducing it to Go map.
  Takes two user provided functions: mapper and reducer.
  [Example](https://pkg.go.dev/github.com/destel/rill#example-MapReduce)
- **All:** Concurrently checks if all items in the stream satisfy a user provided condition.
  [Example](https://pkg.go.dev/github.com/destel/rill#example-All)
- **Any:** Concurrently checks if at least one item in the stream satisfies a user provided condition.
  [Example](https://pkg.go.dev/github.com/destel/rill#example-Any)
- **First:** Returns the first item or error encountered in the stream.
  [Example](https://pkg.go.dev/github.com/destel/rill#example-First)
- **Err:** Returns the first error encountered in the stream.
  [Example](https://pkg.go.dev/github.com/destel/rill#example-Err)




All blocking functions share a common behavior. In case of early termination due to an error or other conditions, 
they keep draining the input stream in the background until it's fully consumed. This is done to prevent goroutine leaks 
by ensuring that all goroutines feeding the input stream are allowed to complete. 
It also possible to consume the stream manually by using the for-range loop. In this case, the caller would be responsible for
draining the stream. See more details in the package documentation.

Rill is context-agnostic, meaning that it does not enforce any specific context usage. 
However, it's recommended to make user-defined pipeline stages context-aware.
This is especially important for the initial stage, as it allows to finish background draining
process, described above, faster.

In the example below the printOddSquares function initiates a pipeline that depends on a context.
When an error occurs in one of the pipeline stages, it propagates down the pipeline, causing an early exit, 
context cancellation (via defer) and resource cleanup.

[Full runnable example](https://pkg.go.dev/github.com/destel/rill#example-package-Context)

```go
func main() {
	ctx := context.Background()

	err := printOddSquares(ctx)
	fmt.Println("Error:", err)
}

func printOddSquares(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	numbers := infiniteNumberStream(ctx)

	odds := rill.Filter(numbers, 3, func(x int) (bool, error) {
		if x == 20 {
			return false, fmt.Errorf("early exit")
		}
		return x%2 == 1, nil
	})

	return rill.ForEach(odds, 3, func(x int) error {
		fmt.Println(x * x)
		return nil
	})
}
```



## Order Preservation
In concurrent applications, maintaining the original sequence of processed items is challenging due to the nature of parallel execution. 
When values are read from an input stream, processed through a function **f**, and written to an output stream, their order might not 
match the order of the input. To address this, rill provides ordered versions of its core functions, such as **OrderedMap**, **OrderedFilter**, 
and others. These ensure that if value **x** precedes value **y** in the input channel, then **f(x)** will precede **f(y)** in the output, 
preserving the original order. It's important to note that these ordered functions have a small overhead compared to their unordered counterparts, 
due to more advanced orchestration and synchronization happening under the hood.

Order preservation is vital in scenarios where the sequence of data impacts the outcome, such as time-series data processing. 
Take, for instance, an application that retrieves daily temperature measurements over a specific period and calculates the change 
in temperature from one day to the next. Such application can benefit from concurrent data fetching, but need fetched data
to be processed in the correct order.

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
```


## Limitations
While rill provides a powerful and expressive way to build concurrent pipelines, there are certain limitations and 
scenarios where alternative approaches might be more suitable.

Go channels are a fundamental and convenient feature for handling concurrency and communication between goroutines. 
However, it's important to note that channels come with a certain overhead. The impact of this overhead varies depending on 
the specific use:

- I/O-bound tasks: Channels are great for handling I/O-bound tasks, such as reading from or writing to files, 
  network communication, or database operations. The overhead of channels is typically negligible compared to 
  the time spent waiting for I/O operations to complete
- Small CPU-bound tasks: When parallelizing a large number of small CPU-bound tasks, such as simple arithmetic operations, 
  the overhead of channels can become significant. In such cases, using channels and goroutines may not provide 
  the desired performance benefits
- Heavy CPU-bound tasks: For computationally intensive tasks, such as complex string manipulation, encryption, 
  or hash calculation, the overhead of channels becomes less significant compared to the overall processing time. 
  In these scenarios, using channels and rill can still provide an efficient way to parallelize the workload

If your use case involves high-performance calculations and you want to minimize the overhead of channels, 
you can consider alternative approaches or libraries. For example, it's possible to transform a slice without channels and 
with almost zero orchestration, just by dividing the slice into n chunks and assigning each chunk to a separate goroutine.

Because of the reasons mentioned above and to manage users' expectations, rill does not provide functions that operate 
directly on slices. It's main focus is streaming. However, slices can still be used with rill by converting them to and from channels, and leveraging 
ordered transformations when necessary. 

Another limitation of rill is that it does not provide a way to create a global worker pool for the entire pipeline. 
Each stage of the pipeline must have at least one alive goroutine to keep the whole pipeline running. 
That's why each stage has its own goroutine pool, which is created and managed internally.