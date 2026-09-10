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
// are [Try] structs, each holding either a value or an error. Under the hood, each stage
// runs one or more goroutines that:
//
//   - receive values and errors from upstream via input streams
//   - process the received values, usually producing new values or errors
//   - send the results downstream via output streams
//   - forward upstream errors to the output streams ([Catch] is the only exception)
//
// Usually, most stages in a pipeline have one input stream and one output stream.
// The exceptions are the first stage, which has no input stream, and the last
// stage, which has no output stream. These stages are called the source and
// the sink, respectively. The [Merge] and [Tee] functions have more inputs/outputs
// and can be used to build DAG pipelines.
//
//	ids := rill.FromSlice(userIDs, nil)      // source
//	filtered := rill.Filter(ids, 5, ...)     // stage, concurrency = 5
//	batches := rill.Batch(filtered, ...)     // stage
//	transformed := rill.Map(batches, 3, ...) // stage, concurrency = 3
//	err := rill.ForEach(transformed, 2, ...) // sink, concurrency = 2
//
// Intermediate stages never block: they return their output streams
// immediately, while the goroutines they started continue working in
// the background. These stages always fully consume and process their
// inputs before closing their outputs. A closed output becomes an
// "all upstream work is done" signal that travels downstream along with
// values and errors.
//
// Sinks are different: they block until the pipeline's outcome is known, which
// can happen before the input is fully consumed and all work across the pipeline is done.
// What "outcome known" means depends on the sink. For example:
//
//   - [ForEach] immediately returns the first error it observes; otherwise, it fully consumes the input
//   - [Any] can additionally short-circuit on the first match it finds
//   - [First] consumes one item and returns
//
// On an early return, a sink drains and discards the remaining input in the
// background, so upstream stages don't block forever and leak their goroutines.
//
// # Context and cancellation
//
// It's up to the caller whether to cancel the extra work that happens
// after an early return. Expensive work and large/infinite sources are usually
// context-aware, so all that's needed is to cancel the context they captured:
//
//	ctx, cancel := context.WithCancel(ctx)
//	defer cancel()
//
//	// source and other pipeline stages go here
//
//	err := rill.ForEach(transformed, 5, func(x int) error {
//		return process(ctx, x)
//	})
//
//	// outcome known; cancel manually or rely on the deferred cancel
//	cancel()
//
// # Structured concurrency
//
// When the caller wants not only to request cancellation but also to wait
// for the pipeline to settle (no work remains and every user callback has
// returned), rill provides the [Scope] API, which is like errgroup for pipelines.
//
//	scope, ctx := rill.NewScope(ctx)
//	defer scope.Cancel()
//
//	// source and other pipeline stages go here
//
//	err := rill.ForEach(transformed, 5, func(x int) error {
//		return process(ctx, x)
//	}, scope)
//
//	// outcome known
//
//	scope.Wait() // cancel ctx and wait for settlement
//
//	// it's now safe to release resources and observe side effects
//
// Under the hood, [Scope.Wait] waits for the sink's own work to finish
// and for the "all upstream work is done" signal carried by the sink's
// input streams.
//
// In computation-only pipelines that never fail or short-circuit, everything
// settles by the time the sink returns, so [Scope] is not needed.
//
// # Ordered stages
//
// By default, stages write results to their output streams as soon as they
// are ready, in completion order. In concurrent stages, that order depends on how the Go runtime
// schedules the stages' goroutines and on the time it takes to produce each result.
//
// For cases where the input order must be preserved, rill provides ordered functions,
// such as [OrderedMap] or [OrderedFilter]. They stay concurrent,
// but each worker holds its result until all earlier results are sent,
// so the output order matches the input order at the cost
// of some latency. This ordering guarantee holds for both values and errors.
//
// # Backpressure
//
// Backpressure means that sending to an unbuffered channel blocks until
// the receiver on the other end is ready to receive. Rill naturally
// inherits this property: a slow stage in the
// pipeline blocks the previous stage, and it in turn blocks the stage before that,
// and so on, until the slow stage catches up.
//
// When this is not desirable, use [Buffer] to add slack between stages.
//
// # Nil handling
//
// Rill relies on input streams eventually closing for pipelines to finish.
// Nil channels never emit values or close, so passing nil as an input
// can leak goroutines or leave a sink blocked forever.
//
// # Panics
//
// Rill validates the arguments to its functions and panics on misuse, such as zero or negative concurrency.
// Rill does not automatically recover from panics in user callbacks: a panicking
// callback can crash the process, as it would in any hand-written concurrent
// code.
//
// # Extending rill
//
// Almost any custom function that takes or returns streams is compatible
// with rill. For example, it's easy to write a context-aware source that
// streams rows from a database table, or a sink that collects all observed
// errors into a slice.
//
// The easiest way to write a custom stage is to compose it from existing
// functions rill provides. For manually written stages, there are a few
// simple rules to follow. Most of them are satisfied by construction
// and are related to preserving background drain and settlement semantics:
//
//   - sources must eventually close their output stream; a source that can
//     run forever must watch a context and be cancellable
//   - stages must close their output stream, but only after the input is fully
//     consumed and processed
//   - sinks must start with a deferred rill.Discard(in, options...), followed by a
//     for-range loop that returns as soon as the sink's outcome is known
package rill
