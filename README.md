# Rill [![GoDoc](https://pkg.go.dev/badge/github.com/destel/rill)](https://pkg.go.dev/github.com/destel/rill) [![Go Report Card](https://goreportcard.com/badge/github.com/destel/rill)](https://goreportcard.com/report/github.com/destel/rill) [![codecov](https://codecov.io/gh/destel/rill/graph/badge.svg?token=252K8OQ7E1)](https://codecov.io/gh/destel/rill) [![Mentioned in Awesome Go](https://awesome.re/mentioned-badge.svg)](https://github.com/avelino/awesome-go) 

Rill is a toolkit that brings composable concurrency to Go, making it easier to build concurrent programs from simple, reusable parts.
It reduces boilerplate while preserving Go's natural channel-based model and backpressure behavior.

```bash
go get -u github.com/destel/rill
```


## Features

- **Not a framework.**  
  Rill is a collection of functions over plain channels. They can be used
  on their own or composed into multi-stage pipelines. Either way, they are
  compatible with existing channel-based code. There's no lock-in: custom functions
  are easy to write.

- **Explicit concurrency.**  
  Every concurrent function takes an *n* argument that bounds how many of its
  callbacks run at once.

- **Centralized error handling.**  
  Errors travel downstream along with values and are handled at the end of
  the pipeline. They can also be intercepted mid-pipeline when needed.

- **Context and structured concurrency.**  
  Rill can manage a context and automatically cancel it on the first error,
  then block until nothing is running anymore, giving pipelines errgroup-style semantics.

- **Streaming.**  
  Functions process items as they arrive, so rill can handle
  infinite streams and datasets larger than memory, with Go's natural
  backpressure between stages.

- **Advanced building blocks.**  
  Batching, order preservation, streaming non-commutative reduction, map-reduce,
  splitting and merging are built in. Pipelines can form any cycle-free topology.

- **Lightweight.**  
  No per-item allocations or goroutines. Small, type-safe API. Zero dependencies.


## Quick Start
Let's look at a practical example: fetch users from an API, activate them, and save the changes back. 
It shows how to control concurrency at each step, and how to handle errors from both operations in one place. 
On the first error it encounters, **ForEach** cancels the context, waits until nothing is running anymore, and returns
that error.

[Try in Go playground ↗](https://goplay.tools/snippet/xN_1zaBzfkq)
```go
ctx, scope := rill.WithContext(ctx)

// Convert a slice into a channel
ids := rill.FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, nil)

// Read users from the API with concurrency = 3
users := rill.Map(ids, 3, func(id int) (*api.User, error) {
  return api.GetUser(ctx, id)
})

// Process users with concurrency = 2
err := rill.ForEach(users, 2, func(u *api.User) error {
  if u.IsActive {
    return nil
  }
  u.IsActive = true
  return api.SaveUser(ctx, u)
}, scope)

// Nothing is running, the context is canceled.
// Handle the error (if any)
fmt.Println("Error:", err)
```




## Batching

Processing items in batches rather than individually can significantly improve performance in many scenarios, 
particularly when working with external services or databases. Batching reduces the number of queries and API calls, 
increases throughput, and typically lowers costs.

To demonstrate batching, let's improve the previous example by using the API's bulk fetching capability. 
The **Batch** function transforms a stream of individual IDs into a stream of slices. This enables the use of `GetUsers` API 
to fetch multiple users in a single call, instead of making individual `GetUser` calls.



[Try in Go playground ↗](https://goplay.tools/snippet/fpltOjeX-Le)
```go
ctx, scope := rill.WithContext(ctx)

// Convert a slice of user IDs into a channel
ids := rill.FromSlice([]int{1, 2, 3, 4, 5, 6, 7,..., 38, 39, 40,}, nil)

// Group IDs into batches of 5
idBatches := rill.Batch(ids, 5, -1)

// Bulk fetch users from the API with concurrency = 3
userBatches := rill.Map(idBatches, 3, func(ids []int) ([]*api.User, error) {
  return api.GetUsers(ctx, ids)
})

// Transform the stream of batches back into a flat stream of users
users := rill.Unbatch(userBatches)

// Same as above, process users with concurrency = 2
err := rill.ForEach(users, 2, func(u *api.User) error {
  if u.IsActive {
    return nil
  }
  u.IsActive = true
  return api.SaveUser(ctx, u)
}, scope)

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

## Order Preservation (Ordered Fan-In)
Regular concurrent code writes its results as soon as they're ready, in completion order. That order
depends on how the Go runtime schedules goroutines and on the time it takes to produce each result.

For cases where the input order must be preserved, rill provides ordered
functions, such as **OrderedMap** or **OrderedFilter**. They stay concurrent, but
each worker holds its result until all earlier results are sent, so the
output order matches the input order at the cost of some latency. This
ordering guarantee holds for both values and errors.


Here's a practical example: check 1000 large files hosted online and find the first one containing a given string.
Downloading files sequentially is slow, while traditional concurrency patterns do not preserve the order of files, 
making it challenging to find the first match.

The combination of **OrderedFilter** and **First** functions solves this,
while downloading and keeping in memory at most 5 files at a time. Before returning,
**First** cancels the context and waits until nothing is running anymore.

[Try in Go playground ↗](https://goplay.tools/snippet/UuuV2t5xbN2)

```go
ctx, scope := rill.WithContext(ctx)

// The string to search for in the downloaded files
needle := []byte("26")

// Generate a stream of URLs from file-0.txt to file-999.txt.
// Stop generating URLs when the context is canceled
urls := rill.Generate(func(send func(string), sendError func(error)) {
	for i := 0; i < 1000 && ctx.Err() == nil; i++ {
		send(fmt.Sprintf("https://example.com/file-%d.txt", i))
	}
})

// Download and process the files. Concurrency = 5
matchedUrls := rill.OrderedFilter(urls, 5, func(url string) (bool, error) {
	content, err := api.DownloadFile(ctx, url)
	if err != nil {
		return false, err
	}

	// keep only URLs of files that contain the needle
	return bytes.Contains(content, needle), nil
})

// Return the first matched URL
firstMatchedUrl, found, err := rill.First(matchedUrls, scope)

// Print the result
fmt.Println("Result:", firstMatchedUrl, found, err)
```


## Parallel Streaming and FlatMap
Sometimes operations that appear inherently sequential can be parallelized by partitioning the problem space. 
Suppose we want to get a stream of all users, but the API is slow and paginated. We can use **FlatMap** to
stream users from individual departments concurrently and combine those smaller streams into a single one.

[Try in Go playground ↗](https://goplay.tools/snippet/ckenCrDV3eN)
```go
func main() {
	ctx, scope := rill.WithContext(context.Background())

	// Start with a stream of department names
	departments := rill.FromSlice([]string{"IT", "Finance", "Marketing", "Support", "Engineering"}, nil)

	// Stream users from all departments concurrently.
	// At most 3 departments at the same time.
	users := rill.FlatMap(departments, 3, func(department string) <-chan rill.Try[*api.User] {
		return StreamUsers(ctx, &api.UserQuery{Department: department})
	})

	// Print the users from the combined stream
	err := rill.ForEach(users, 1, func(user *api.User) error {
		fmt.Printf("%+v\n", user)
		return nil
	}, scope)

	fmt.Println("Error:", err)
}

// StreamUsers streams users from a paginated API.
func StreamUsers(ctx context.Context, query *api.UserQuery) <-chan rill.Try[*api.User] {
	return rill.Generate(func(send func(*api.User), sendError func(error)) {
		var currentQuery api.UserQuery
		if query != nil {
			currentQuery = *query
		}

		for page := 0; ; page++ {
			currentQuery.Page = page

			users, err := api.ListUsers(ctx, &currentQuery)
			if err != nil {
				sendError(err)
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

This example also shows how to write a reusable streaming wrapper over paginated API calls - the
`StreamUsers` function. Such a wrapper is useful both on its own or as part of larger pipelines. 
Thanks to generic type aliases, its return type can optionally be simplified to `rill.Stream[*api.User]`

```go
func StreamUsers(ctx context.Context, query *api.UserQuery) rill.Stream[*api.User] {
    ...
}
```


## Streaming Non-Commutative Reduction

Rill ships a concurrent, streaming **Reduce** function. It combines values using a user-supplied associative,
but not necessarily commutative, reducer. Under the hood, the function builds a reduction tree.

The demo below uses string concatenation, a simple non-commutative operation.
The sleep makes the reduction cost and the concurrency gain visible.

[Try in Go playground ↗](link)
```go
// A stream of 62 single-character strings
str := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
letters := rill.FromSlice(strings.Split(str, ""), nil)

// Reassemble the original string
start := time.Now()
res, _, _ := rill.Reduce(letters, 4, func(x, y string) (string, error) {
	time.Sleep(1 * time.Millisecond)
	return x + y, nil
})

fmt.Printf("Duration: %v (vs sequential %v)\n", time.Since(start), time.Duration(len(str)-1)*time.Millisecond)
fmt.Println("Result:", res)
```


## Testing Strategy
Rill's concurrency-sensitive tests use Go's [testing/synctest](https://pkg.go.dev/testing/synctest): virtual time makes 
timing assertions exact, while goroutine scheduling stays nondeterministic, so repeated runs exercise different valid 
interleavings and assertions must hold for all of them.

With coverage above 99%, testing focuses on:
- **Correctness**: functions produce accurate results at different levels of concurrency
- **Concurrency**: operations reach the requested callback concurrency under load
- **Ordering**: ordered versions preserve the input order, while basic versions do not
- **Lifecycle**: return and cancellation happen as early as they can, and nothing runs longer than it should
- **Leaks**: goroutines are not leaked (every synctest bubble is also a leak check)

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
