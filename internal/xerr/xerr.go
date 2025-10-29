package xerr

import (
	"context"
	"fmt"
)

type Middleware func(ctx context.Context, ok bool, tip string, err error)

// Run executes a step with middleware.
// tip is ONLY for middleware tracing, NOT for error wrapping.
// *err = first non-nil error, no wrapping.
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

// With creates a reusable runner
func With(mids ...Middleware) func(context.Context, *error, string, func(context.Context) error) {
	return func(ctx context.Context, err *error, tip string, fn func(context.Context) error) {
		Run(ctx, err, tip, fn, mids...)
	}
}

// LoggerMiddleware: uses tip for tracing
func LoggerMiddleware(ctx context.Context, ok bool, tip string, err error) {
	if ok {
		fmt.Printf("✅[%s] %s\n", getTraceID(ctx), tip)
	} else {
		fmt.Printf("❌[%s] %s: %v\n", getTraceID(ctx), tip, err)
	}
}

// MetricsMiddleware: uses tip for metrics
func MetricsMiddleware(ctx context.Context, ok bool, tip string, err error) {
	// e.g., prometheus.CounterInc("step_failure", "tip", tip)
}

// getTraceID: extract from ctx (example)
func getTraceID(ctx context.Context) string {
	if id, ok := ctx.Value("trace_id").(string); ok {
		return id
	}
	return "unknown"
}
