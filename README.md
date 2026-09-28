# Rill [![GoDoc](https://pkg.go.dev/badge/github.com/destel/rill)](https://pkg.go.dev/github.com/destel/rill#pkg-overview) [![codecov](https://codecov.io/gh/destel/rill/graph/badge.svg?token=252K8OQ7E1)](https://codecov.io/gh/destel/rill) [![Mentioned in Awesome Go](https://awesome.re/mentioned-badge.svg)](https://github.com/avelino/awesome-go) 

Rill is a composable concurrency toolkit for Go, making it easier to build concurrent programs
from simple, reusable parts. It reduces boilerplate while preserving Go's natural channel-based
model.

```bash
go get github.com/destel/rill
```


## Features

- **Not a framework.**
  Rill is a collection of functions over plain channels. They can be used on their own or composed
  into multi-stage pipelines. Either way, it's straightforward to integrate rill into existing
  projects and to write custom pipeline stages.

- **Explicit concurrency.** 
  Every concurrent function takes an *n* argument that bounds how many of its callbacks run at once.

- **Centralized error handling.** 
  Errors travel downstream along with values and are handled at the end of the pipeline. They can
  also be intercepted mid-pipeline when needed.

- **Context and structured concurrency.** 
  Rill can manage a context, giving pipelines errgroup-style cancellation and waiting: cancel on
  the first error, wait until nothing is running anymore.

- **Streaming.** 
  Functions process items as they arrive, with natural backpressure, so the same code can handle a
  small slice, an input larger than memory, or an infinite stream.

- **Advanced building blocks.** 
  Batching, order preservation, streaming non-commutative reduction, map-reduce, splitting and
  merging are built in. Pipelines, while usually linear, can form any cycle-free topology.

- **Lightweight.** 
  No per-item allocations or goroutines. Small, type-safe API. Zero dependencies.


## Quick Start
Let's look at a practical example: fetch users from an API, activate them, and save the changes
back. It shows how to control concurrency at each step, and how to handle errors from both
operations in one place. On the first error it encounters, **ForEach** cancels the context,
waits until nothing is running anymore, and returns that error. The package documentation explains
this behavior in detail.

[Try in Go playground ↗](https://goplay.tools/snippet/xN_1zaBzfkq)
```go
ctx, scope := rill.WithContext(ctx)

// Convert a slice into a channel
ids := rill.FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, nil)

// Read users from the API. Concurrency = 3
users := rill.Map(ids, 3, func(id int) (*api.User, error) {
	return api.GetUser(ctx, id)
})

// Process users. Concurrency = 2
err := rill.ForEach(users, 2, func(u *api.User) error {
	if u.IsActive {
		return nil
	}
	u.IsActive = true
	return api.SaveUser(ctx, u)
}, scope) // scope is a functional option

// Nothing is running anymore; the context is canceled.
// Handle the error (if any)
fmt.Println("Error:", err)
```

To get the users back as a slice instead of processing them, just replace **ForEach** with
**ToSlice**:

```go
res, err := rill.ToSlice(users, scope)
```


## Batching

Processing items in batches rather than individually can significantly improve performance in many
scenarios, particularly when working with external services or databases. Batching reduces the
number of queries and API calls, increases throughput, and typically lowers costs.

Let's improve the previous example by using the API's bulk fetching capability. The **Batch**
function transforms a stream of individual IDs into a stream of slices. This enables the use of
`GetUsers` API to fetch multiple users in a single call, instead of making individual `GetUser`
calls.



[Try in Go playground ↗](https://goplay.tools/snippet/fpltOjeX-Le)
```go
ctx, scope := rill.WithContext(ctx)

// Convert a slice of user IDs into a channel
ids := rill.FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}, nil)

// Group IDs into batches of 5
idBatches := rill.Batch(ids, 5, -1)

// Bulk fetch users from the API. Concurrency = 3
userBatches := rill.Map(idBatches, 3, func(ids []int) ([]*api.User, error) {
	return api.GetUsers(ctx, ids)
})

// Transform the stream of batches back into a flat stream of users
users := rill.Unbatch(userBatches)

// Same as above, process users. Concurrency = 2
err := rill.ForEach(users, 2, func(u *api.User) error {
	if u.IsActive {
		return nil
	}
	u.IsActive = true
	return api.SaveUser(ctx, u)
}, scope)

// Handle the error
fmt.Println("Error:", err)
```


## Order Preservation
Regular concurrent code writes its results as soon as they're ready, in completion order. That order
depends on how the Go runtime schedules goroutines and on the time it takes to produce each result.

For cases where the input order must be preserved, rill provides ordered functions, such as
**OrderedMap** or **OrderedFilter**. They stay concurrent, but each worker holds its result until
all earlier results are sent, so the output order matches the input order at the cost of some
latency. This ordering guarantee holds for both values and errors.


Here's a practical example: check 1000 large files hosted online and find the first one containing a
given string. Downloading files sequentially is slow, while traditional concurrency patterns find
the fastest match instead of the first one.

The combination of **OrderedFilter** and **First** functions solves this, while downloading and
keeping in memory at most 5 files at a time. On the first match or error, **First**, just like
**ForEach**, cancels the context and waits for the pipeline to finish.

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

	// Keep only URLs of files that contain the needle
	return bytes.Contains(content, needle), nil
})

// Get the first matched URL or error
firstMatchedUrl, found, err := rill.First(matchedUrls, scope)

// Nothing is running anymore; the context is canceled.
// Handle the result
fmt.Println("Result:", firstMatchedUrl, found, err)
```


## Real-Time Batching
Rill’s **Batch** function is also useful for grouping independent operations happening in real time 
across an application. 

Consider an `UpdateUserTimestamp` function that's called on every user action to update the
`last_active_at` column. The function looks normal at the call site: it takes a user ID, waits for
the database to respond, and returns an error. Under the hood, a background worker uses rill to
combine concurrent calls into bulk updates and send results back to the corresponding callers.

Since calls happen at unpredictable times, waiting for a full batch can take arbitrarily long. 
To avoid this, **Batch** takes a timeout argument that limits how long each batch waits to fill. 
When the timeout expires, a partial batch is emitted.

[Try in Go playground ↗](https://goplay.tools/snippet/w0xsLilX1ca)

```go
func UpdateUserTimestamp(userID int) error {
	// Prepare a request to the worker.
	req := request{userID: userID, replyTo: make(chan error, 1)}

	// Send the request and wait for a reply.
	queue <- req
	err := <-req.replyTo
	return err
}

func updateUserTimestampWorker() {
	// Start with a stream of update requests
	requests := rill.FromChan(queue, nil)

	// Group up to 10 requests; when requests are sparse, add at most 20ms of latency
	requestBatches := rill.Batch(requests, 10, 20*time.Millisecond)

	// Send bulk updates to DB. At most 2 concurrent DB queries
	_ = rill.ForEach(requestBatches, 2, func(batch []request) error {
		// Create a slice of user IDs
		ids := make([]int, len(batch))
		for i, req := range batch {
			ids[i] = req.userID
		}

		// Do bulk update
		err := executeQuery("UPDATE users SET last_active_at = NOW() WHERE id IN (?)", ids)

		// Send result back to all callers in this batch
		for _, req := range batch {
			req.replyTo <- err
		}

		// Keep the pipeline running
		return nil
	})
}

// This type represents a single request to the worker
type request struct {
	userID  int
	replyTo chan error
}

// Queue of update requests
var queue = make(chan request)
```

## Parallel Streaming and FlatMap

Concurrent processing doesn't help when the source itself is slow. If the source can be partitioned
(and often it can), **FlatMap** can remove this bottleneck by streaming the partitions concurrently
and merging them into a single stream.

> This technique can significantly speed up scans of large S3 buckets, as described in one of the blog posts below.

In the example below, we retrieve users from a slow, paginated API by partitioning them by
department. Each department is streamed page by page. There can be arbitrarily many departments,
while **FlatMap**’s concurrency argument caps how many are streamed at once.

[Try in Go playground ↗](https://goplay.tools/snippet/ckenCrDV3eN)
```go
func main() {
	ctx, scope := rill.WithContext(context.Background())

	// Start with a stream of department names
	departments := rill.FromSlice([]string{"IT", "Finance", "Marketing", "Support", "Engineering"}, nil)

	// Stream users from all departments concurrently.
	// At most 3 departments at the same time.
	users := rill.FlatMap(departments, 3, func(department string) <-chan rill.Try[*api.User] {
		return StreamUsers(ctx, api.UserQuery{Department: department})
	})

	// Print the users from the combined stream
	err := rill.ForEach(users, 1, func(user *api.User) error {
		fmt.Printf("%+v\n", user)
		return nil
	}, scope)

	// Handle the error
	fmt.Println("Error:", err)
}

// StreamUsers streams users from a paginated API.
func StreamUsers(ctx context.Context, query api.UserQuery) <-chan rill.Try[*api.User] {
	return rill.Generate(func(send func(*api.User), sendError func(error)) {
		for page := 0; ; page++ {
			query.Page = page

			users, err := api.ListUsers(ctx, query)
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
`StreamUsers` function. Such a wrapper is useful both on its own or as part of larger
pipelines. Thanks to generic type aliases, its return type can optionally be simplified to
`rill.Stream[*api.User]`

```go
func StreamUsers(ctx context.Context, query api.UserQuery) rill.Stream[*api.User] {
	...
}
```


## Streaming Non-Commutative Reduction

Rill ships a concurrent, streaming **Reduce** function. It combines values using a user-supplied
associative, but not necessarily commutative, reducer. Under the hood, the function builds a
reduction tree.

The demo below uses string concatenation, a simple non-commutative operation. The sleep makes the
reduction cost and the concurrency gain visible.

[Try in Go playground ↗](https://goplay.tools/snippet/H4LHA5AHjz)
```go
// A stream of 62 single-character strings
str := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
letters := rill.FromSlice(strings.Split(str, ""), nil)

// Reassemble the original string. Concurrency = 4
start := time.Now()
res, _, _ := rill.Reduce(letters, 4, func(x, y string) (string, error) {
	time.Sleep(1 * time.Millisecond)
	return x + y, nil
})

fmt.Printf("Duration: %v (vs sequential %v)\n", time.Since(start), time.Duration(len(str)-1)*time.Millisecond)
fmt.Println("Result:", res)
```


## Testing Strategy
Rill's concurrency-sensitive tests use Go's [testing/synctest](https://pkg.go.dev/testing/synctest):
virtual time makes timing assertions exact, while goroutine scheduling stays nondeterministic,
so repeated runs exercise different valid interleavings and assertions must hold for all of them.

With coverage above 95%, testing focuses on:
- **Correctness**: functions produce accurate results at different levels of concurrency
- **Concurrency**: operations reach the requested callback concurrency under load
- **Ordering**: ordered versions preserve the input order, while basic versions do not
- **Lifecycle**: return and cancellation happen as early as they can, and nothing runs longer than it should
- **Leaks**: goroutines are not leaked (every synctest bubble is also a leak check)


## Blog Posts
Technical articles exploring different aspects and applications of rill's concurrency patterns:
- [Preserving Order in Concurrent Go Apps](https://destel.dev/blog/preserving-order-in-concurrent-go?ref=rill-readme)
- [Real-Time Batching in Go](https://destel.dev/blog/real-time-batching-in-go?ref=rill-readme)
- [Parallel Streaming Pattern in Go: How to Scan Large S3 or GCS Buckets Significantly Faster](https://destel.dev/blog/fast-listing-of-files-from-s3-gcs-and-other-object-storages?ref=rill-readme)


## Contributing
Thank you for your interest in improving rill! Before submitting your pull request, please consider:

- Focus on generic, widely applicable solutions
- Consider use cases. Try to avoid highly specialized features that could be separate packages
- Keep the API surface clean and focused
- Try to avoid adding functions that can be easily misused
- Avoid external dependencies 
- Add tests and documentation
- For major changes, prefer opening an issue first to discuss the approach

For bug reports and feature requests, please include a clear description and minimal example when
possible.
