// Package gctx 提供自定义请求上下文，专注于标准库没有的功能
// 包括请求跟踪、span管理、超时控制、类型安全值存取
package gctx

import (
	"context"
	"net/http"
	"time"
)

// Context 自定义请求上下文
// 提供标准库没有的功能：请求跟踪、span管理、超时控制、map存储值
type Context struct {
	context.Context                     // 嵌入标准上下文接口
	req             *http.Request       // 原始 HTTP 请求
	rw              http.ResponseWriter // HTTP 响应写入器
	traceID         string              // 请求追踪ID
	spanID          string              // 当前span ID
	parentSpanID    string              // 父span ID
	values          map[string]any      // 自定义值存储
	cancel          context.CancelFunc  // 取消函数
}

// NewContext 创建新的请求上下文
// 基于请求的上下文创建自定义上下文，自动生成trace ID
// 初始上下文不包含span ID，需要通过StartSpan()创建span
// 创建的上下文是可取消的，可以通过Cancel()方法取消
func NewContext(r *http.Request, w http.ResponseWriter) *Context {
	// 从请求头获取trace ID，如果没有则生成
	traceID := r.Header.Get("X-Trace-ID")
	if traceID == "" {
		traceID = generateID()
	}

	// 创建可取消的上下文
	ctx, cancel := context.WithCancel(r.Context())

	return &Context{
		Context: ctx,
		req:     r,
		rw:      w,
		traceID: traceID,
		values:  make(map[string]any),
		cancel:  cancel,
	}
}

// Request 获取原始 HTTP 请求
func (c *Context) Request() *http.Request {
	return c.req
}

// Cancel 取消上下文
// 调用此方法会取消当前上下文及其所有子上下文
func (c *Context) Cancel() {
	if c.cancel != nil {
		c.cancel()
	}
}

// ResponseWriter 获取 HTTP 响应写入器
func (c *Context) ResponseWriter() http.ResponseWriter {
	return c.rw
}

// TraceID 获取请求追踪ID
func (c *Context) TraceID() string {
	return c.traceID
}

// SpanID 获取当前span ID
func (c *Context) SpanID() string {
	return c.spanID
}

// ParentSpanID 获取父span ID
func (c *Context) ParentSpanID() string {
	return c.parentSpanID
}

// StartSpan 创建新的span上下文
// 生成新的span ID，将当前span ID设置为父span ID
func (c *Context) StartSpan() *Context {
	newSpanID := generateID()
	return &Context{
		Context:      c.Context,
		req:          c.req,
		rw:           c.rw,
		traceID:      c.traceID,
		spanID:       newSpanID,
		parentSpanID: c.spanID,
		values:       c.copyValues(),
	}
}

// WithTimeout 创建带超时的子上下文
func (c *Context) WithTimeout(timeout time.Duration) (*Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(c.Context, timeout)
	newCtx := &Context{
		Context:      ctx,
		req:          c.req,
		rw:           c.rw,
		traceID:      c.traceID,
		spanID:       c.spanID,
		parentSpanID: c.parentSpanID,
		values:       c.copyValues(),
	}
	return newCtx, cancel
}

// WithTimeoutFunc 创建带超时的子上下文，并在超时时调用回调函数
// 回调函数在新的goroutine中执行，用于清理操作
func (c *Context) WithTimeoutFunc(timeout time.Duration, callback func()) (*Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(c.Context, timeout)
	newCtx := &Context{
		Context:      ctx,
		req:          c.req,
		rw:           c.rw,
		traceID:      c.traceID,
		spanID:       c.spanID,
		parentSpanID: c.parentSpanID,
		values:       c.copyValues(),
	}

	// 启动goroutine监听超时
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
	newCtx := &Context{
		Context:      ctx,
		req:          c.req,
		rw:           c.rw,
		traceID:      c.traceID,
		spanID:       c.spanID,
		parentSpanID: c.parentSpanID,
		values:       c.copyValues(),
	}
	return newCtx, cancel
}

// WithValue 创建新的上下文，并添加键值对
// 基于当前上下文创建新的上下文，复制现有值并添加新值
func (c *Context) WithValue(key, value any) *Context {
	newValues := c.copyValues()
	if strKey, ok := key.(string); ok {
		newValues[strKey] = value
	}
	return &Context{
		Context:      c.Context,
		req:          c.req,
		rw:           c.rw,
		traceID:      c.traceID,
		spanID:       c.spanID,
		parentSpanID: c.parentSpanID,
		values:       newValues,
	}
}

// Value 获取值
// 首先从自定义值存储中获取，然后从标准上下文中获取
func (c *Context) Value(key any) any {
	// 首先从自定义值存储中获取
	if strKey, ok := key.(string); ok {
		if val, exists := c.values[strKey]; exists {
			return val
		}
	}
	// 然后从标准上下文中获取
	return c.Context.Value(key)
}

// SetTraceIDHeader 设置追踪ID响应头
func (c *Context) SetTraceIDHeader() {
	c.rw.Header().Set("X-Trace-ID", c.traceID)
}

// SetSpanIDHeader 设置span ID响应头
func (c *Context) SetSpanIDHeader() {
	c.rw.Header().Set("X-Span-ID", c.spanID)
}

// SetParentSpanIDHeader 设置父span ID响应头
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

// copyValues 复制值映射
func (c *Context) copyValues() map[string]any {
	newValues := make(map[string]any, len(c.values))
	for k, v := range c.values {
		newValues[k] = v
	}
	return newValues
}

// generateID 生成简单的随机ID
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
