package gctx

import (
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/memory198/go-gear/logger"
)

func TestNewContext(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	ctx := NewContext(req, w)

	if ctx.Request() != req {
		t.Error("Request() should return original request")
	}
	if ctx.ResponseWriter() != w {
		t.Error("ResponseWriter() should return original response writer")
	}
}

func TestContextTraceID(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	ctx := NewContext(req, w)

	if ctx.TraceID() == "" {
		t.Error("TraceID() should not be empty")
	}

	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.Header.Set("X-Trace-ID", "test-trace-id")
	w2 := httptest.NewRecorder()
	ctx2 := NewContext(req2, w2)

	if ctx2.TraceID() != "test-trace-id" {
		t.Errorf("TraceID() = %s, want 'test-trace-id'", ctx2.TraceID())
	}
}

func TestContextSpanID(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	ctx := NewContext(req, w)

	if ctx.SpanID() == "" {
		t.Error("Initial context should have span ID (root span)")
	}
	if ctx.ParentSpanID() != "" {
		t.Error("Root span should not have parent span ID")
	}
	if ctx.TraceID() == "" {
		t.Error("TraceID() should not be empty")
	}
}

func TestContextStartSpan(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	ctx := NewContext(req, w)

	originalSpanID := ctx.SpanID()

	childCtx := ctx.StartSpan()

	if childCtx.SpanID() == originalSpanID {
		t.Error("Child span should have different span ID")
	}
	if childCtx.ParentSpanID() != originalSpanID {
		t.Errorf("Child parent span ID = %s, want %s", childCtx.ParentSpanID(), originalSpanID)
	}
	if childCtx.TraceID() != ctx.TraceID() {
		t.Error("Child span should inherit trace ID")
	}
	if childCtx.Request() != req {
		t.Error("Child span should inherit request")
	}
	if childCtx.ResponseWriter() != w {
		t.Error("Child span should inherit response writer")
	}

	// 中间链：父 span 应进入 middleSpanIDs
	if len(childCtx.MiddleSpanIDs()) != 1 || childCtx.MiddleSpanIDs()[0] != originalSpanID {
		t.Errorf("MiddleSpanIDs = %v, want [%s]", childCtx.MiddleSpanIDs(), originalSpanID)
	}
}

func TestContextStartSpanLoggerKeys(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	ctx := NewContext(req, w)

	// 根上下文应可被 logger 读取 trace/span
	if ctx.Value(logger.RootTraceIDKey) != ctx.TraceID() {
		t.Error("RootTraceIDKey should return traceID")
	}
	if ctx.Value(logger.CurrentSpanIDKey) != ctx.SpanID() {
		t.Error("CurrentSpanIDKey should return spanID")
	}

	child := ctx.StartSpan()

	// 子 span 继承 root trace
	if child.Value(logger.RootTraceIDKey) != ctx.TraceID() {
		t.Error("Child should inherit root trace ID")
	}
	// 子 span 的 current 是新 ID
	if child.Value(logger.CurrentSpanIDKey) != child.SpanID() {
		t.Error("Child CurrentSpanIDKey should match new spanID")
	}
	// 中间链包含原 span
	middle := child.Value(logger.MiddleSpanIDsKey).([]string)
	if len(middle) != 1 || middle[0] != ctx.SpanID() {
		t.Errorf("MiddleSpanIDsKey = %v, want [%s]", middle, ctx.SpanID())
	}
}

func TestContextWithTimeout(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	ctx := NewContext(req, w)

	timeoutCtx, cancel := ctx.WithTimeout(100 * time.Millisecond)
	defer cancel()

	if timeoutCtx.Request() != req {
		t.Error("Timeout context should inherit request")
	}
	if timeoutCtx.TraceID() != ctx.TraceID() {
		t.Error("Timeout context should inherit trace ID")
	}
	if timeoutCtx.SpanID() != ctx.SpanID() {
		t.Error("Timeout context should inherit span ID")
	}

	select {
	case <-timeoutCtx.Done():
	case <-time.After(200 * time.Millisecond):
		t.Error("Context should have timed out")
	}
}

func TestContextWithTimeoutFunc(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	ctx := NewContext(req, w)

	var wg sync.WaitGroup
	wg.Add(1)
	callbackCalled := false

	timeoutCtx, cancel := ctx.WithTimeoutFunc(50*time.Millisecond, func() {
		callbackCalled = true
		wg.Done()
	})
	defer cancel()

	if timeoutCtx.Request() != req {
		t.Error("Timeout context should inherit request")
	}
	if timeoutCtx.TraceID() != ctx.TraceID() {
		t.Error("Timeout context should inherit trace ID")
	}

	select {
	case <-timeoutCtx.Done():
	case <-time.After(200 * time.Millisecond):
		t.Error("Context should have timed out")
	}

	wg.Wait()
	if !callbackCalled {
		t.Error("Callback should have been called on timeout")
	}
}

func TestContextSet(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	ctx := NewContext(req, w)

	ctx.Set("key1", "value1")
	if ctx.Value("key1") != "value1" {
		t.Errorf("Value('key1') = %v, want 'value1'", ctx.Value("key1"))
	}

	// 重复 key 应 panic
	defer func() {
		if r := recover(); r == nil {
			t.Error("Set duplicate key should panic")
		}
	}()
	ctx.Set("key1", "value2")
}

func TestContextSetAcrossDerivedContext(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	ctx := NewContext(req, w)

	ctx.Set("key", "parent-value")

	// 派生上下文可以覆盖同 key 而不 panic
	child := ctx.StartSpan()
	child.Set("key", "child-value") // 不应 panic
	if child.Value("key") != "child-value" {
		t.Errorf("child value = %v, want 'child-value'", child.Value("key"))
	}

	// 父上下文值不受影响
	if ctx.Value("key") != "parent-value" {
		t.Errorf("parent value = %v, want 'parent-value'", ctx.Value("key"))
	}
}

func TestContextDeadline(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	ctx := NewContext(req, w)

	deadline := time.Now().Add(100 * time.Millisecond)
	deadlineCtx, cancel := ctx.WithDeadline(deadline)
	defer cancel()

	if deadlineCtx.Request() != req {
		t.Error("Deadline context should inherit request")
	}
	if deadlineCtx.TraceID() != ctx.TraceID() {
		t.Error("Deadline context should inherit trace ID")
	}

	select {
	case <-deadlineCtx.Done():
	case <-time.After(200 * time.Millisecond):
		t.Error("Context should have reached deadline")
	}
}

func TestContextSetTraceHeaders(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	ctx := NewContext(req, w)

	ctx.SetTraceHeaders()

	if w.Header().Get("X-Trace-ID") != ctx.TraceID() {
		t.Errorf("X-Trace-ID header = %s, want %s", w.Header().Get("X-Trace-ID"), ctx.TraceID())
	}
	if w.Header().Get("X-Span-ID") != ctx.SpanID() {
		t.Errorf("X-Span-ID header = %s, want %s", w.Header().Get("X-Span-ID"), ctx.SpanID())
	}
	if w.Header().Get("X-Parent-Span-ID") != "" {
		t.Error("X-Parent-Span-ID header should not be set for root span")
	}
}

func TestContextStartSpanHeaders(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	ctx := NewContext(req, w)

	childCtx := ctx.StartSpan()
	childCtx.SetTraceHeaders()

	if w.Header().Get("X-Trace-ID") != ctx.TraceID() {
		t.Errorf("X-Trace-ID header = %s, want %s", w.Header().Get("X-Trace-ID"), ctx.TraceID())
	}
	if w.Header().Get("X-Span-ID") != childCtx.SpanID() {
		t.Errorf("X-Span-ID header = %s, want %s", w.Header().Get("X-Span-ID"), childCtx.SpanID())
	}
	if w.Header().Get("X-Parent-Span-ID") != ctx.SpanID() {
		t.Errorf("X-Parent-Span-ID header = %s, want %s", w.Header().Get("X-Parent-Span-ID"), ctx.SpanID())
	}
}

func TestContextStartSpanNested(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	root := NewContext(req, w)

	span1 := root.StartSpan()
	span2 := span1.StartSpan()

	// span2 的中间链应包含 root.spanID 和 span1.spanID
	middle := span2.MiddleSpanIDs()
	if len(middle) != 2 {
		t.Fatalf("MiddleSpanIDs length = %d, want 2", len(middle))
	}
	if middle[0] != root.SpanID() {
		t.Errorf("middle[0] = %s, want root span %s", middle[0], root.SpanID())
	}
	if middle[1] != span1.SpanID() {
		t.Errorf("middle[1] = %s, want span1 %s", middle[1], span1.SpanID())
	}
}
