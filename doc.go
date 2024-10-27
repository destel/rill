// Package rill provides composable channel-based concurrency primitives for Go that simplify parallel processing,
// batching, and stream handling. It reduces boilerplate and abstracts away the complexities of goroutine orchestration
// while offering a clean API for building concurrent pipelines from reusable parts.
// The library centralizes error handling, maintains precise control over concurrency levels, and has zero external dependencies.
//
// # Streams and Try Containers
//
// In this package, a stream refers to a channel of [Try] containers. A Try container is a simple struct that holds a value and an error.
// When an "empty stream" is referred to, it means a channel of Try containers that has been closed and was never written to.
//
// Most functions in this package are concurrent, and the level of concurrency can be controlled by the argument n.
// Some functions share common behaviors and characteristics, which are described below.
//
// # Non-blocking functions
//
// Functions such as [Map], [Filter], and [Batch] take a stream as an input and return a new stream as an output.
// They do not block and return the output stream immediately. All the processing is done in the background by the goroutine pools they spawn.
// These functions forward all errors from the input stream to the output stream.
// Any errors returned by the user-provided functions are also sent to the output stream.
// When such a function reaches the end of the input stream, it closes the output stream, stops processing and cleans up resources.
//
// Such functions are designed to be composed together to build complex processing pipelines:
//
//	stage2 := rill.Map(input, ...)
//	stage3 := rill.Batch(stage2, ...)
//	stage4 := rill.Map(stage3, ...)
//	results := rill.Unbatch(stage4, ...)
//	// consume the results and handle errors with some blocking function
//
// # Blocking functions
//
// Functions such as [ForEach], [Reduce] and [MapReduce] are used at the last stage of the pipeline
// to consume the stream and return the final result or error.
//
// Usually, these functions block until one of the following conditions is met:
//   - The end of the stream is reached. In this case, the function returns the final result.
//   - An error is encountered either in the input stream or in some user-provided function. In this case, the function returns the error.
//
// In case of an early termination (before reaching the end of the input stream), such functions initiate
// background draining of the remaining items. This is done to prevent goroutine
// leaks by ensuring that all goroutines feeding the stream are allowed to complete.
// The input stream should not be used anymore after calling such functions.
//
// It's also possible to consume the pipeline results manually, for example using a for-range loop.
// In this case, add a deferred call to [DrainNB] before the loop to ensure that goroutines are not leaked.
//
//	 defer rill.DrainNB(results)
//
//	 for res := range results {
//			if res.Error != nil {
//				return res.Error
//			}
//			// process res.Value
//	 }
//
// # Unordered functions
//
// Functions such as [Map], [Filter], and [FlatMap] write items to the output stream as soon as they become available.
// Due to the concurrent nature of these functions, the order of items in the output stream may not match the order of items in the input stream.
// These functions prioritize performance and concurrency over maintaining the original order.
//
// # Ordered functions
//
// Functions such as [OrderedMap] or [OrderedFilter] preserve the order of items from the input stream.
// These functions are still concurrent, but use special synchronization techniques to ensure that
// items are written to the output stream in the same order as they were read from the input stream.
// This additional synchronization has some overhead, but it is negligible for i/o bound workloads.
//
// Some other functions, such as [ToSlice], [Batch] or [First] are not concurrent and are ordered by nature.
//
// # Error handling
//
// Error handling can be non-trivial in concurrent applications. Rill simplifies this by providing a structured error handling approach.
// As described above, all errors are automatically propagated down the pipeline to the final stage, where they can be caught.
// This allows the pipeline to terminate after the first error is encountered and return it to the caller.
//
// In cases where more complex error handling logic is required, the [Catch] function can be used.
// It can catch and handle errors at any point in the pipeline, providing the flexibility to handle not only the first error, but any of them.
package rill
