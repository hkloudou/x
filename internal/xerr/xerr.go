// Package xerr provides error handling utilities with middleware support for observability.
//
// Key Features:
//   - Global error handling with automatic short-circuit (fail-fast semantics)
//   - No error wrapping - preserves original errors
//   - Middleware support for logging, metrics, and tracing
//   - Context-aware execution
//
// Quick Start:
//
//	ctx := context.WithValue(context.Background(), "trace_id", "req-123")
//	run := xerr.WithGlobalError(ctx, xerr.LoggerMiddleware)
//
//	var err error
//	run(&err, "step 1", step1Fn)
//	run(&err, "step 2", step2Fn)  // Automatically skipped if step 1 failed
//	run(&err, "step 3", step3Fn)  // Automatically skipped if any previous step failed
//
//	if err != nil {
//	    log.Printf("workflow failed: %v", err)
//	}
//
// Design Principles:
//   - tip parameter is for middleware observation only, NOT for error wrapping
//   - Errors are never wrapped or modified
//   - First error stops the entire flow (short-circuit)
//   - Middleware has read-only access to execution state
//
// For more examples, see example_test.go
package xerr

import (
	"context"
	"fmt"
)

// Middleware is a function that observes operation execution.
// It receives:
//   - ctx: the execution context
//   - ok: true if operation succeeded, false if it failed
//   - tip: the operation description (for logging/metrics)
//   - err: the error if operation failed, nil otherwise
//
// Note: Middleware has read-only access and cannot modify the error result.
type Middleware func(ctx context.Context, ok bool, tip string, err error)

// Run executes a single operation with middleware support.
// This is a low-level function. For multiple sequential operations, prefer WithGlobalError.
//
// Parameters:
//   - ctx: context for the operation
//   - err: pointer to error variable (if already set, execution is skipped - short-circuit)
//   - tip: description for middleware tracing (NOT used for error wrapping)
//   - fn: the operation to execute
//   - mids: optional middleware functions for observability
//
// Behavior:
//   - If *err is already set, fn is NOT executed (short-circuit on first error)
//   - If fn returns error, it's assigned to *err WITHOUT wrapping
//   - Middleware receives the tip for logging/metrics, but cannot modify the error
//
// Example (single operation):
//
//	var err error
//	xerr.Run(ctx, &err, "fetch user", fetchUserFn)
//
// Example (multiple operations - verbose):
//
//	var err error
//	xerr.Run(ctx, &err, "step1", step1Fn)
//	xerr.Run(ctx, &err, "step2", step2Fn)  // Skipped if step1 failed
//
// For cleaner code with multiple operations, use WithGlobalError instead.
func Run(ctx context.Context, err *error, tip string, fn func(context.Context) error, mids ...Middleware) {
	if *err != nil {
		return // short-circuit
	}

	e := fn(ctx)

	// Middleware uses tip for tracing, cannot modify e
	for _, mid := range mids {
		mid(ctx, e == nil, tip, e)
	}

	// Keep original error, no wrapping!
	if e != nil {
		*err = e
	}
	// Success: do nothing, *err remains nil
}

// WithGlobalError creates a reusable runner for sequential operations with shared error state.
// This is the RECOMMENDED API for handling multiple operations where any error stops the flow.
//
// The returned runner:
//   - Binds context and middleware upfront
//   - Uses a single *error pointer (global error state)
//   - Short-circuits on first failure (subsequent operations are skipped)
//   - Simplifies code when you have many sequential steps
//
// Use case: Sequential operations where any error should stop the entire workflow
// (e.g., HTTP handlers, batch jobs, multi-step transactions)
//
// Example:
//
//	ctx := context.WithValue(context.Background(), "trace_id", "req-123")
//	run := xerr.WithGlobalError(ctx, xerr.LoggerMiddleware)
//
//	var err error
//	run(&err, "validate input", validateFn)
//	run(&err, "fetch data", fetchFn)      // Skipped if validate failed
//	run(&err, "process data", processFn)   // Skipped if any previous step failed
//	run(&err, "save result", saveFn)       // Skipped if any previous step failed
//
//	if err != nil {
//	    // Handle the first error that occurred
//	}
//
// Note: For future extension, consider WithBatchError for collecting all errors without short-circuit.
func WithGlobalError(ctx context.Context, mids ...Middleware) func(*error, string, func(context.Context) error) {
	return func(err *error, tip string, fn func(context.Context) error) {
		Run(ctx, err, tip, fn, mids...)
	}
}

// LoggerMiddleware logs operation execution status to stdout.
// It extracts trace_id from context for request tracing.
// Format:
//   - Success: ✅[trace_id] tip
//   - Failure: ❌[trace_id] tip: error
//
// Example usage:
//
//	run := xerr.WithGlobalError(ctx, xerr.LoggerMiddleware)
func LoggerMiddleware(ctx context.Context, ok bool, tip string, err error) {
	if ok {
		fmt.Printf("✅[%s] %s\n", getTraceID(ctx), tip)
	} else {
		fmt.Printf("❌[%s] %s: %v\n", getTraceID(ctx), tip, err)
	}
}

// MetricsMiddleware is a placeholder for custom metrics collection.
// Uncomment and implement to integrate with Prometheus, StatsD, etc.
//
// Example implementation:
//
//	func MetricsMiddleware(ctx context.Context, ok bool, tip string, err error) {
//	    if !ok {
//	        prometheus.CounterInc("operation_failure", "operation", tip)
//	    }
//	}
//
// func MetricsMiddleware(ctx context.Context, ok bool, tip string, err error) {
// 	// Placeholder: integrate with your metrics system
// 	// e.g., prometheus.CounterInc("step_failure", "operation", tip)
// }

// getTraceID extracts trace_id from context for request tracing.
// Returns "unknown" if trace_id is not found or has wrong type.
func getTraceID(ctx context.Context) string {
	if id, ok := ctx.Value("trace_id").(string); ok {
		return id
	}
	return "unknown"
}
