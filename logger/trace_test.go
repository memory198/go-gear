package logger

import (
	"context"
	"testing"
)

func TestTraceFromCtx(t *testing.T) {
	t.Run("nil context", func(t *testing.T) {
		ti := traceFromCtx(nil)
		if ti.RootID != "" || ti.CurrentID != "" || len(ti.MiddleIDs) != 0 {
			t.Errorf("traceFromCtx(nil) should return empty traceInfo, got %+v", ti)
		}
	})

	t.Run("missing keys", func(t *testing.T) {
		ti := traceFromCtx(context.Background())
		if ti.RootID != "" || ti.CurrentID != "" || len(ti.MiddleIDs) != 0 {
			t.Errorf("traceFromCtx() should return empty traceInfo, got %+v", ti)
		}
	})

	t.Run("wrong type value is ignored", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), RootTraceIDKey, 123)
		ti := traceFromCtx(ctx)
		if ti.RootID != "" {
			t.Errorf("RootID should be empty for non-string value, got %q", ti.RootID)
		}
	})

	t.Run("root_trace_id only", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), RootTraceIDKey, "root-abc")
		ti := traceFromCtx(ctx)
		if ti.RootID != "root-abc" {
			t.Errorf("RootID = %q, want %q", ti.RootID, "root-abc")
		}
	})

	t.Run("full trace info", func(t *testing.T) {
		ctx := context.Background()
		ctx = context.WithValue(ctx, RootTraceIDKey, "r1")
		ctx = context.WithValue(ctx, MiddleSpanIDsKey, []string{"m1", "m2"})
		ctx = context.WithValue(ctx, CurrentSpanIDKey, "c3")
		ti := traceFromCtx(ctx)
		if ti.RootID != "r1" {
			t.Errorf("RootID = %q, want r1", ti.RootID)
		}
		if len(ti.MiddleIDs) != 2 || ti.MiddleIDs[0] != "m1" || ti.MiddleIDs[1] != "m2" {
			t.Errorf("MiddleIDs = %v, want [m1 m2]", ti.MiddleIDs)
		}
		if ti.CurrentID != "c3" {
			t.Errorf("CurrentID = %q, want c3", ti.CurrentID)
		}
	})
}
