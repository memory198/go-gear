package framework

import (
	"net/http/httptest"
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

	// 测试超时
	select {
	case <-timeoutCtx.Done():
		// 正常，应该超时
	case <-time.After(200 * time.Millisecond):
		t.Error("Context should have timed out")
	}
}

func TestContextSet(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	ctx := NewContext(req, w)

	// Set 在当前上下文存储值
	ctx.Set("key", "value")
	if ctx.Value("key") != "value" {
		t.Errorf("Value('key') = %v, want 'value'", ctx.Value("key"))
	}

	// 重复 key 应 panic
	defer func() {
		if r := recover(); r == nil {
			t.Error("Set duplicate key should panic")
		}
	}()
	ctx.Set("key", "value2")
}

func TestContextSetAcrossDerived(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	ctx := NewContext(req, w)

	ctx.Set("key", "parent")

	// 派生上下文可覆盖同 key
	child := ctx.StartSpan()
	child.Set("key", "child")
	if child.Value("key") != "child" {
		t.Errorf("child value = %v, want 'child'", child.Value("key"))
	}

	// 父上下文不受影响
	if ctx.Value("key") != "parent" {
		t.Errorf("parent value = %v, want 'parent'", ctx.Value("key"))
	}

	if child.TraceID() != ctx.TraceID() {
		t.Error("Child should inherit trace ID")
	}
	if child.Request() != req {
		t.Error("Child should inherit request")
	}
}

func TestContextSetTraceIDHeader(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	ctx := NewContext(req, w)

	ctx.SetTraceIDHeader()
	if w.Header().Get("X-Trace-ID") != ctx.TraceID() {
		t.Errorf("SetTraceIDHeader failed, got %s, want %s", w.Header().Get("X-Trace-ID"), ctx.TraceID())
	}
}

func TestContextSetChain(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	ctx := NewContext(req, w)

	child := ctx.StartSpan()
	child.Set("key1", "value1")
	child.Set("key2", "value2")

	if child.Value("key1") != "value1" {
		t.Errorf("Value('key1') = %v, want 'value1'", child.Value("key1"))
	}
	if child.Value("key2") != "value2" {
		t.Errorf("Value('key2') = %v, want 'value2'", child.Value("key2"))
	}

	// 父上下文不受影响
	if ctx.Value("key1") != nil {
		t.Error("Original context should not have key1")
	}
	if ctx.Value("key2") != nil {
		t.Error("Original context should not have key2")
	}
}

func TestContextWithDeadline(t *testing.T) {
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

	// 测试截止时间
	select {
	case <-deadlineCtx.Done():
		// 正常，应该超时
	case <-time.After(200 * time.Millisecond):
		t.Error("Context should have reached deadline")
	}
}
