// Package rill provides composable primitives for building streaming
// pipelines over plain Go channels: functions that transform, filter, batch,
// reduce, and consume data streams, with bounded concurrency per stage,
// centralized error handling, optional order preservation, and minimal
// boilerplate.
//
// The model is similar to the Go blog's "Pipelines and cancellation"
// (https://go.dev/blog/pipelines), but it unifies error handling and
// cancellation by letting errors travel downstream along with values.
//
// # Streams
//
// In this package, a stream is a plain channel that carries both values and errors.
// Each strream item is an instance of a [Try] struct that represents either a value or an error.
// This is Go's (value, error) return convention, carried over to channels.
//
// # Stages, composition and pipelines
//
// Most functions in this package, such as [Map] or [Filter], take a
// stream as input and return a new stream as output. These functions are called stages and they:
//
//   - do not block, and return the output stream immediately
//   - process input values as they arrive
//   - write processing results to the output as they are ready
//   - forward input error items to the output as-is
//   - write processing errors to the output as they occur
//   - close the output stream after the input is fully consumed and processed
//
// Such functions, along with sources and sinks described below, are generic and can
// be used either standalone or composed into multi-stage pipelines,
// where the output of one stage is the input to the next.
//
//	ids := rill.FromSlice(userIDs)
//	filtered := rill.Filter(input, ...)
//	batches := rill.Batch(filtered, ...)
//	err := rill.ForEach(batches, ...)
//
// # Sources
//
// Every pipeline begins with a stream that is created rather than
// transformed. Any channel of [Try] structs can play this role, no matter
// where it comes from - a rill helper such as [FromSlice] or [Generate], a
// third-party library, or hand-written code. This first stream, together
// with the code feeding it, is called the source.
//
// # Sinks
//
// A sink is function that takes a stream as input, but returns
// a a regular Go value and/or an error. Sinks, such as [ForEach]
// or [MapReduce], are usually the final stage of a pipeline.
//
//   - block, until the final outcome (successful or not) is known
//   - return early (before the input is fully consumed) on the first observed error, regardless of where it came from - upstream or the sink iteself
//   - can return early because of the sink's internal logic, for example [Any] returns as soon as it finds a match
//   - on early return keep consuming and discarding the remaining input items (including
//     late errors) in the background, so upstream stages do not block and leak their goroutines
//   - can optionally report pipeline settlement (see below) via the [Scope] API
//
// # Concurrency
//
// Most stages and sinks are concurrent, and take the argument n, which
// acts as both an upper bound and a target for the number of concurrent
// invocations of the user callback. Rill never exceeds this bound, and,
// given enough input, reaches it. With n = 1, the callback is never
// invoked concurrently: items are processed one by one, in input order.
//
// # Ordered stages
//
// By default, results and errors are written to the output stream as soon
// as they are ready, so their order depends on how the Go runtime schedules
// the goroutines in the stage's worker pool, and how much time each individual
// item takes to process.
//
// This is the normal behavior of a worker pool. For cases where
// the order of outputs matters, rill provides ordered functions,
// such as [OrderedMap] or [OrderedFilter]. They stay concurrent,
// but each worker holds its result until all earlier results are sent,
// so the output order matches the input order at the cost
// of some latency. This ordering guarantee holds for both values and errors.
//
// Some stages, such as [Batch] or [Unbatch], process items sequentially and
// are naturally ordered.
//
// # Error handling
//
// Every error, wherever in the pipeline it originates, eventually reaches
// the sink, and the sink returns the first one it observes to the caller.
//
// To handle errors mid-pipeline, use [Catch]: a stage whose callback sees
// errors rather, and can handle, keep, or rewrite them.
//
// [Catch] is also handy for tracking where errors come from. The snippet
// below tags every error coming out of the source, so that later they
// can be told apart from errors raised in the stages:
//
//	var errSource = errors.New("source failed")
//
//	source = rill.Catch(source, 1, func(err error) error {
//		return fmt.Errorf("%w: %w", errSource, err)
//	})
//
// # Context, cancellation and pipeline lifecycle
//
// Rill's lifecycle and cancellation model follows from two design decisions:
//
//   - don't become a framework: pipelines are not first class objects, but
//     compositions of simpler functions that know nothing about each other
//   - streams are plain channels: data and errors can only travel downstream
//
// Together these force background draining: nothing travels upstream, so a sink
// that returns early cannot stop the stages feeding it, and abandoning them
// would block their sends forever.
//
// A pipeline goes through the following lifecycle phases:
//
//   - active: processing is in progress, sink is blocked
//   - result known: sink returned result to the caller; upstream stages might still be working, but sink drains and discards their results in the background
//   - cancelled: caller can optionally cancel a context to stop the source from producing more work and stages from doing it
//   - settled: all work is done, all user callbacks across the pipeline have returned
//
// If you do not need to wait for settlement, the cancellation model often
// collapses to nothing. Heavy network and database calls are context-aware
// by design; the sink immediately returns the first observed error to the caller;
// the caller passes that error up the stack until something handles it and
// cancels the context. That stops the remaining network calls, and the
// pipeline settles on its own in the background.
//
// When the caller must wait for settlement, use a [Scope]. It has an
// errgroup-like shape, applied to pipelines: [NewScope] derives a context,
// and [Scope.Wait] cancels it and blocks until the pipeline has settled.
// Cancellation is cooperative: rill cancels the context, and callbacks that
// captured it stop the heavy work currently in progress. [Scope.Wait]
// returns even when the source is infinite, as long as the source watches
// the context too.
//
//	scope, ctx := rill.NewScope(ctx)
//	defer scope.Cancel()
//
//	// source and other pipeline stages go here
//
//	err := rill.ForEach(stream, 5, func(x int) error {
//		return process(ctx, x)
//	}, scope)
//
//	// result known
//
//	scope.Wait() // cancel and wait for settlement
//
// Scope API can also be used for cancellation only, without calling [Scope.Wait] and waiting for settlement.
// In this case it becomes equivalent to [context.WithCancel].
//
// When the sink consumes its input to the end - no errors across the pipeline,
// no short-circuit - the pipeline is already settled by the time the sink
// returns. This property makes both [Scope] and the context redundant for computation-only pipelines that can never fail.
//
// In one special case the cost of not cancelling is bounded: a pipeline where
// all the heavy work lives in the sink's callback. Once the sink returns and
// switches to background draining, it quickly stops calling its own callback.
// This is not settlement - O(concurrency) calls may still happen, but that
// number does not depend on the remaining input size.
//
//	ids := rill.FromSlice(userIDs, nil)
//	exists, err := rill.Any(ids, 5, func(id int) (bool, error) {
//		user, err := getUser(ctx, id)
//		if err != nil {
//			return false, err
//		}
//		return user.Age > 35, nil
//	})
//
// # Extending rill
//
// Sources, stages, and sinks are ordinary functions that receive and/or
// return channels, so any user function of a similar shape works with the
// rest of the library.
//
// For example, it's easy to write a source that streams rows from
// a database table, or a sink that collects all observed errors into a
// slice.
//
// There are a few rules custom functions must follow to be compatible with rill's lifecycle.
// These rules are usually satisfied by construction.
//
//   - sources must eventually close their output stream; a source that can
//     run forever must watch a context
//   - stages must close their output stream only after the input is fully
//     consumed and all workers have returned
//   - sinks must start with a deferred rill.Discard(in, options...), followed by a
//     for-range loop that returns as soon as the sink's outcome is known
//
// Custom stages and sinks can also be built by composing existing
// rill functions, which is often the simplest way.
//
// # Backpressure
//
// In the context of Go channels, backpressure means that sending to an
// unbuffered channel blocks until the receiver on the other end is ready to
// receive. Rill naturally inherits this property: a slow stage in the
// pipeline blocks the previous stage, and it in turn blocks the stage before that,
// and so on, until the slow stage catches up.
//
// When this is not desirable, use [Buffer] to add slack between stages.
//
// # Nil handling
//
// Nil channels are valid in Go. They never emit values and are never closed.
// In practice, this means that an attempt to read from a nil channel blocks
// forever.
//
// Rill does not introduce any special semantics for nil channels. If a stage
// receives a channel that blocks forever when read, it returns a channel that
// also blocks forever. If a sink receives such a channel, the sink itself
// hangs.
//
// # Panics
//
// Rill validates arguments of its functions and panics on misuse, such that zero or negative concurrency level.
// Rill does not automatically recover panics in user callbacks: a panicking
// callback can crash the process, as it would in any hand-written concurrent
// code.
package rill
