# Rill [![GoDoc](https://pkg.go.dev/badge/github.com/destel/rill)](https://pkg.go.dev/github.com/destel/rill) [![Go Report Card](https://goreportcard.com/badge/github.com/destel/rill)](https://goreportcard.com/report/github.com/destel/rill) [![codecov](https://codecov.io/gh/destel/rill/graph/badge.svg?token=252K8OQ7E1)](https://codecov.io/gh/destel/rill) [![Mentioned in Awesome Go](https://awesome.re/mentioned-badge.svg)](https://github.com/avelino/awesome-go) 

Rill is a toolkit that brings composable concurrency to Go, making it easier to build concurrent programs from simple, reusable parts.
It reduces boilerplate while preserving Go's natural channel-based model.

```bash
go get -u github.com/destel/rill
```


## Goals

- **Make common tasks easier.**  
Rill provides a clean and safe way of solving common concurrency problems, such as parallel job execution.
It removes boilerplate and abstracts away the complexities of goroutine, channel, and error management.
At the same time, developers retain full control over the concurrency level of all operations.

- **Make concurrent code composable and clean.**  
Most functions in the library take Go channels as inputs and return new, transformed channels as outputs.
This allows them to be chained in various ways to build reusable pipelines from simpler parts,
similar to Unix pipes.
As a result, concurrent tasks become clear sequences of reusable operations.

- **Centralize error handling.**  
Errors are automatically propagated through the pipeline and can be handled in a single place at the end.
For more complex scenarios, Rill also provides tools to intercept and handle errors at any point in the pipeline.

- **Simplify stream processing.**    
Thanks to Go channels, built-in functions can handle potentially infinite streams, processing items as they arrive.
This makes Rill a convenient tool for real-time data processing, handling large datasets that don't fit in memory,
or building responsive data pipelines.

- **Provide solutions for advanced tasks.**  
Beyond basic operations, the library includes ready-to-use functions for batching, ordered fan-in, map-reduce, 
stream splitting, merging, and more. Pipelines, while usually linear, 
can have any topology forming a directed acyclic graph (DAG).

- **Support custom extensions.**  
Since Rill operates on standard Go channels, it's easy to write custom functions compatible with the library.

- **Keep it lightweight.**  
Rill has a small, type-safe, channel-based API, and zero dependencies, making it straightforward to integrate into existing projects.
It's also lightweight in terms of resource usage, ensuring that the number of memory allocations and goroutines
does not grow with the input size.


## Quick Start


Rill transforms concurrent operations into clean, readable pipelines. 
Here's a practical example that fetches users from an API, activates them, and saves the changes back - 
all with explicit control over concurrency at each step. 

[Try it](https://pkg.go.dev/github.com/destel/rill#example-package)
```go
func main() {
	ctx := context.Background()

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
While processing items individually works well in many cases, it's often more efficient to handle items in batches, 
especially when working with external services or databases. This reduces the number of queries and API calls,
improves throughput, and often reduces costs

The previous example can be improved by using the API's bulk fetching capability. Instead of making individual 
`GetUser` calls, the IDs can be grouped into batches to fetch multiple users at once using a single `GetUsers` API call. 
The **Batch** function transforms a stream of individual items into a stream of batches, and **Unbatch** does the reverse.

[Try it](https://pkg.go.dev/github.com/destel/rill#example-package-Batching)
```go
func main() {
	ctx := context.Background()

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
Real-world applications often need to handle data that arrives at unpredictable rates. While batching is still 
desirable for efficiency, waiting to collect a full batch might introduce unacceptable delays when 
the input stream becomes slow or sparse.

Rill solves this with timeout-based batching: batches are emitted either when they're full or after a specified timeout, 
whichever comes first. This ensures optimal batch sizes during high load while maintaining responsiveness during quiet periods.

Consider an application that needs to update users' _last_active_at_ timestamps in a database. The function responsible 
for this - `UpdateUserTimestamp` can be called concurrently, at unpredictable rates, and from different parts of the application.
Sending all this updates individually may create too many concurrent queries, potentially overwhelming the database.

 In the example below, updates are collected into batches of up to 5 items, but a batch is also emitted if 100ms passes 
without reaching the full size. 
This provides an excellent balance between efficiency and latency: full batches and zero latency during high load, 
smaller batches and up to 100ms latency during quiet periods.

[Try it](https://pkg.go.dev/github.com/destel/rill#example-package-BatchingWithTimeout)
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

Rill provides a wide selection of blocking functions. Some of them are:

- **ForEach:** Concurrently applies a user function to each item in the stream.
  [Example](https://pkg.go.dev/github.com/destel/rill#example-ForEach)
- **ToSlice:** Collects all stream items into a slice.
  [Example](https://pkg.go.dev/github.com/destel/rill#example-ToSlice)
- **First:** Returns the first item or error encountered in the stream.
  [Example](https://pkg.go.dev/github.com/destel/rill#example-First)
- **Reduce:** Concurrently reduces the stream to a single value, using a user provided reducer function.
  [Example](https://pkg.go.dev/github.com/destel/rill#example-Reduce)
- **Any:** Concurrently checks if at least one item in the stream satisfies a user provided condition.
  [Example](https://pkg.go.dev/github.com/destel/rill#example-Any)


All blocking functions share a common behavior. In case of an early termination (before reaching the end of the input stream or in case of an error),
such functions initiate background draining of the remaining items. This is done to prevent goroutine leaks by ensuring that
all goroutines feeding the stream are allowed to complete.

Rill is context-agnostic, meaning that it does not enforce any specific context usage.
However, it's recommended to make user-defined pipeline stages context-aware.
This is especially important for the initial stage, as it allows to stop feeding the pipeline with new items when the context is canceled.

In the example below the `CheckAllUsersExist` function uses several concurrent workers to check if all users 
from the given list exist. The function returns as soon as it encounters a non-existent user. 
Such early return triggers the context cancellation, which in-turn stops all remaining users fetches.

[Try it](https://pkg.go.dev/github.com/destel/rill#example-package-Context)
```go
func main() {
	ctx := context.Background()

	// ID 999 doesn't exist, so fetching will stop after hitting it.
	err := CheckAllUsersExist(ctx, 3, []int{1, 2, 3, 4, 5, 999, 7, 8, 9, 10})
	fmt.Printf("Check result: %v\n", err)
}

// CheckAllUsersExist uses several concurrent workers to checks if all users with given IDs exist.
func CheckAllUsersExist(ctx context.Context, concurrency int, ids []int) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	idsStream := rill.FromSlice(ids, nil)

	users := rill.Map(idsStream, concurrency, func(id int) (*mockapi.User, error) {
		u, err := mockapi.GetUser(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch user %d: %w", id, err)
		}

		fmt.Printf("Fetched user %d\n", id)
		return u, nil
	})

	return rill.Err(users)
}
```


## Boosting Sequential Operations
There is a technique that significantly accelerates some seemingly sequential operations by 
branching them into multiple parallel streams and then merging the results. 
Common examples include listing S3 objects, querying APIs, or reading from databases.

Example below fetches all users from an external paginated API. Doing it sequentially, page-by-page,
would take a long time since the API is slow and the number of pages is large.
One way to speed this up is to fetch users from multiple departments at the same time.
The code below uses **FlatMap** to stream users from 3 departments concurrently and merge the results as they arrive, 
achieving up to 3x speedup compared to sequential processing.

Additionally, it demonstrates how to write a custom reusable streaming wrapper around an existing API function.
The `StreamUsers` function is useful on its own, but can also be a part of a larger pipeline.

[Try it](https://pkg.go.dev/github.com/destel/rill#example-package-ParallelStreams)
```go
func main() {
	ctx := context.Background()

	// Convert a list of all departments into a stream
	departments := rill.FromSlice(mockapi.GetDepartments())

	// Use FlatMap to stream users from 3 departments concurrently.
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
// It iterates through all listing pages and returns a stream of users.
// This function is useful on its own or as a building block for more complex pipelines.
func StreamUsers(ctx context.Context, query *mockapi.UserQuery) <-chan rill.Try[*mockapi.User] {
	res := make(chan rill.Try[*mockapi.User])

	if query == nil {
		query = &mockapi.UserQuery{}
	}

	go func() {
		defer close(res)

		for page := 0; ; page++ {
			query.Page = page

			users, err := mockapi.ListUsers(ctx, query)
			if err != nil {
				res <- rill.Wrap[*mockapi.User](nil, err)
				return
			}

			if len(users) == 0 {
				break
			}

			for _, user := range users {
				res <- rill.Wrap(user, nil)
			}
		}
	}()

	return res
}
```
 



## Order Preservation (Ordered Fan-In)
Concurrent processing can boost performance, but since tasks take different amounts of time to complete,
the results' order usually differs from the input order. This seemingly simple problem is deceptively challenging to solve correctly,
especially at scale.
While out-of-order results are acceptable in many scenarios, some cases require preserving the original order.

To address this, rill provides ordered versions of its core functions, such as **OrderedMap** or **OrderedFilter**.
These functions perform additional synchronization under the hood to ensure that if value **x** precedes value **y** in the input channel,
then **f(x)** will precede **f(y)** in the output.

Here's a practical example: finding the first occurrence of a specific string among 1000 large files hosted online.
Downloading all files at once would consume too much memory, processing them sequentially would be too slow,
and traditional concurrency patterns do not preserve the order of files, making it challenging to find the first match.

The combination of **OrderedFilter** and **First** functions solves this elegantly,
while downloading and keeping in memory at most 5 files at a time.

[Try it](https://pkg.go.dev/github.com/destel/rill#example-package-Ordering)

```go
func main() {
	ctx := context.Background()

	// The string to search for in the downloaded files
	needle := []byte("26")

	// Manually generate a stream of URLs from http://example.com/file-0.txt to http://example.com/file-999.txt
	urls := make(chan rill.Try[string])
	go func() {
		defer close(urls)
		for i := 0; i < 1000; i++ {
			// Stop generating URLs after the context is canceled (when the file is found)
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

## Go 1.23 Iterators
Starting from Go 1.23, the language adds *range-over-function* feature, allowing users to define custom iterators 
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
	results := rill.Map(numbers, 3, func(x int) (int, error) {
		return doSomethingWithNumber(x), nil
	})

	// Convert the stream into an iterator and use for-range to print the results
	for val, err := range rill.ToSeq2(results) {
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


## Contributing
Contributions are welcome! Whether it's reporting a bug, suggesting a feature, or submitting a pull request, your support helps improve Rill.
Please ensure that your code adheres to the existing style and includes relevant tests._
