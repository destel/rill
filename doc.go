// Package rill provides composable primitives for building concurrent streaming
// pipelines over plain Go channels: functions that transform, filter, batch,
// reduce, and consume data streams, with bounded concurrency per stage,
// centralized error handling, optional order preservation, and minimal
// boilerplate.
//
// # Pipelines and streams
//
// Rill functions can be used standalone or composed into multi-stage pipelines.
// The model is similar to the Go blog's "Pipelines and cancellation"
// (https://go.dev/blog/pipelines), but it unifies error handling by letting
// errors travel downstream along with values. In rill's terms, the post's
// definition of a pipeline becomes:
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
// Usually, in a pipeline most stages have one input stream and one output stream,
// except the first stage that has no input stream and the last stage that has no output stream.
// These stages are called the source and the sink, respectively. [Merge] and [Tee] functions
// have more inputs/outputs and can be used to build DAG pipelines.
//
//	ids := rill.FromSlice(userIDs, nil)      // source
//	filtered := rill.Filter(ids, 5, ...)     // stage, concurrency = 5
//	batches := rill.Batch(filtered, ...)     // stage
//	transformed := rill.Map(batches, 3, ...) // stage, concurrency = 3
//	err := rill.ForEach(transformed, 2, ...) // sink, concurrency = 2
//
// Intermediate stages never block: they return their output streams
// immediately, while the goroutines they started stay working in
// the background. These stages always fully consume and process their
// inputs, before closing their outputs. This closure becomes an "all
// upstream work is done" signal that travels downstream along with values
// and errors.
//
// Sinks are different, they block until the pipeline's outcome is known, which
// can happen before the input is fully consumed and all work across the pipeline is done.
// What "outcome known" means depends on the sink, for example:
//
//   - [ForEach] immediately returns the first error it observes, otherwise fully consumes the input
//   - [Any] can additionally short-circuit on the first match it finds
//   - [First] never consumes more than one item from its input
//
// On early return, a sink drains and discards the remaining input in the
// background, so upstream stages don't block forever and leak their goroutines.
//
// # Context, cancellation and settlement
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
//	// result known; cancel manually or rely on deferred cancel
//	cancel()
//
// When the caller wants not only to request cancellation but also to wait
// for the pipeline to settle (no work remains, every user callback has
// returned), rill provides the [Scope] API, which is like errgroup for pipelines.
//
//	scope, ctx := rill.NewScope(ctx)
//	defer scope.Cancel()
//
//	// source and other pipeline stages go here; the source must also
//	// watch ctx, or Wait below never returns (see [NewScope]'s example)
//
//	err := rill.ForEach(transformed, 5, func(x int) error {
//		return process(ctx, x)
//	}, scope)
//	// result known
//
//	scope.Wait() // cancel and wait for settlement
//
// Under the hood, [Scope.Wait] waits for the sink's own work to finish,
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
// schedules the stages' goroutines, and on the time each result takes to produce.
//
// For cases where the input order must be preserved, rill provides ordered functions,
// such as [OrderedMap] or [OrderedFilter]. They stay concurrent,
// but each worker holds its result until all earlier results are sent,
// so the output order matches the input order at the cost
// of some latency. This ordering guarantee holds for both values and errors.
//
// # Extending rill
//
// Rill is not a framework, but a collection of functions over plain Go
// channels. Almost any function that receives and/or returns such
// channels is compatible with rill.
//
// For example, it's easy to write a context-aware source that streams rows from
// a database table, or a sink that collects all observed errors into a
// slice.
//
// The easiest way to write a custom stage is to compose it from existing
// functions rill provides. For manually written stages there are a few
// simple rules to follow. Most of them are satisfied by construction,
// and related to preserving background drain and settlement semantics:
//
//   - sources must eventually close their output stream; a source that can
//     run forever must watch a context
//   - stages must close their output stream only after the input is fully
//     consumed and processed
//   - sinks must start with a deferred rill.Discard(in, options...), followed by a
//     for-range loop that returns as soon as the sink's outcome is known
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
// Nil channels are valid in Go. They never emit values and are never closed.
// In practice, this means that an attempt to read from a nil channel blocks
// forever.
//
// Rill does not introduce any special semantics for nil channels. If a stage
// receives a stream that's never closed, it never closes its output stream.
// If a sink receives such a stream, the sink itself blocks forever.
//
// # Panics
//
// Rill validates arguments of its functions and panics on misuse, such as zero or negative concurrency.
// Rill does not automatically recover panics in user callbacks: a panicking
// callback can crash the process, as it would in any hand-written concurrent
// code.
package rill
