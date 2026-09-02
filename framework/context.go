// Package framework 提供轻量级 HTTP 框架功能
package framework

import (
	"net/http"

	"github.com/memory198/go-gear/gctx"
)

// Context 自定义请求上下文，实际实现在gctx包中
// 提供标准库没有的功能：请求跟踪、span管理、超时控制、map存储值
type Context = gctx.Context

// NewContext 创建新的请求上下文，实际实现在gctx包中
func NewContext(r *http.Request, w http.ResponseWriter) *Context {
	return gctx.NewContext(r, w)
}
