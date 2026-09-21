// Package rill provides composable concurrency primitives: functions over
// plain channels that transform, filter, batch, reduce, and consume data
// streams while propagating errors and optionally preserving order.
//
// Rill is not a framework: its functions can be used on their own or
// composed into multi-stage pipelines. Either way, they are compatible
// with existing channel-based code.
//
// # Pipelines and streams
//
// The pipeline model in this package is similar to the one described in the
// Go blog's "Pipelines and cancellation" (https://go.dev/blog/pipelines),
// but it unifies error handling by letting errors travel downstream along with
// values. In rill's terms, the post's definition of a pipeline becomes:
//
// A pipeline is a series of stages connected by streams - channels whose items
// are [Try] structs, each holding either a value or an error. Under the hood,
// each stage runs one or more goroutines that:
//
//   - receive values and errors from upstream via input streams
//   - process the received values, usually producing new values or errors
//   - send the results downstream via output streams
//   - forward upstream errors to the output streams ([Catch] is the only
//     exception)
//
// Most stages in a pipeline have one input stream and one output stream. The
// first stage has no input stream, and the last stage has no output stream.
// These stages are called the source and the sink, respectively. The [Merge]
// and [Tee] functions have more inputs/outputs and can be used to build DAG
// pipelines.
//
//	ids := rill.FromSlice(userIDs, nil)      // source
//	filtered := rill.Filter(ids, 5, ...)     // stage, concurrency = 5
//	batches := rill.Batch(filtered, ...)     // stage
//	transformed := rill.Map(batches, 3, ...) // stage, concurrency = 3
//	err := rill.ForEach(transformed, 2, ...) // sink, concurrency = 2
//
// Intermediate stages return their output streams immediately, while their
// goroutines continue working in the background. These stages always fully
// consume and process their inputs before closing their outputs. A closed
// output becomes an "all upstream work has finished" signal that travels
// downstream along with values and errors.
//
// # Sinks
//
// Unlike intermediate stages, sinks return regular Go values, not channels.
// By default, a sink blocks until its outcome is known, then returns it,
// even if more work remains in the pipeline. Given a [WithContext] option,
// the sink blocks until the whole pipeline has finished.
//
// Every sink knows its outcome after consuming and processing the whole input.
// Some know it earlier, for example:
//
//   - [ForEach] - on the first error
//   - [Any] - on the first match or error, whichever comes first
//   - [First] - after consuming one item
//
// After an early return, a sink keeps draining and discarding any remaining
// input in the background, so upstream stages don't block forever and leak
// their goroutines. While draining, the sink suppresses its own callbacks. The
// suppression is best effort: when the sink runs callbacks concurrently, a few
// extra calls can start after the early return.
//
// # Context and structured concurrency
//
// Rill can manage the context and give the pipeline structured
// concurrency semantics similar to errgroup's.
//
//   - [WithContext] derives a context
//   - User callbacks and custom stages capture and watch the context
//   - A sink cancels the context as soon as the outcome is known (typically on
//     the first error that reaches the sink)
//   - Instead of returning the outcome immediately, the sink first waits for
//     the pipeline to finish, as errgroup's Wait does
//
// Example:
//
//	ctx, scope := rill.WithContext(ctx)
//
//	// Source and other pipeline stages go here.
//	// They can also watch ctx to stop early
//
//	// scope covers both the sink and the upstream stages
//	err := rill.ForEach(transformed, 5, func(x int) error {
//		return process(ctx, x)
//	}, scope)
//
//	// Nothing is running anymore
//
// Draining and callback suppression still apply, even though the sink no
// longer returns early.
//
// # Ordered stages
//
// By default, stages write results to their output streams as soon as they are
// ready, in completion order. In concurrent stages, that order depends on how
// the Go runtime schedules the stages' goroutines and on the time it takes to
// produce each result.
//
// For cases where the input order must be preserved, rill provides ordered
// functions, such as [OrderedMap] or [OrderedFilter]. They stay concurrent, but
// each worker holds its result until all earlier results are sent, so the
// output order matches the input order at the cost of some latency. This
// ordering guarantee holds for both values and errors.
//
// # Backpressure
//
// Backpressure means that sending to an unbuffered channel blocks until the
// receiver on the other end is ready to receive. Rill naturally inherits this
// property: a slow stage in the pipeline blocks the previous stage, and it,
// in turn, blocks the stage before that, and so on, until the slow stage
// catches up.
//
// When this is not desirable, use [Buffer] to add slack between stages.
//
// # Nil handling
//
// A nil channel never emits values and never closes, and rill treats it as
// exactly that: stages with a nil input never close their outputs and can
// leak goroutines; sinks with a nil input can block forever. Nil channels
// never make sense within a pipeline.
//
// # Panics
//
// Rill validates the arguments to its functions and panics on misuse, such as
// zero or negative concurrency. Rill does not automatically recover from panics
// in user callbacks: a panicking callback can crash the process, as it would in
// any hand-written concurrent code.
//
// # Extending rill
//
// Almost any custom function that takes or returns streams is compatible
// with rill. For example, it's easy to write a context-aware source that
// streams rows from a database table, or a sink that collects all observed
// errors into a slice.
//
// The easiest way to write a custom stage is to compose it from existing
// functions rill provides. For manually written stages, a few simple rules
// keep background draining and the "all upstream work has finished" signal
// working. Ordinary Go channel code usually satisfies most of them:
//
//   - Sources must eventually close their output stream; a source that can
//     run forever must watch a context and be cancellable
//   - Intermediate stages must close their output stream, but only after the
//     input is fully consumed and processed
//   - Non-concurrent sinks must start with a deferred
//     rill.Discard(in, options...), followed by a for-range loop that returns
//     as soon as the sink's outcome is known
package rill
