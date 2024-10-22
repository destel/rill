# Rill [![GoDoc](https://pkg.go.dev/badge/github.com/destel/rill)](https://pkg.go.dev/github.com/destel/rill) [![Go Report Card](https://goreportcard.com/badge/github.com/destel/rill)](https://goreportcard.com/report/github.com/destel/rill) [![codecov](https://codecov.io/gh/destel/rill/graph/badge.svg?token=252K8OQ7E1)](https://codecov.io/gh/destel/rill) [![Mentioned in Awesome Go](https://awesome.re/mentioned-badge.svg)](https://github.com/avelino/awesome-go) 
Rill is a Go toolkit for concurrency and streaming, based on composable, channel-based transformations.

```bash
go get -u github.com/destel/rill
```


## Goals

- **Make common concurrent tasks easier.**  
Rill provides a cleaner way of solving common problems, such as processing slices or channels in parallel.
It removes boilerplate and abstracts away the complexities of goroutine orchestration and error handling.
At the same time, developers maintain full control over the concurrency level of all operations.

- **Enable composable concurrent code**  
Most built-in functions take Go channel as input and return new, transformed channel as output.
This allows them to be composed in various ways to build complex, concurrent and reusable pipelines from simpler parts.
The result is cleaner, more modular, and more maintainable code.

- **Centralize error handling.**  
Errors are automatically propagated through the pipeline and can be handled in a single place at the end.
This greatly simplifies error management in concurrent code. For more complex scenarios, Rill also provides
tools to intercept and handle errors at any point in the pipeline.

- **Simplify stream processing.**    
Thanks to Go channels, built-in functions can handle potentially infinite streams, processing items as they arrive.
This makes Rill suitable for real-time data processing, handling large datasets that don't fit in memory,
or building responsive data pipelines.

- **Provide solutions for advanced tasks.**  
The library includes ready-to-use functions for batching, ordered fan-in, map-reduce, stream splitting, merging, and more.

- **Support custom extensions.**  
Since Rill operates on standard Go channels, it's easy to write custom functions compatible with the library.

- **Keep it lightweight.**  
Rill has a small type-safe API and zero dependencies, making it straightforward to integrate into existing projects.
It's also lightweight in terms of resource usage, ensuring that the number of memory allocations and goroutines
does not grow with the input size.





## Example Usage
Consider a basic example demonstrating how **ForEach** can be used to process a list of items concurrently and 
handle errors.

[Full runnable example](https://pkg.go.dev/github.com/destel/rill#example-ForEach)
```go
func main() {
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
```


While similar result can be achieved using WaitGroup or ErrGroup from the Go standard library, let's examine a more complex example. 
It fetches users from an external API in batches, updates their status to active, and saves them back, 
while controlling the level of concurrency at each step.

[Full runnable example](https://pkg.go.dev/github.com/destel/rill#example-package-Batching)
```go
func main() {
	ctx := context.Background()

	// Start with a stream of user ids
	ids := rill.FromSlice([]int{
		1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20,
		21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40,
	}, nil)

	// Group IDs into batches of 5 for bulk processing
	idBatches := rill.Batch(ids, 5, 1*time.Second)

	// Fetch users for each batch of IDs
	// Concurrency = 3
	userBatches := rill.Map(idBatches, 3, func(ids []int) ([]*User, error) {
		return getUsers(ctx, ids...)
	})

	// Transform stream of batches back into a stream of users
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

```


## Batching
Batching is a common pattern in stream processing, especially when dealing with external services or databases.
Rill provides a **Batch** function that transforms a stream of items into a stream of batches of a specified size. It's also possible
to specify a timeout, after which a batch is emitted even if it's not full. This is useful for keeping an application reactive
when the input stream is slow or sparse.

Let's look at a simplified example that demonstrates batching with a timeout:  
Consider an _UpdateUserTimestamp_ function that updates the _last_active_at_ column in the _users_ table with the current timestamp.
This function is called concurrently from multiple places in the application, such as HTTP handlers. A large number of such calls
would cause a large number of concurrent SQL queries, potentially overwhelming the database.

To mitigate this, it's possible to group the updates and send them to the database in bulk using the **Batch** function.
On the other hand, if updates are sparse, they are delayed by at most 100ms, balancing between reducing database load and data freshness.

[Full runnable example](https://pkg.go.dev/github.com/destel/rill#example-package-Batching_updatesGrouping)
```go
func main() {
	// Start the background worker that will process the updates
	go updateUserTimestampWorker()

    // Do some updates. They'll be grouped and sent in the background.
	UpdateUserTimestamp(1)
	UpdateUserTimestamp(2)
	UpdateUserTimestamp(3)
	UpdateUserTimestamp(4)
	UpdateUserTimestamp(5)
	UpdateUserTimestamp(6)
	UpdateUserTimestamp(7)
}

// This is the queue of user IDs to update.
var userIDsToUpdate = make(chan int)

// UpdateUserTimestamp is the public function to update the last_active_at column in the users table.
func UpdateUserTimestamp(userID int) {
	userIDsToUpdate <- userID
}

func updateUserTimestampWorker() {
	ids := rill.FromChan(userIDsToUpdate, nil)

	idBatches := rill.Batch(ids, 5, 100*time.Millisecond)

	_ = rill.ForEach(idBatches, 1, func(batch []int) error {
		randomSleep(1 * time.Second)
		fmt.Printf("Executed: UPDATE users SET last_active_at = NOW() WHERE id IN (%v)\n", batch)
		return nil
	})
}
```





## Errors, Termination and Contexts
Error handling can be non-trivial in concurrent applications. Rill simplifies this by providing a structured approach to the problem.
Pipelines typically consist of a sequence of non-blocking channel transformations, followed by a blocking stage that returns the final result. 
The general rule is: any error occurring anywhere in a pipeline is propagated down to the final stage, 
where it's caught by some blocking function and returned to the caller.

Rill provides a wide selection of blocking functions. Some of them are:

- **ForEach:** Concurrently applies a user function to each item in the stream. 
  [Example](https://pkg.go.dev/github.com/destel/rill#example-ForEach)
- **ToSlice:** Collects all stream items into a slice.
  [Example](https://pkg.go.dev/github.com/destel/rill#example-ToSlice)
- **First:** Returns the first item or error encountered in the stream.
  [Example](https://pkg.go.dev/github.com/destel/rill#example-First)
- **Reduce:** Concurrently reduces the stream to a single value, using a user provided reducer function.
  [Example](https://pkg.go.dev/github.com/destel/rill#example-Reduce)
- **MapReduce:** Performs a concurrent MapReduce operation one the stream, reducing it to Go map,
  using user provided mapper and reducer functions.
  [Example](https://pkg.go.dev/github.com/destel/rill#example-MapReduce)
- **All:** Concurrently checks if all items in the stream satisfy a user provided condition.
  [Example](https://pkg.go.dev/github.com/destel/rill#example-All)
- **Any:** Concurrently checks if at least one item in the stream satisfies a user provided condition.
  [Example](https://pkg.go.dev/github.com/destel/rill#example-Any)
- **Err:** Checks if there's an error somewhere in the stream an returns it.
  [Example](https://pkg.go.dev/github.com/destel/rill#example-Err)
- **ToSeq2:** Converts a stream into an iterator of value-error pairs.
  [Example](https://pkg.go.dev/github.com/destel/rill#example-ToSeq2)



All blocking functions share a common behavior. In case of an early termination (before reaching the end of the input stream), 
such functions initiate background draining of the remaining items. This is done to prevent goroutine leaks by ensuring that 
all goroutines feeding the stream are allowed to complete.  

Rill is context-agnostic, meaning that it does not enforce any specific context usage. 
However, it's recommended to make user-defined pipeline stages context-aware.
This is especially important for the initial stage, as it allows to stop feeding the pipeline with new items when the context is canceled.

In the example below the _findFirstPrime_ function uses several concurrent workers to find the first prime number after 
a given number. Internally it creates a pipeline that starts from an infinite stream of numbers. 
When the first prime number is found, in that stream, the context gets canceled, and the pipeline terminates gracefully.

[Full runnable example](https://pkg.go.dev/github.com/destel/rill#example-package-Context)
```go
func main() {
	p := findFirstPrime(10000, 3) // Concurrency = 3
	fmt.Println("First prime after 10000:", p)
}

// findFirstPrime finds the first prime number after the given number, using several concurrent workers.
func findFirstPrime(after int, concurrency int) int {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	numbers := make(chan rill.Try[int])
	go func() {
		defer close(numbers)
		for i := after + 1; ; i++ {
			select {
			case <-ctx.Done():
				return
			case numbers <- rill.Wrap(i, nil):
			}
		}
	}()

	primes := rill.OrderedFilter(numbers, concurrency, func(x int) (bool, error) {
		return isPrime(x), nil
	})

	result, _, _ := rill.First(primes)
	return result
}

```


## Fan-in and Fan-out
Go channels support both Fan-in and Fan-out patterns, meaning that multiple goroutines can write to a single channel (fan-in)
or read from a single channel (fan-out). Many built-in functions such as **Map** or **Filter** use this pattern under the hood for concurrency.
On top of that, Rill adds a **Merge** function that can be used to combine multiple streams into a single one.

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


## Order Preservation (Ordered Fan-In)
Concurrent processing can boost performance, but every item takes different time to complete. In most cases this leads to out-of-order results.
While this is acceptable in many scenarios, there are cases when the original order of items must be preserved.

To address this, rill provides ordered versions of its core functions, such as **OrderedMap** or **OrderedFilter**.
These ensure that if value **x** precedes value **y** in the input channel, then **f(x)** will precede **f(y)** in the output,
preserving the original order.

The example below how to find the first file containing a specific string among 1000 large files hosted online.
Downloading all files at once would consume too much memory, processing them one-by-one would take too long,
and traditional concurrent patterns would lose the order of files.

The combination of **OrderedFilter** and **First** functions solves the problem, 
while downloading and holding in memory at most 5 files at the same time.

[Full runnable example](https://pkg.go.dev/github.com/destel/rill#example-package-Ordering)

```go
func main() {
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
```

## Go 1.23 Iterators
Starting from Go 1.23, the language adds *range-over-function* feature, allowing users to define custom iterators 
for use in for-range loops. This feature enables Rill to integrate seamlessly with existing iterator-based functions
in the standard library and third-party packages.

Rill provides **FromSeq** and **FromSeq2** functions to convert an iterator into a stream. 
Additionally, there's a **ToSeq2** function to convert a stream back into an iterator.

**ToSeq2** can be a good alternative to **ForEach** when concurrency is not needed. 
It gives more control and performs all necessary cleanup and draining, even if the loop is terminated early using *break* or *return*.

[Full runnable example](https://pkg.go.dev/github.com/destel/rill#example-ToSeq2)

```go
func main() {
	numbers := rill.FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, nil)

	squares := rill.Map(numbers, 1, func(x int) (int, error) {
		return x * x, nil
	})

	for val, err := range rill.ToSeq2(squares) {
		if err != nil {
			fmt.Println("Error:", err)
			break // cleanup and early exit 
		}
		fmt.Printf("%+v\n", val)
	}
}
```


## Testing Strategy
Rill has a test coverage of over 95%, with testing focused on:
- **Correctness**: ensuring that functions produce accurate results at different levels of concurrency
- **Concurrency**: confirming that correct number of goroutines is spawned and utilized
- **Ordering**: ensuring that ordered versions of functions preserve the order, while basic versions do not


## Contributing
Contributions are welcome! Whether it's reporting a bug, suggesting a feature, or submitting a pull request, your support helps improve Rill.
Please ensure that your code adheres to the existing style and includes relevant tests.
