// Package gctx 提供自定义请求上下文，专注于标准库没有的功能
// 包括请求跟踪、span管理、超时控制、类型安全值存取
package gctx

import (
	"context"
	"fmt"
	"net/http"
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
func NewContext(r *http.Request, w http.ResponseWriter) *Context {
	traceID := r.Header.Get("X-Trace-ID")
	if traceID == "" {
		traceID = generateID()
	}

	ctx, cancel := context.WithCancel(r.Context())

	return &Context{
		Context: ctx,
		req:     r,
		rw:      w,
		traceID: traceID,
		spanID:  generateID(),
		values:  make(map[string]any),
		cancel:  cancel,
	}
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
	newSpanID := generateID()

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

// generateID 生成简单的随机 ID
func generateID() string {
	return time.Now().Format("20060102150405") + "-" + randomString(8)
}

// randomString 生成随机字符串
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
		time.Sleep(time.Nanosecond)
	}
	return string(b)
}
