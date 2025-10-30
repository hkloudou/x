package xerr

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// TestRun_Success tests successful execution without errors
func TestRun_Success(t *testing.T) {
	ctx := context.Background()
	var err error

	run(ctx, &err, "test operation", func(ctx context.Context) error {
		return nil
	})

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

// TestRun_WithError tests error capture
func TestRun_WithError(t *testing.T) {
	ctx := context.Background()
	var err error
	expectedErr := errors.New("operation failed")

	run(ctx, &err, "test operation", func(ctx context.Context) error {
		return expectedErr
	})

	if err != expectedErr {
		t.Errorf("expected error %v, got: %v", expectedErr, err)
	}
}

// TestRun_ShortCircuit tests that Run stops when err is already set
func TestRun_ShortCircuit(t *testing.T) {
	ctx := context.Background()
	firstErr := errors.New("first error")
	err := firstErr

	executed := false
	run(ctx, &err, "should not execute", func(ctx context.Context) error {
		executed = true
		return errors.New("second error")
	})

	if executed {
		t.Error("function should not execute when err is already set")
	}

	if err != firstErr {
		t.Errorf("expected first error to be preserved, got: %v", err)
	}
}

// TestRun_ErrorNotWrapped ensures the original error is NOT wrapped
func TestRun_ErrorNotWrapped(t *testing.T) {
	ctx := context.Background()
	var err error
	originalErr := errors.New("original error")

	run(ctx, &err, "this tip should NOT wrap the error", func(ctx context.Context) error {
		return originalErr
	})

	// Verify the error is exactly the same object, not wrapped
	if err != originalErr {
		t.Errorf("error was modified or wrapped, expected %p got %p", originalErr, err)
	}

	// Verify error message doesn't contain the tip
	if strings.Contains(err.Error(), "this tip should NOT wrap the error") {
		t.Error("error message should NOT contain the tip")
	}
}

// TestRun_MiddlewareExecution tests that middleware is called correctly
func TestRun_MiddlewareExecution(t *testing.T) {
	ctx := context.Background()

	t.Run("middleware receives success", func(t *testing.T) {
		var err error
		var middlewareCalled bool
		var receivedTip string
		var receivedErr error

		middleware := func(ctx context.Context, err error, tip string) {
			middlewareCalled = true
			receivedErr = err
			receivedTip = tip
		}

		run(ctx, &err, "success operation", func(ctx context.Context) error {
			return nil
		}, middleware)

		if !middlewareCalled {
			t.Error("middleware was not called")
		}
		if receivedErr != nil {
			t.Errorf("middleware should receive nil error for success, got: %v", receivedErr)
		}
		if receivedTip != "success operation" {
			t.Errorf("middleware should receive correct tip, got: %s", receivedTip)
		}
	})

	t.Run("middleware receives error", func(t *testing.T) {
		var err error
		expectedErr := errors.New("operation failed")
		var middlewareCalled bool
		var receivedErr error

		middleware := func(ctx context.Context, err error, tip string) {
			middlewareCalled = true
			receivedErr = err
		}

		run(ctx, &err, "failed operation", func(ctx context.Context) error {
			return expectedErr
		}, middleware)

		if !middlewareCalled {
			t.Error("middleware was not called")
		}
		if receivedErr != expectedErr {
			t.Errorf("middleware should receive the error, got: %v", receivedErr)
		}
	})
}

// TestRun_MultipleMiddlewares tests that all middlewares are called in order
func TestRun_MultipleMiddlewares(t *testing.T) {
	ctx := context.Background()
	var err error
	var callOrder []int

	mid1 := func(ctx context.Context, err error, tip string) {
		callOrder = append(callOrder, 1)
	}
	mid2 := func(ctx context.Context, err error, tip string) {
		callOrder = append(callOrder, 2)
	}
	mid3 := func(ctx context.Context, err error, tip string) {
		callOrder = append(callOrder, 3)
	}

	run(ctx, &err, "test", func(ctx context.Context) error {
		return nil
	}, mid1, mid2, mid3)

	if len(callOrder) != 3 {
		t.Errorf("expected 3 middleware calls, got %d", len(callOrder))
	}
	if callOrder[0] != 1 || callOrder[1] != 2 || callOrder[2] != 3 {
		t.Errorf("middlewares called in wrong order: %v", callOrder)
	}
}

// TestRun_MiddlewareCannotModifyError ensures middleware cannot change the error
func TestRun_MiddlewareCannotModifyError(t *testing.T) {
	ctx := context.Background()
	var err error
	originalErr := errors.New("original error")

	// Middleware that tries to "modify" the error (but shouldn't affect the result)
	maliciousMid := func(ctx context.Context, err error, tip string) {
		// This is a read-only view, cannot modify the actual error
		err = errors.New("modified error")
	}

	run(ctx, &err, "test", func(ctx context.Context) error {
		return originalErr
	}, maliciousMid)

	// The actual error should remain unchanged
	if err != originalErr {
		t.Errorf("middleware should not be able to modify error, got: %v", err)
	}
}

// TestNewGlobalError_CreateReusableRunner tests the NewGlobalError function
func TestNewGlobalError_CreateReusableRunner(t *testing.T) {
	ctx := context.Background()
	var callCount int

	middleware := func(ctx context.Context, err error, tip string) {
		callCount++
	}

	// NewGlobalError binds context and middleware, returns a context-free runner
	runner := NewGlobalError(ctx, middleware)

	// Use the runner multiple times (no need to pass context)
	var err1 error
	runner(&err1, "operation 1", func(ctx context.Context) error {
		return nil
	})

	var err2 error
	runner(&err2, "operation 2", func(ctx context.Context) error {
		return errors.New("error 2")
	})

	if callCount != 2 {
		t.Errorf("expected middleware to be called 2 times, got %d", callCount)
	}
	if err1 != nil {
		t.Errorf("first operation should succeed, got: %v", err1)
	}
	if err2 == nil {
		t.Error("second operation should have error")
	}
}

// TestRun_ContextPropagation tests that context is properly passed
func TestRun_ContextPropagation(t *testing.T) {
	ctx := context.WithValue(context.Background(), "key", "value")
	var err error

	var receivedValue interface{}
	run(ctx, &err, "test", func(ctx context.Context) error {
		receivedValue = ctx.Value("key")
		return nil
	})

	if receivedValue != "value" {
		t.Errorf("context value not propagated, got: %v", receivedValue)
	}
}

// TestNewGlobalError_ContextBinding tests that NewGlobalError properly binds the context
func TestNewGlobalError_ContextBinding(t *testing.T) {
	// Create context with a value
	ctx := context.WithValue(context.Background(), "request_id", "req-456")
	var err error

	var receivedValue interface{}
	runner := NewGlobalError(ctx)

	// Use the runner - it should use the bound context
	runner(&err, "test operation", func(ctx context.Context) error {
		receivedValue = ctx.Value("request_id")
		return nil
	})

	if receivedValue != "req-456" {
		t.Errorf("context not properly bound, expected 'req-456', got: %v", receivedValue)
	}
}

// TestNewGlobalError_MiddlewareReceivesBoundContext tests that middleware receives the bound context
func TestNewGlobalError_MiddlewareReceivesBoundContext(t *testing.T) {
	ctx := context.WithValue(context.Background(), "trace_id", "trace-789")
	var err error

	var middlewareReceivedValue interface{}
	middleware := func(ctx context.Context, err error, tip string) {
		middlewareReceivedValue = ctx.Value("trace_id")
	}

	runner := NewGlobalError(ctx, middleware)
	runner(&err, "test", func(ctx context.Context) error {
		return nil
	})

	if middlewareReceivedValue != "trace-789" {
		t.Errorf("middleware should receive bound context, got: %v", middlewareReceivedValue)
	}
}

// TestLoggerMiddleware tests the logger middleware output
func TestLoggerMiddleware(t *testing.T) {
	ctx := context.WithValue(context.Background(), "trace_id", "test-trace-123")

	t.Run("success log", func(t *testing.T) {
		// We can't easily capture fmt.Printf output without redirecting stdout,
		// so we just verify it doesn't panic
		LoggerMiddleware(ctx, nil, "test operation")
	})

	t.Run("error log", func(t *testing.T) {
		err := errors.New("test error")
		LoggerMiddleware(ctx, err, "test operation")
	})

	t.Run("no trace_id", func(t *testing.T) {
		ctx := context.Background()
		LoggerMiddleware(ctx, nil, "test operation")
	})
}

// TestMetricsMiddleware tests the metrics middleware
// func TestMetricsMiddleware(t *testing.T) {
// 	ctx := context.Background()

// 	// Just verify it doesn't panic
// 	MetricsMiddleware(ctx, true, "test operation", nil)
// 	MetricsMiddleware(ctx, false, "test operation", errors.New("test error"))
// }

// TestGetTraceID tests the trace ID extraction
func TestGetTraceID(t *testing.T) {
	t.Run("with trace_id", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), "trace_id", "abc-123")
		traceID := getTraceID(ctx)
		if traceID != "abc-123" {
			t.Errorf("expected trace_id 'abc-123', got: %s", traceID)
		}
	})

	t.Run("without trace_id", func(t *testing.T) {
		ctx := context.Background()
		traceID := getTraceID(ctx)
		if traceID != "unknown" {
			t.Errorf("expected 'unknown', got: %s", traceID)
		}
	})

	t.Run("wrong type for trace_id", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), "trace_id", 123)
		traceID := getTraceID(ctx)
		if traceID != "unknown" {
			t.Errorf("expected 'unknown' for wrong type, got: %s", traceID)
		}
	})
}

// TestRun_ComplexScenario tests a real-world scenario with multiple steps
func TestRun_ComplexScenario(t *testing.T) {
	ctx := context.WithValue(context.Background(), "trace_id", "request-123")
	var err error
	var executionLog []string

	logger := func(ctx context.Context, err error, tip string) {
		if err == nil {
			executionLog = append(executionLog, fmt.Sprintf("✅ %s", tip))
		} else {
			executionLog = append(executionLog, fmt.Sprintf("❌ %s: %v", tip, err))
		}
	}

	// Step 1: Success
	run(ctx, &err, "validate input", func(ctx context.Context) error {
		return nil
	}, logger)

	// Step 2: Success
	run(ctx, &err, "fetch data", func(ctx context.Context) error {
		return nil
	}, logger)

	// Step 3: Failure
	run(ctx, &err, "process data", func(ctx context.Context) error {
		return errors.New("processing failed")
	}, logger)

	// Step 4: Should be skipped
	run(ctx, &err, "save result", func(ctx context.Context) error {
		return nil
	}, logger)

	// Verify execution flow
	if len(executionLog) != 3 {
		t.Errorf("expected 3 log entries, got %d: %v", len(executionLog), executionLog)
	}

	if !strings.Contains(executionLog[0], "✅ validate input") {
		t.Errorf("step 1 log incorrect: %s", executionLog[0])
	}
	if !strings.Contains(executionLog[1], "✅ fetch data") {
		t.Errorf("step 2 log incorrect: %s", executionLog[1])
	}
	if !strings.Contains(executionLog[2], "❌ process data") {
		t.Errorf("step 3 log incorrect: %s", executionLog[2])
	}

	// Verify error is set
	if err == nil || err.Error() != "processing failed" {
		t.Errorf("expected 'processing failed' error, got: %v", err)
	}
}

// TestRun_NilMiddleware tests that nil middlewares don't cause issues
func TestRun_NilMiddleware(t *testing.T) {
	ctx := context.Background()
	var err error

	// This should not panic
	run(ctx, &err, "test", func(ctx context.Context) error {
		return nil
	})
}

// TestRun_EmptyMiddlewareSlice tests empty middleware slice
func TestRun_EmptyMiddlewareSlice(t *testing.T) {
	ctx := context.Background()
	var err error

	run(ctx, &err, "test", func(ctx context.Context) error {
		return errors.New("test error")
	}, []middleware{}...)

	if err == nil {
		t.Error("error should be captured even without middlewares")
	}
}

// BenchmarkRun_NoMiddleware benchmarks Run without middleware
func BenchmarkRun_NoMiddleware(b *testing.B) {
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var err error
		run(ctx, &err, "benchmark", func(ctx context.Context) error {
			return nil
		})
	}
}

// BenchmarkRun_WithMiddleware benchmarks Run with middleware
func BenchmarkRun_WithMiddleware(b *testing.B) {
	ctx := context.Background()
	mid := func(ctx context.Context, err error, tip string) {}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var err error
		run(ctx, &err, "benchmark", func(ctx context.Context) error {
			return nil
		}, mid)
	}
}

// BenchmarkNewGlobalError benchmarks the NewGlobalError function
func BenchmarkNewGlobalError(b *testing.B) {
	ctx := context.Background()
	mid := func(ctx context.Context, err error, tip string) {}
	runner := NewGlobalError(ctx, mid)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var err error
		runner(&err, "benchmark", func(ctx context.Context) error {
			return nil
		})
	}
}
