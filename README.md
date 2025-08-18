# Rill [![GoDoc](https://pkg.go.dev/badge/github.com/destel/rill)](https://pkg.go.dev/github.com/destel/rill) [![Go Report Card](https://goreportcard.com/badge/github.com/destel/rill)](https://goreportcard.com/report/github.com/destel/rill) <!--[![codecov](https://codecov.io/gh/destel/rill/graph/badge.svg?token=252K8OQ7E1)](https://codecov.io/gh/destel/rill)--> [![Coverage Status](https://coveralls.io/repos/github/destel/rill/badge.svg?branch=main)](https://coveralls.io/github/destel/rill?branch=main) [![Mentioned in Awesome Go](https://awesome.re/mentioned-badge.svg)](https://github.com/avelino/awesome-go) 

Rill is a toolkit that brings composable concurrency to Go, making it easier to build concurrent programs from simple, reusable parts.
It reduces boilerplate while preserving Go's natural channel-based model and backpressure behavior.

```bash
go get -u github.com/destel/rill
```


## Goals

- **Make common tasks easier.**  
Rill provides a cleaner and safer way of solving common concurrency problems, such as parallel job execution or
real-time event processing.
It removes boilerplate and abstracts away the complexities of goroutine, channel, and error management.
At the same time, developers retain full control over the concurrency level of all operations.

- **Make concurrent code composable and clean.**  
Most functions in the library take Go channels as inputs and return new, transformed channels as outputs.
This allows them to be chained in various ways to build reusable pipelines from simpler parts,
similar to Unix pipes.
As a result, concurrent programs become clear sequences of reusable operations.

- **Centralize error handling.**  
Errors are automatically propagated through a pipeline and can be handled in a single place at the end.
For more complex scenarios, Rill also provides tools to intercept and handle errors at any point in a pipeline.

- **Simplify stream processing.**    
Thanks to Go channels, built-in functions can handle potentially infinite streams, processing items as they arrive.
This makes Rill a convenient tool for real-time processing or handling large datasets that don't fit in memory.

- **Provide solutions for advanced tasks.**  
Beyond basic operations, the library includes ready-to-use functions for batching, ordered fan-in, map-reduce, 
stream splitting, merging, and more. Pipelines, while usually linear, can have any cycle-free topology (DAG).

- **Support custom extensions.**  
Since Rill operates on standard Go channels, it's easy to write custom functions compatible with the library.

- **Keep it lightweight.**  
Rill has a small, type-safe, channel-based API, and zero dependencies, making it straightforward to integrate into existing projects.
It's also lightweight in terms of resource usage, ensuring that the number of memory allocations and goroutines
does not grow with the input size.


## Quick Start
Let's look at a practical example: fetch users from an API, activate them, and save the changes back. 
It shows how to control concurrency at each step while keeping the code clean and manageable.
**ForEach** returns on the first error, and context cancellation via defer stops all remaining fetches.


[Try it](https://pkg.go.dev/github.com/destel/rill#example-package)
```go
func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Convert a slice of user IDs into a channel
	ids := rill.FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, nil)

	// Read users from the API.
	// Concurrency = 3
	users := rill.Map(ids, 3, func(id int) (*mockapi.User, error) {
		return mockapi.GetUser(ctx, id)
	})

	// Activate users.
	// Concurrency = 2
	err := rill.ForEach(users, 2, func(u *mockapi.User) error {
		if u.IsActive {
			fmt.Printf("User %d is already active\n", u.ID)
			return nil
		}

		u.IsActive = true
		err := mockapi.SaveUser(ctx, u)
		if err != nil {
			return err
		}

		fmt.Printf("User saved: %+v\n", u)
		return nil
	})

	// Handle errors
	fmt.Println("Error:", err)
}
```


## Batching
Processing items in batches rather than individually can significantly improve performance in many scenarios, 
particularly when working with external services or databases. Batching reduces the number of queries and API calls, 
increases throughput, and typically lowers costs.

To demonstrate batching, let's improve the previous example by using the API's bulk fetching capability. 
The **Batch** function transforms a stream of individual IDs into a stream of slices. This enables the use of `GetUsers` API 
to fetch multiple users in a single call, instead of making individual `GetUser` calls.



[Try it](https://pkg.go.dev/github.com/destel/rill#example-package-Batching)
```go
func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Convert a slice of user IDs into a channel
	ids := rill.FromSlice([]int{
		1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20,
		21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40,
	}, nil)

	// Group IDs into batches of 5
	idBatches := rill.Batch(ids, 5, -1)

	// Bulk fetch users from the API
	// Concurrency = 3
	userBatches := rill.Map(idBatches, 3, func(ids []int) ([]*mockapi.User, error) {
		return mockapi.GetUsers(ctx, ids)
	})

	// Transform the stream of batches back into a flat stream of users
	users := rill.Unbatch(userBatches)

	// Activate users.
	// Concurrency = 2
	err := rill.ForEach(users, 2, func(u *mockapi.User) error {
		if u.IsActive {
			fmt.Printf("User %d is already active\n", u.ID)
			return nil
		}

		u.IsActive = true
		err := mockapi.SaveUser(ctx, u)
		if err != nil {
			return err
		}

		fmt.Printf("User saved: %+v\n", u)
		return nil
	})

	// Handle errors
	fmt.Println("Error:", err)
}
```


## Real-Time Batching
Real-world applications often need to handle events or data that arrives at unpredictable rates. While batching is still 
desirable for efficiency, waiting to collect a full batch might introduce unacceptable delays when 
the input stream becomes slow or sparse.

Rill solves this with timeout-based batching: batches are emitted either when they're full or after a specified timeout, 
whichever comes first. This approach ensures optimal batch sizes during high load while maintaining responsiveness during quiet periods.

Consider an application that needs to update users' _last_active_at_ timestamps in a database. The function responsible 
for this - `UpdateUserTimestamp` can be called concurrently, at unpredictable rates, and from different parts of the application.
Performing all these updates individually may create too many concurrent queries, potentially overwhelming the database.

In the example below, the updates are queued into `userIDsToUpdate` channel and then grouped into batches of up to 5 items, 
with each batch sent to the database as a single query.
The **Batch** function is used with a timeout of 100ms, ensuring zero latency during high load, 
and up to 100ms latency with smaller batches during quiet periods.

[Try it](https://pkg.go.dev/github.com/destel/rill#example-package-BatchingRealTime)
```go
func main() {
	// Start the background worker that processes the updates
	go updateUserTimestampWorker()

	// Do some updates. They'll be automatically grouped into
	// batches: [1,2,3,4,5], [6,7], [8]
	UpdateUserTimestamp(1)
	UpdateUserTimestamp(2)
	UpdateUserTimestamp(3)
	UpdateUserTimestamp(4)
	UpdateUserTimestamp(5)
	UpdateUserTimestamp(6)
	UpdateUserTimestamp(7)
	time.Sleep(500 * time.Millisecond) // simulate sparse updates
	UpdateUserTimestamp(8)
}

// This is the queue of user IDs to update.
var userIDsToUpdate = make(chan int)

// UpdateUserTimestamp is the public API for updating the last_active_at column in the users table
func UpdateUserTimestamp(userID int) {
	userIDsToUpdate <- userID
}

// This is a background worker that sends queued updates to the database in batches.
// For simplicity, there are no retries, error handling and synchronization
func updateUserTimestampWorker() {

	ids := rill.FromChan(userIDsToUpdate, nil)

	idBatches := rill.Batch(ids, 5, 100*time.Millisecond)

	_ = rill.ForEach(idBatches, 1, func(batch []int) error {
		fmt.Printf("Executed: UPDATE users SET last_active_at = NOW() WHERE id IN (%v)\n", batch)
		return nil
	})
}
```



## Errors, Termination and Contexts
Error handling can be non-trivial in concurrent applications. Rill simplifies this by providing a structured approach to the problem.
Pipelines typically consist of a sequence of non-blocking channel transformations, followed by a blocking stage that returns a final result and an error.
The general rule is: any error occurring anywhere in a pipeline is propagated down to the final stage,
where it's caught by some blocking function and returned to the caller.

Rill provides a wide selection of blocking functions. Here are some commonly used ones:

- **ForEach:** Concurrently applies a user function to each item in the stream.
  [Example](https://pkg.go.dev/github.com/destel/rill#example-ForEach)
- **ToSlice:** Collects all stream items into a slice.
  [Example](https://pkg.go.dev/github.com/destel/rill#example-ToSlice)
- **First:** Returns the first item or error encountered in the stream and discards the rest
  [Example](https://pkg.go.dev/github.com/destel/rill#example-First)
- **Reduce:** Concurrently reduces the stream to a single value, using a user provided reducer function.
  [Example](https://pkg.go.dev/github.com/destel/rill#example-Reduce)
- **All:** Concurrently checks if all items in the stream satisfy a user provided condition.
  [Example](https://pkg.go.dev/github.com/destel/rill#example-All)
- **Err:** Returns the first error encountered in the stream or nil, and discards the rest of the stream.
  [Example](https://pkg.go.dev/github.com/destel/rill#example-Err) 


All blocking functions share a common behavior. When they terminate early (before reaching the end of the input stream or when an error occurs),
they return immediately but spawn a background goroutine that discards the remaining items from the input channel. This prevents goroutine leaks by ensuring that
all goroutines feeding the stream are allowed to complete.

Rill is context-agnostic, meaning that it does not enforce any specific context usage.
However, it's recommended to make user-defined pipeline stages context-aware.
This is especially important for the initial stage, as it allows to stop feeding the pipeline with new items after the context cancellation.
In practice the first stage is often naturally context-aware through Go's standard APIs for databases, HTTP clients, and other external sources. 

In the example below the `CheckAllUsersExist` function uses several concurrent workers to check if all users  
from the given list exist. When an error occurs (like a non-existent user), the function returns that error  
and cancels the context, which in turn stops all remaining user fetches.

[Try it](https://pkg.go.dev/github.com/destel/rill#example-package-Context)
```go
func main() {
	ctx := context.Background()

	// ID 999 doesn't exist, so fetching will stop after hitting it.
	err := CheckAllUsersExist(ctx, 3, []int{1, 2, 3, 4, 5, 999, 7, 8, 9, 10, 11, 12, 13, 14, 15})
	fmt.Printf("Check result: %v\n", err)
}

// CheckAllUsersExist uses several concurrent workers to check if all users with given IDs exist.
func CheckAllUsersExist(ctx context.Context, concurrency int, ids []int) error {
	// Create new context that will be canceled when this function returns
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Convert the slice into a stream
	idsStream := rill.FromSlice(ids, nil)

	// Fetch users concurrently.
	users := rill.Map(idsStream, concurrency, func(id int) (*mockapi.User, error) {
		u, err := mockapi.GetUser(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch user %d: %w", id, err)
		}

		fmt.Printf("Fetched user %d\n", id)
		return u, nil
	})

	// Return the first error (if any) and cancel remaining fetches via context
	return rill.Err(users)
}
```

In the example above only the second stage (`mockapi.GetUser`) of the pipeline is context-aware.
**FromSlice** works well here since the input is small, iteration is fast and context cancellation prevents expensive API calls regardless.
The following code demonstrates how to replace **FromSlice** with **Generate** when full context awareness becomes important.

```go
idsStream := rill.Generate(func(send func(int), sendErr func(error)) {
	for _, id := range ids {
		if ctx.Err() != nil {
			return
		}
		send(id)
	}
})
```



## Order Preservation (Ordered Fan-In)
Concurrent processing can boost performance, but since tasks take different amounts of time to complete,
the results' order usually differs from the input order. While out-of-order results are acceptable in many scenarios, 
some cases require preserving the original order. This seemingly simple problem is deceptively challenging to solve correctly.

To address this, Rill provides ordered versions of its core functions, such as **OrderedMap** or **OrderedFilter**.
These functions perform additional synchronization under the hood to ensure that if value **x** precedes value **y** in the input stream,
then **f(x)** will precede **f(y)** in the output.

Here's a practical example: finding the first occurrence of a specific string among 1000 large files hosted online.
Downloading all files at once would consume too much memory, processing them sequentially would be too slow,
and traditional concurrency patterns do not preserve the order of files, making it challenging to find the first match.

The combination of **OrderedFilter** and **First** functions solves this elegantly,
while downloading and keeping in memory at most 5 files at a time. **First** returns on the first match,
this triggers the context cancellation via defer, stopping URL generation and file downloads.

[Try it](https://pkg.go.dev/github.com/destel/rill#example-package-Ordering)

```go
func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// The string to search for in the downloaded files
	needle := []byte("26")

	// Generate a stream of URLs from https://example.com/file-0.txt 
	// to https://example.com/file-999.txt
	// Stop generating URLs if the context is canceled
	urls := rill.Generate(func(send func(string), sendErr func(error)) {
		for i := 0; i < 1000 && ctx.Err() == nil; i++ {
			send(fmt.Sprintf("https://example.com/file-%d.txt", i))
		}
	})

	// Download and process the files
	// At most 5 files are downloaded and held in memory at the same time
	matchedUrls := rill.OrderedFilter(urls, 5, func(url string) (bool, error) {
		fmt.Println("Downloading:", url)

		content, err := mockapi.DownloadFile(ctx, url)
		if err != nil {
			return false, err
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


## Parallel Streaming and FlatMap
Sometimes operations that appear inherently sequential can be parallelized by partitioning the problem space. 
This can dramatically speed up data processing by allowing multiple streams to work concurrently instead of waiting 
for each to complete sequentially.

**FlatMap** is particularly powerful for this pattern. It transforms each input item into its own stream, then merges 
all these streams together, giving you full control over the level of concurrency. 

In the example below, **FlatMap** transforms each department into a stream of users, then merges these streams into one.
Like other Rill functions, **FlatMap** gives full control over concurrency. 
In this particular case the concurrency level is 3, meaning that users are fetched from at most 3 departments at the same time. 

Additionally, this example demonstrates how to write a reusable streaming wrapper over paginated API calls - the `StreamUsers` function.
This wrapper can be useful both on its own and as part of larger pipelines.

[Try it](https://pkg.go.dev/github.com/destel/rill#example-package-FlatMap)
```go
func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start with a stream of department names
	departments := rill.FromSlice([]string{"IT", "Finance", "Marketing", "Support", "Engineering"}, nil)

	// Stream users from all departments concurrently.
	// At most 3 departments at the same time.
	users := rill.FlatMap(departments, 3, func(department string) <-chan rill.Try[*mockapi.User] {
		return StreamUsers(ctx, &mockapi.UserQuery{Department: department})
	})

	// Print the users from the combined stream
	err := rill.ForEach(users, 1, func(user *mockapi.User) error {
		fmt.Printf("%+v\n", user)
		return nil
	})
	fmt.Println("Error:", err)
}

// StreamUsers is a reusable streaming wrapper around the mockapi.ListUsers function.
// It iterates through all listing pages and uses [Generate] to simplify sending users and errors to the resulting stream.
// This function is useful both on its own and as part of larger pipelines.
func StreamUsers(ctx context.Context, query *mockapi.UserQuery) <-chan rill.Try[*mockapi.User] {
	return rill.Generate(func(send func(*mockapi.User), sendErr func(error)) {
		var currentQuery mockapi.UserQuery
		if query != nil {
			currentQuery = *query
		}

		for page := 0; ; page++ {
			currentQuery.Page = page

			users, err := mockapi.ListUsers(ctx, &currentQuery)
			if err != nil {
				sendErr(err)
				return
			}

			if len(users) == 0 {
				break
			}

			for _, user := range users {
				send(user)
			}
		}
	})
}
```

**Note:** Starting from Go 1.24, thanks to generic type aliases, the return type of the `StreamUsers` function 
can optionally be simplified to `rill.Stream[*mockapi.User]`

```go
func StreamUsers(ctx context.Context, query *mockapi.UserQuery) rill.Stream[*mockapi.User] {
    ...
}
```


## Go 1.23 Iterators
Starting from Go 1.23, the language added *range-over-function* feature, allowing users to define custom iterators 
for use in for-range loops. This feature enables Rill to integrate seamlessly with existing iterator-based functions
in the standard library and third-party packages.

Rill provides **FromSeq** and **FromSeq2** functions to convert an iterator into a stream, 
and **ToSeq2** function to convert a stream back into an iterator.

**ToSeq2** can be a good alternative to **ForEach** when concurrency is not needed. 
It gives more control and performs all necessary cleanup and draining, even if the loop is terminated early using *break* or *return*.

[Try it](https://pkg.go.dev/github.com/destel/rill#example-ToSeq2)

```go
func main() {
	// Convert a slice of numbers into a stream
	numbers := rill.FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, nil)

	// Transform each number
	// Concurrency = 3
	squares := rill.Map(numbers, 3, func(x int) (int, error) {
		return square(x), nil
	})

	// Convert the stream into an iterator and use for-range to print the results
	for val, err := range rill.ToSeq2(squares) {
		if err != nil {
			fmt.Println("Error:", err)
			break // cleanup is done regardless of early exit
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


## Blog Posts
Technical articles exploring different aspects and applications of Rill's concurrency patterns:
- [Real-Time Batching in Go](https://destel.dev/blog/real-time-batching-in-go)
- [Parallel Streaming Pattern in Go: How to Scan Large S3 or GCS Buckets Significantly Faster](https://destel.dev/blog/fast-listing-of-files-from-s3-gcs-and-other-object-storages)


## Contributing
Thank you for your interest in improving Rill! Before submitting your pull request, please consider:

- Focus on generic, widely applicable solutions
- Consider use cases. Try to avoid highly specialized features that could be separate packages
- Keep the API surface clean and focused
- Try to avoid adding functions that can be easily misused
- Avoid external dependencies 
- Add tests and documentation
- For major changes, prefer opening an issue first to discuss the approach

For bug reports and feature requests, please include a clear description and minimal example when possible.
