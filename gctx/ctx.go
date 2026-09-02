// Package gctx 提供自定义请求上下文，专注于标准库没有的功能
// 包括请求跟踪、span管理、超时控制、类型安全值存取
package gctx

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/memory198/go-gear/logger"
)

// Context 自定义请求上下文
// 提供标准库没有的功能：请求跟踪、span管理、超时控制、map存储值
type Context struct {
	context.Context // 嵌入标准上下文（仅用于超时/取消，不用于 WithValue）

	parent *Context // 父上下文，用于 Value 回溯；nil 表示根

	req *http.Request
	rw  http.ResponseWriter

	traceID       string   // 请求追踪 ID
	spanID        string   // 当前 span ID
	parentSpanID  string   // 父 span ID
	middleSpanIDs []string // 中间 span ID 链（不含 current）

	values map[string]any // 自定义值存储，Set 时检查本层重复
	cancel context.CancelFunc
}

// NewContext 创建新的请求上下文（根节点）
// 优先从 W3C traceparent 头提取 traceID 和 parentSpanID，X-Trace-ID/X-Span-ID 为兜底
func NewContext(r *http.Request, w http.ResponseWriter) *Context {
	traceID, parentSpanID := extractTraceFromHeader(r)
	if traceID == "" {
		traceID = generateTraceID()
	}

	ctx, cancel := context.WithCancel(r.Context())

	return &Context{
		Context:      ctx,
		req:          r,
		rw:           w,
		traceID:      traceID,
		spanID:       generateSpanID(),
		parentSpanID: parentSpanID,
		values:       make(map[string]any),
		cancel:       cancel,
	}
}

// extractTraceFromHeader 从请求头提取链路信息
// 优先 W3C traceparent: version-trace_id-parent_id-flags
// 兜底 X-Trace-ID / X-Span-ID
func extractTraceFromHeader(r *http.Request) (traceID, parentSpanID string) {
	if tp := r.Header.Get("traceparent"); tp != "" {
		parts := strings.Split(tp, "-")
		if len(parts) == 4 && len(parts[1]) == 32 && len(parts[2]) == 16 {
			return parts[1], parts[2]
		}
	}
	traceID = r.Header.Get("X-Trace-ID")
	parentSpanID = r.Header.Get("X-Span-ID")
	return
}

// Request 获取原始 HTTP 请求
func (c *Context) Request() *http.Request { return c.req }

// Cancel 取消上下文及其所有子上下文
func (c *Context) Cancel() {
	if c.cancel != nil {
		c.cancel()
	}
}

// ResponseWriter 获取 HTTP 响应写入器
func (c *Context) ResponseWriter() http.ResponseWriter { return c.rw }

// TraceID 获取请求追踪 ID
func (c *Context) TraceID() string { return c.traceID }

// SpanID 获取当前 span ID
func (c *Context) SpanID() string { return c.spanID }

// ParentSpanID 获取父 span ID
func (c *Context) ParentSpanID() string { return c.parentSpanID }

// MiddleSpanIDs 获取中间 span ID 链
func (c *Context) MiddleSpanIDs() []string {
	// 从父链聚合所有中间 span
	var ids []string
	if c.parent != nil {
		ids = append(ids, c.parent.MiddleSpanIDs()...)
		if c.parent.spanID != "" {
			ids = append(ids, c.parent.spanID)
		}
	}
	return ids
}

// StartSpan 创建新的 span 上下文
// 将当前 spanID 推入中间链，新 spanID 作为当前
func (c *Context) StartSpan() *Context {
	newSpanID := generateSpanID()

	return &Context{
		Context:      c.Context,
		parent:       c,
		req:          c.req,
		rw:           c.rw,
		traceID:      c.traceID,
		spanID:       newSpanID,
		parentSpanID: c.spanID,
		values:       make(map[string]any), // 新层，不继承父层 values
	}
}

// WithTimeout 创建带超时的子上下文
func (c *Context) WithTimeout(timeout time.Duration) (*Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(c.Context, timeout)
	return &Context{
		Context:      ctx,
		parent:       c,
		req:          c.req,
		rw:           c.rw,
		traceID:      c.traceID,
		spanID:       c.spanID,
		parentSpanID: c.parentSpanID,
		values:       make(map[string]any),
	}, cancel
}

// WithTimeoutFunc 创建带超时的子上下文，超时时调用回调
func (c *Context) WithTimeoutFunc(timeout time.Duration, callback func()) (*Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(c.Context, timeout)
	newCtx := &Context{
		Context:      ctx,
		parent:       c,
		req:          c.req,
		rw:           c.rw,
		traceID:      c.traceID,
		spanID:       c.spanID,
		parentSpanID: c.parentSpanID,
		values:       make(map[string]any),
	}

	go func() {
		<-ctx.Done()
		if ctx.Err() == context.DeadlineExceeded && callback != nil {
			callback()
		}
	}()

	return newCtx, cancel
}

// WithDeadline 创建带截止时间的子上下文
func (c *Context) WithDeadline(deadline time.Time) (*Context, context.CancelFunc) {
	ctx, cancel := context.WithDeadline(c.Context, deadline)
	return &Context{
		Context:      ctx,
		parent:       c,
		req:          c.req,
		rw:           c.rw,
		traceID:      c.traceID,
		spanID:       c.spanID,
		parentSpanID: c.parentSpanID,
		values:       make(map[string]any),
	}, cancel
}

// Set 在当前上下文存储值，key 已存在时 panic
// 派生上下文（StartSpan/WithTimeout 等）不会继承父层 values，可自由覆盖
func (c *Context) Set(key string, value any) {
	if _, exists := c.values[key]; exists {
		panic(fmt.Sprintf("gctx: duplicate key %q", key))
	}
	c.values[key] = value
}

// Value 获取值
// 优先检查 logger 的链路追踪 key，再查本层 values，最后沿 parent 链向上回溯
func (c *Context) Value(key any) any {
	switch key {
	case logger.RootTraceIDKey:
		return c.traceID
	case logger.CurrentSpanIDKey:
		return c.spanID
	case logger.MiddleSpanIDsKey:
		return c.MiddleSpanIDs()
	}

	if strKey, ok := key.(string); ok {
		if val, exists := c.values[strKey]; exists {
			return val
		}
		if c.parent != nil {
			return c.parent.Value(key)
		}
	}
	return c.Context.Value(key)
}

// SetTraceIDHeader 设置追踪 ID 响应头
func (c *Context) SetTraceIDHeader() {
	c.rw.Header().Set("X-Trace-ID", c.traceID)
}

// SetSpanIDHeader 设置 span ID 响应头
func (c *Context) SetSpanIDHeader() {
	c.rw.Header().Set("X-Span-ID", c.spanID)
}

// SetParentSpanIDHeader 设置父 span ID 响应头
func (c *Context) SetParentSpanIDHeader() {
	if c.parentSpanID != "" {
		c.rw.Header().Set("X-Parent-Span-ID", c.parentSpanID)
	}
}

// SetTraceHeaders 设置所有追踪相关响应头
func (c *Context) SetTraceHeaders() {
	c.SetTraceIDHeader()
	c.SetSpanIDHeader()
	c.SetParentSpanIDHeader()
}

// generateTraceID 生成 OTel 兼容的 32 位 hex TraceID（128-bit）
func generateTraceID() string {
	return hex.EncodeToString(randBytes(16))
}

// generateSpanID 生成 OTel 兼容的 16 位 hex SpanID（64-bit）
func generateSpanID() string {
	return hex.EncodeToString(randBytes(8))
}

// randBytes 生成 n 字节的加密级随机数据
func randBytes(n int) []byte {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("gctx: crypto/rand failed: %v", err))
	}
	return b
}
