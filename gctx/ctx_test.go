package gctx

import (
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestNewContext(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	ctx := NewContext(req, w)

	// 测试基本属性
	if ctx.Request() != req {
		t.Error("Request() should return original request")
	}
	if ctx.ResponseWriter() != w {
		t.Error("ResponseWriter() should return original response writer")
	}
}

func TestContextTraceID(t *testing.T) {
	// 测试自动生成trace ID
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	ctx := NewContext(req, w)

	if ctx.TraceID() == "" {
		t.Error("TraceID() should not be empty")
	}

	// 测试从请求头获取trace ID
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

	// 初始上下文不应该有span ID，但应该有trace ID
	if ctx.SpanID() != "" {
		t.Error("Initial context should not have span ID")
	}
	if ctx.ParentSpanID() != "" {
		t.Error("Initial context should not have parent span ID")
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

	// 开始新span
	childCtx := ctx.StartSpan()

	// 子span应该有不同的span ID
	if childCtx.SpanID() == originalSpanID {
		t.Error("Child span should have different span ID")
	}

	// 子span的父span ID应该是原span ID
	if childCtx.ParentSpanID() != originalSpanID {
		t.Errorf("Child parent span ID = %s, want %s", childCtx.ParentSpanID(), originalSpanID)
	}

	// 子span应该继承trace ID
	if childCtx.TraceID() != ctx.TraceID() {
		t.Error("Child span should inherit trace ID")
	}

	// 子span应该继承请求和响应写入器
	if childCtx.Request() != req {
		t.Error("Child span should inherit request")
	}
	if childCtx.ResponseWriter() != w {
		t.Error("Child span should inherit response writer")
	}
}

func TestContextWithTimeout(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	ctx := NewContext(req, w)

	// 测试超时上下文
	timeoutCtx, cancel := ctx.WithTimeout(100 * time.Millisecond)
	defer cancel()

	// 检查是否继承了原有属性
	if timeoutCtx.Request() != req {
		t.Error("Timeout context should inherit request")
	}
	if timeoutCtx.TraceID() != ctx.TraceID() {
		t.Error("Timeout context should inherit trace ID")
	}
	if timeoutCtx.SpanID() != ctx.SpanID() {
		t.Error("Timeout context should inherit span ID")
	}

	// 测试超时
	select {
	case <-timeoutCtx.Done():
		// 正常，应该超时
	case <-time.After(200 * time.Millisecond):
		t.Error("Context should have timed out")
	}
}

func TestContextWithTimeoutFunc(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	ctx := NewContext(req, w)

	// 测试超时回调
	var wg sync.WaitGroup
	wg.Add(1)
	callbackCalled := false

	callback := func() {
		callbackCalled = true
		wg.Done()
	}

	// 创建带超时的子上下文
	timeoutCtx, cancel := ctx.WithTimeoutFunc(50*time.Millisecond, callback)
	defer cancel()

	// 检查是否继承了原有属性
	if timeoutCtx.Request() != req {
		t.Error("Timeout context should inherit request")
	}
	if timeoutCtx.TraceID() != ctx.TraceID() {
		t.Error("Timeout context should inherit trace ID")
	}
	if timeoutCtx.SpanID() != ctx.SpanID() {
		t.Error("Timeout context should inherit span ID")
	}

	// 等待超时
	select {
	case <-timeoutCtx.Done():
		// 正常，应该超时
	case <-time.After(200 * time.Millisecond):
		t.Error("Context should have timed out")
	}

	// 等待回调执行
	wg.Wait()
	if !callbackCalled {
		t.Error("Callback should have been called on timeout")
	}
}

func TestContextWithValue(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	ctx := NewContext(req, w)

	// 测试 WithValue
	newCtx := ctx.WithValue("key", "value")
	if newCtx.Value("key") != "value" {
		t.Errorf("Value('key') = %v, want 'value'", newCtx.Value("key"))
	}

	// 确保原始上下文不受影响
	if ctx.Value("key") != nil {
		t.Error("Original context should not have the new value")
	}

	// 测试继承属性
	if newCtx.TraceID() != ctx.TraceID() {
		t.Error("New context should inherit trace ID")
	}
	if newCtx.SpanID() != ctx.SpanID() {
		t.Error("New context should inherit span ID")
	}
	if newCtx.Request() != req {
		t.Error("New context should inherit request")
	}
}

func TestContextWithValueChain(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	ctx := NewContext(req, w)

	// 测试链式调用
	ctx2 := ctx.WithValue("key1", "value1")
	ctx3 := ctx2.WithValue("key2", "value2")

	if ctx3.Value("key1") != "value1" {
		t.Errorf("Value('key1') = %v, want 'value1'", ctx3.Value("key1"))
	}
	if ctx3.Value("key2") != "value2" {
		t.Errorf("Value('key2') = %v, want 'value2'", ctx3.Value("key2"))
	}

	// 确保原始上下文不受影响
	if ctx.Value("key1") != nil {
		t.Error("Original context should not have key1")
	}
	if ctx.Value("key2") != nil {
		t.Error("Original context should not have key2")
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
	// 初始上下文没有父span ID，所以不应该设置该头
	if w.Header().Get("X-Parent-Span-ID") != "" {
		t.Error("X-Parent-Span-ID header should not be set for initial context")
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

func TestContextDeadline(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	ctx := NewContext(req, w)

	// 测试截止时间上下文
	deadline := time.Now().Add(100 * time.Millisecond)
	deadlineCtx, cancel := ctx.WithDeadline(deadline)
	defer cancel()

	// 检查是否继承了原有属性
	if deadlineCtx.Request() != req {
		t.Error("Deadline context should inherit request")
	}
	if deadlineCtx.TraceID() != ctx.TraceID() {
		t.Error("Deadline context should inherit trace ID")
	}
	if deadlineCtx.SpanID() != ctx.SpanID() {
		t.Error("Deadline context should inherit span ID")
	}

	// 测试截止时间
	select {
	case <-deadlineCtx.Done():
		// 正常，应该超时
	case <-time.After(200 * time.Millisecond):
		t.Error("Context should have reached deadline")
	}
}
