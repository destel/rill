# Rill [![GoDoc](https://pkg.go.dev/badge/github.com/destel/rill)](https://pkg.go.dev/github.com/destel/rill) [![Go Report Card](https://goreportcard.com/badge/github.com/destel/rill)](https://goreportcard.com/report/github.com/destel/rill) [![codecov](https://codecov.io/gh/destel/rill/graph/badge.svg?token=252K8OQ7E1)](https://codecov.io/gh/destel/rill) [![Mentioned in Awesome Go](https://awesome.re/mentioned-badge.svg)](https://github.com/avelino/awesome-go) 

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
It shows how to control concurrency at each step and how to handle errors: 
**ForEach** returns the first error it encounters


[Try in Go playground ↗](https://goplay.tools/snippet/xN_1zaBzfkq)
```go
// Convert a slice into a channel
ids := rill.FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, nil)

// Read users from the API with concurrency = 3
users := rill.Map(ids, 3, func(id int) (*mockapi.User, error) {
  return mockapi.GetUser(ctx, id)
})

// Process users with concurrency = 2
err := rill.ForEach(users, 2, func(u *mockapi.User) error {
  if u.IsActive {
    return nil
  }
  u.IsActive = true
  return mockapi.SaveUser(ctx, u)
})

// Handle errors
fmt.Println("Error:", err)
```

In rill errors are handled in one place no matter where they occur. Pipeline stages are connected by plain Go channels that carry both values and errors, so an error from any stage travels downstream to the sink (the last stage) that reports it to the caller.



## Batching

Processing items in batches rather than individually can significantly improve performance in many scenarios, 
particularly when working with external services or databases. Batching reduces the number of queries and API calls, 
increases throughput, and typically lowers costs.

To demonstrate batching, let's improve the previous example by using the API's bulk fetching capability. 
The **Batch** function transforms a stream of individual IDs into a stream of slices. This enables the use of `GetUsers` API 
to fetch multiple users in a single call, instead of making individual `GetUser` calls.



[Try in Go playground ↗](https://goplay.tools/snippet/fpltOjeX-Le)
```go
// Convert a slice of user IDs into a channel
ids := rill.FromSlice([]int{1, 2, 3, 4, 5, 6, 7,..., 38, 39, 40,}, nil)

// Group IDs into batches of 5
idBatches := rill.Batch(ids, 5, -1)

// Bulk fetch users from the API with concurrency = 3
userBatches := rill.Map(idBatches, 3, func(ids []int) ([]*mockapi.User, error) {
  return mockapi.GetUsers(ctx, ids)
})

// Transform the stream of batches back into a flat stream of users
users := rill.Unbatch(userBatches)

// Same as above, process users with concurrency = 2
err := rill.ForEach(users, 2, func(u *mockapi.User) error {
  if u.IsActive {
    return nil
  }
  u.IsActive = true
  return mockapi.SaveUser(ctx, u)
})

// Handle errors
fmt.Println("Error:", err)
```


## Real-Time Batching
Rill’s **Batch** function can also be used to batch independent operations happening in real time across an application.
For example, HTTP request handlers may need to update users’ `last_active_at` timestamps and have these independent
updates automatically combined into bulk database queries.

`UpdateUserTimestamp` looks normal at the call site: it's context aware, takes a user ID, blocks waiting for the result,
then returns nil or an error. But under the hood, a background worker combines individual calls into bulk database updates. 
The query errors (if any) are then sent back to the corresponding callers.

Waiting for a full batch to accumulate can take a long time if calls to `UpdateUserTimestamp` are rare.
We can limit this wait and process partial batches using a timeout argument.
Using a small value, like _50ms_, allows us to benefit from bulk queries when there are many concurrent updates, while
introducing at most _50ms_ of additional latency when the stream of updates is sparse.

(todo: clear the code; fix casing in the request struct)

[Try in Go playground ↗](https://goplay.tools/snippet/w0xsLilX1ca)

```go
func UpdateUserTimestamp(ctx context.Context, userID int) error {
	// Prepare a request to the worker.
	req := request{userID:  userID, ReplyTo: make(chan error, 1)}

	// Send request to the worker
	select {
	case <-ctx.Done():
		return ctx.Err()
	case queue <- req:
	}

	// Block and wait for the result 
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-req.ReplyTo:
		return err
	}
}

func updateUserTimestampWorker() {
	// Start with a stream of update requests
	requests := rill.FromChan(queue, nil)

	// Group requests into batches with timeout
	requestBatches := rill.Batch(requests, 100, 50*time.Millisecond))

	// Send bulk updates to DB with concurrency = 2
	_ = rill.ForEach(requestBatches, 2, func(batch []request) error {
		// Create a slice of user IDs
		ids := make([]int, len(batch))
		for i, req := range batch {
			ids[i] = req.userID
		}

		// Execute batched update
		err := sendQueryToDB("UPDATE users SET last_active_at = NOW() WHERE id IN (?)", ids)

		// Send result back to all callers in this batch
		for _, req := range batch {
			req.ReplyTo <- err
			close(req.ReplyTo)
		}
		return nil
	})
}

// This type represents a single request to the worker
type request struct {
	userID  int
	ReplyTo chan error
}

// This is the queue of user IDs to update.
var queue = make(chan request)
```

## Context and Structured Concurrency

In rill, the sink returns as soon as the pipeline's outcome is known. **ForEach**, for example, returns the first error it encounters without waiting for the calls still in flight in other stages. This is a deliberate choice: control returns to the caller as early as possible.

In cases when the caller needs more control over a pipeline's lifetime and cancellation, rill provides **Scope**. Think of it as errgroup for pipelines: by the time `Wait` returns, the context is canceled and nothing is running anymore, which is what structured concurrency means here. The main difference from errgroup is that in rill the outcome comes from the sink, not from `Wait`.

- `rill.NewScope` derives a cancellable context, like `errgroup.WithContext`
- Pipeline stages capture that context and watch it
- The scope is passed to the sink, which covers every stage behind it
- The sink returns the outcome
- `scope.Wait` cancels the context and waits until the pipeline settles (nothing is running anymore)

Let's modify the quick start example, adding scope to it:

```go
scope, ctx := rill.NewScope(ctx)

// Convert a slice into a stream
ids := rill.FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, nil)

// Read users from the API with concurrency = 3
users := rill.Map(ids, 3, func(id int) (*mockapi.User, error) {
	return mockapi.GetUser(ctx, id)
})

// Process users with concurrency = 2
err := rill.ForEach(users, 2, func(u *mockapi.User) error {
	if u.IsActive {
		return nil
	}
	u.IsActive = true
	return mockapi.SaveUser(ctx, u)
}, scope) // The scope is passed as an option

// Handle the error (outcome known)
fmt.Println("Error:", err)

// Cancel the context and wait for the pipeline to settle
scope.Wait()

// Settled: safe to close the API client, observe side effects, etc.
```



Depending on the use case, `Wait` can be called before or after handling the outcome. It can also be
  deferred so that the enclosing function returns only after the pipeline has settled.

```go
scope, ctx := rill.NewScope(ctx)
defer scope.Wait()
```



Scope is optional. When you don't need to wait for settlement and the surrounding code already owns cancellation, the pipeline can use the existing context directly, as the earlier examples do. A request context, for instance, is canceled when its handler returns, and any work depending on that context stops with it.




> [!NOTE]
> After a sink returns early, it keeps draining its input to prevent upstream stages from blocking and leaking their goroutines. This also enables seamless composition of stages: each stage just sends more values and errors to its output, always knowing there's a live consumer somewhere downstream. Draining handles the channels but does not stop the work; that is what the context is for.

A scope can be shared by several sinks. When a pipeline is branched with [Tee](https://pkg.go.dev/github.com/destel/rill#example-Tee) and each branch ends with its own sink, `scope.Wait`, called once every sink has returned, waits for all branches.







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

[Try in Go playground ↗](https://goplay.tools/snippet/AVigyK2JFLC)
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

[Try in Go playground ↗](https://goplay.tools/snippet/UuuV2t5xbN2)

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

[Try in Go playground ↗](https://goplay.tools/snippet/ckenCrDV3eN)
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

[Try in Go playground ↗](https://goplay.tools/snippet/M8B0xJj8btk)

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
Rill's concurrency-sensitive tests use Go's [testing/synctest](https://pkg.go.dev/testing/synctest): virtual time makes timing assertions exact,
while goroutine scheduling stays nondeterministic, so repeated runs exercise different valid interleavings and assertions must hold for all of them.
With coverage above 95%, testing focuses on:
- **Correctness**: functions produce accurate results at different levels of concurrency
- **Concurrency**: operations reach the requested callback concurrency under load
- **Ordering**: ordered versions preserve the input order, while basic versions do not
- **Leaks**: synctest-wrapped cases detect unexpected durably blocked goroutines, with explicit assertions for intentionally blocking behavior
- **Early exit**: after an error or short-circuit, blocking functions return immediately; finite upstreams drain in the background, and tests bound extra callback work


## Blog Posts
Technical articles exploring different aspects and applications of Rill's concurrency patterns:
- [Preserving Order in Concurrent Go Apps](https://destel.dev/blog/preserving-order-in-concurrent-go?ref=rill-readme)
- [Real-Time Batching in Go](https://destel.dev/blog/real-time-batching-in-go?ref=rill-readme)
- [Parallel Streaming Pattern in Go: How to Scan Large S3 or GCS Buckets Significantly Faster](https://destel.dev/blog/fast-listing-of-files-from-s3-gcs-and-other-object-storages?ref=rill-readme)


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
