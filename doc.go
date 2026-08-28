// Package rill provides composable primitives for building streaming
// pipelines over plain Go channels: functions that transform, filter, batch,
// reduce, and consume data streams, with bounded concurrency per stage,
// optional order preservation, centralized error handling, and minimal
// boilerplate.
//
// The model is similar to the Go blog's "Pipelines and cancellation"
// (https://go.dev/blog/pipelines), but it unifies error handling and
// cancellation, by letting errors travel downstream along with values.
//
// # Streams
//
// In this package, a stream is a plain channel of [Try] structs, each
// holding either a value or an error. This is Go's (value, error) return
// convention, carried over to channels. Such structs are often referred to
// as items below.
//
// # Composition and Stages
//
// Most functions in this package, such as [Map] or [Filter], take a
// stream as input and return a new stream as output. These functions:
//
//   - do not block, and return the output stream immediately
//   - process input values as they arrive
//   - write processing results to the output as they are ready
//   - forward input error items to the output as-is
//   - write processing errors to the output as they occur
//   - close the output stream after the input is fully consumed and processed
//
// Such functions are generic and can be composed into multi-stage pipelines,
// where the output of one stage is the input to the next, and the functions
// themselves are called stages.
//
//	filtered := rill.Filter(input, ...)
//	batches := rill.Batch(filtered, ...)
//	results := rill.Map(batches, ...)
//
// # Sinks
//
// A sink is a special type of stage that takes a stream as input, but returns
// a plain value and/or an error instead of a stream. Sinks, such as [ForEach]
// or [MapReduce], are usually the final stage of a pipeline. Sinks can also
// do processing on their input stream, but the lifecycle is different. Sinks:
//
//   - block until the final result (successful or not) is known
//   - return the first observed error immediately, regardless of where it
//     came from (input or processing)
//   - on early return (because of an error or the sink's internal logic),
//     keep consuming and discarding the remaining input items (including
//     late errors) in the background
//
// # Sources
//
// Every pipeline begins with a stream that is created rather than
// transformed. Any channel of [Try] structs can play this role, no matter
// where it comes from - a rill helper such as [FromSlice] or [Generate], a
// third-party library, or hand-written code. This first stream, together
// with the code feeding it, is called the source.
//
// # Extending rill
//
// Stages, sinks, and sources are ordinary functions that receive and/or
// return channels. Any user function of a similar shape is fully compatible with
// rill.
//
// For example, it's trivial to create a source that streams rows from a
// database table (just remember to close the channel when the data ends),
// or a sink that collects all observed errors into a slice. Custom reusable
// stages can also be created by composing existing rill functions.
//
// # Concurrency
//
// Most stages and sinks are concurrent, and take the argument n, which
// acts as both an upper bound and a target for the number of concurrent
// invocations of the user callback. Rill never exceeds this bound, and,
// given enough input, reaches it. With n = 1, the callback is never
// invoked concurrently: items are processed one by one, in input order.
//
// Concurrency is per stage, not per pipeline: each stage enforces its own
// limit, so an I/O-bound stage can use a much larger n than a CPU-bound
// one.
//
// # Backpressure
//
// In the context of Go channels, backpressure means that sending to an
// unbuffered channel blocks until the receiver on the other end is ready to
// receive. Rill naturally inherits this property: a slow stage in the
// pipeline blocks the previous stage, and it in turn blocks the stage before that,
// and so on, until the slow stage catches up.
//
// In cases when this is not desirable, use [Buffer] to add slack between stages.
//
// # Ordered stages
//
// By default, results are written to the output stream as they are ready, so
// the order of outputs depends on how the Go runtime schedules the goroutines
// in the worker pool, and how much time each individual item takes to
// process. This is the normal behavior of a worker pool, but sometimes the
// order of outputs matters.
//
// One solution is to disable concurrency within the stage by setting n = 1.
// Another is to use ordered functions, such as [OrderedMap]. These functions
// stay concurrent, but each worker holds its result until all earlier
// results are sent, so the output order matches the input order at the cost
// of some latency. This ordering guarantee holds for both values and errors.
//
// Some stages, such as [Batch] or [Tee], process items sequentially and
// are naturally ordered.
//
// # Error handling
//
// Stages forward errors they encounter downstream: user callbacks never see
// them. As a result, every error, no matter where it originates, eventually
// reaches the sink, which returns the first one it observes to the user code,
// where it can be handled.
//
// When errors need to be handled mid-pipeline, use [Catch].
//
// # Context and cancellation
//
// Rill itself is context-agnostic: none of its functions take a
// [context.Context]. The stopping mechanism is the user's choice - a
// context, a done channel, or any other signal the source and the callbacks
// understand.
//
// The cancellation model is cooperative and follows from three properties
// of the library:
//
//   - pipelines are not first-class objects, but compositions of simpler
//     stages, which know nothing about other stages or their in-flight
//     callbacks
//   - streams are plain channels: data and errors can only travel downstream
//   - a source can be infinite, and no stage or sink can know whether it is
//
// The entire model is built around one idea: return control to the user
// code as soon as possible, and let it stop the source from producing new
// items. All other behaviors serve this idea:
//
//   - stages forward all errors downstream
//   - the sink returns the first error it observes, without waiting for
//     in-flight callbacks to complete or for its input to end, which might
//     not even be possible if the source is infinite
//   - the sink keeps draining and discarding its input in the background,
//     so that nothing upstream is blocked during the cancellation
//
// A sink can also return early without any error - for example, [Any] does
// so when it finds a match. The model and the responsibilities stay the
// same.
//
// While this may sound complicated, in typical use cases it boils down to at
// most one deferred call, as shown in the examples below.
//
// A pipeline doing I/O. Create a cancellable context before building the
// pipeline, and defer cancel(). Stages doing database or network calls are
// typically context-aware. When the sink returns and the deferred cancel
// fires, the source and all in-flight I/O stop quickly, while the sink's
// background drain disposes of whatever the pipeline still produces, late
// errors included.
//
//	ctx, cancel := context.WithCancel(ctx)
//	defer cancel()
//
//	// context-aware source
//	ids := streamUserIDs(ctx)
//
//	// context-aware stage
//	users := rill.Map(ids, 5, func(id int) (*User, error) {
//		return db.GetUser(ctx, id)
//	})
//
//	// context-aware sink
//	return rill.ForEach(users, 5, func(u *User) error {
//		// do something with the user
//		return db.Save(ctx, u)
//	})
//
// A sink-only pipeline over a finite source - for example, a standalone
// [ForEach] over a slice. Here even defer cancel() is not strictly
// necessary: after the return, the sink switches into drain mode and
// discards the remaining input items without invoking the user's callback.
//
//	err := rill.ForEach(rill.FromSlice(finiteSource), 5, func(x int) error {
//		return doSomething(x)
//	})
//
// Manual consumption. Add a deferred [Discard] call before the loop. With no
// sink, there is no one to drain the stream on early exit, so it becomes the
// caller's job - otherwise the goroutines feeding the stream leak:
//
//	defer rill.Discard(results)
//	for res := range results {
//		if res.Error != nil {
//			return res.Error
//		}
//		// process res.Value
//	}
//
// [ToSeq2] handles this automatically and does not require a deferred call:
//
//	for value, err := range rill.ToSeq2(results) {
//		if err != nil {
//			return err
//		}
//		// process value
//	}
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
// Rill validates its arguments and panics on misuse - a concurrency level
// below one, a nil callback, an invalid batch size. Such panics happen when
// the function is called, before any item is processed, and never depend on
// the data flowing through the pipeline.
//
// Rill does not automatically recover panics in user callbacks: a panicking
// callback can crash the process, as it would in any hand-written concurrent
// code.
package rill
