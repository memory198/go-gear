package framework

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// 测试请求结构体
type TestRequest struct {
	Name string `json:"name" validate:"required"`
	Age  int    `json:"age" validate:"min=1"`
}

// 测试响应结构体
type TestResponse struct {
	Message string `json:"message"`
}

func TestHandle(t *testing.T) {
	// 定义测试处理器
	handler := func(ctx context.Context, req *TestRequest) (*TestResponse, error) {
		return &TestResponse{Message: "Hello " + req.Name}, nil
	}

	// 包装为HTTP处理器
	httpHandler := Handle(handler)

	// 测试成功情况
	t.Run("success", func(t *testing.T) {
		body := `{"name":"test","age":25}`
		req := httptest.NewRequest("POST", "/test", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		httpHandler(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var resp Response
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		if resp.Code != 0 {
			t.Errorf("Expected code 0, got %d", resp.Code)
		}

		// 检查响应数据
		data, ok := resp.Data.(map[string]interface{})
		if !ok {
			t.Fatal("Response data should be a map")
		}
		if data["message"] != "Hello test" {
			t.Errorf("Expected message 'Hello test', got %v", data["message"])
		}
	})

	// 测试绑定错误
	t.Run("bind error", func(t *testing.T) {
		body := `invalid json`
		req := httptest.NewRequest("POST", "/test", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		httpHandler(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", w.Code)
		}

		var resp Response
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		if resp.Code != ErrBadRequest.Code {
			t.Errorf("Expected code %d, got %d", ErrBadRequest.Code, resp.Code)
		}
	})

	// 测试验证错误
	t.Run("validation error", func(t *testing.T) {
		body := `{"name":"","age":0}`
		req := httptest.NewRequest("POST", "/test", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		httpHandler(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", w.Code)
		}
	})

	// 测试业务错误
	t.Run("business error", func(t *testing.T) {
		handler := func(ctx context.Context, req *TestRequest) (*TestResponse, error) {
			return nil, ErrNotFound.WithMsg("user not found")
		}

		httpHandler := Handle(handler)
		body := `{"name":"test","age":25}`
		req := httptest.NewRequest("POST", "/test", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		httpHandler(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", w.Code)
		}

		var resp Response
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		if resp.Code != ErrNotFound.Code {
			t.Errorf("Expected code %d, got %d", ErrNotFound.Code, resp.Code)
		}
		if resp.Message != "user not found" {
			t.Errorf("Expected message 'user not found', got %s", resp.Message)
		}
	})
}

func TestHandleContext(t *testing.T) {
	// 定义测试处理器
	handler := func(ctx *Context, req *TestRequest) (*TestResponse, error) {
		// 使用上下文方法
		traceID := ctx.TraceID()
		return &TestResponse{Message: "Hello " + req.Name + " with trace " + traceID}, nil
	}

	// 包装为HTTP处理器
	httpHandler := HandleContext(handler)

	// 测试成功情况
	t.Run("success with context", func(t *testing.T) {
		body := `{"name":"test","age":25}`
		req := httptest.NewRequest("POST", "/test", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Trace-ID", "test-trace-id")
		w := httptest.NewRecorder()

		httpHandler(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var resp Response
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		data, ok := resp.Data.(map[string]interface{})
		if !ok {
			t.Fatal("Response data should be a map")
		}
		if data["message"] != "Hello test with trace test-trace-id" {
			t.Errorf("Expected message 'Hello test with trace test-trace-id', got %v", data["message"])
		}
	})

	// 测试上下文功能
	t.Run("context features", func(t *testing.T) {
		handler := func(ctx *Context, req *TestRequest) (*TestResponse, error) {
			// 测试上下文方法
			ctx.SetTraceIDHeader()
			query := ctx.Request().URL.Query().Get("page")
			return &TestResponse{Message: "page=" + query}, nil
		}

		httpHandler := HandleContext(handler)
		body := `{"name":"test","age":25}`
		req := httptest.NewRequest("POST", "/test?page=2", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		httpHandler(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		// 检查响应头
		if w.Header().Get("X-Trace-ID") == "" {
			t.Error("Expected X-Trace-ID header to be set")
		}

		var resp Response
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		data, ok := resp.Data.(map[string]interface{})
		if !ok {
			t.Fatal("Response data should be a map")
		}
		if data["message"] != "page=2" {
			t.Errorf("Expected message 'page=2', got %v", data["message"])
		}
	})
}
