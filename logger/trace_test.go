package logger

import (
	"context"
	"testing"
)

func TestTraceFromCtx(t *testing.T) {
	t.Run("nil context", func(t *testing.T) {
		ti := traceFromCtx(nil)
		if ti.RootTraceID != "" || ti.CurrentSpanID != "" || len(ti.MiddleSpanIDs) != 0 {
			t.Errorf("traceFromCtx(nil) should return empty traceInfo, got %+v", ti)
		}
	})

	t.Run("missing keys", func(t *testing.T) {
		ti := traceFromCtx(context.Background())
		if ti.RootTraceID != "" || ti.CurrentSpanID != "" || len(ti.MiddleSpanIDs) != 0 {
			t.Errorf("traceFromCtx() should return empty traceInfo, got %+v", ti)
		}
	})

	t.Run("wrong type value is ignored", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), RootTraceIDKey, 123)
		ti := traceFromCtx(ctx)
		if ti.RootTraceID != "" {
			t.Errorf("RootTraceID should be empty for non-string value, got %q", ti.RootTraceID)
		}
	})

	t.Run("root_trace_id only", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), RootTraceIDKey, "root-abc")
		ti := traceFromCtx(ctx)
		if ti.RootTraceID != "root-abc" {
			t.Errorf("RootTraceID = %q, want %q", ti.RootTraceID, "root-abc")
		}
	})

	t.Run("full trace info", func(t *testing.T) {
		ctx := context.Background()
		ctx = context.WithValue(ctx, RootTraceIDKey, "r1")
		ctx = context.WithValue(ctx, MiddleSpanIDsKey, []string{"m1", "m2"})
		ctx = context.WithValue(ctx, CurrentSpanIDKey, "c3")
		ti := traceFromCtx(ctx)
		if ti.RootTraceID != "r1" {
			t.Errorf("RootTraceID = %q, want r1", ti.RootTraceID)
		}
		if len(ti.MiddleSpanIDs) != 2 || ti.MiddleSpanIDs[0] != "m1" || ti.MiddleSpanIDs[1] != "m2" {
			t.Errorf("MiddleSpanIDs = %v, want [m1 m2]", ti.MiddleSpanIDs)
		}
		if ti.CurrentSpanID != "c3" {
			t.Errorf("CurrentSpanID = %q, want c3", ti.CurrentSpanID)
		}
	})
}
