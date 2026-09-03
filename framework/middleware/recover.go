// Package middleware 提供 HTTP 中间件实现
package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/memory198/go-gear/framework"
	"github.com/memory198/go-gear/logger"
)

// Recoverer panic 恢复中间件
// 捕获后续处理器中的 panic，记录错误日志和堆栈信息，并返回 500 错误响应
// 日志使用 ERROR 级；若外层中间件（如 OTel/gctx）已注入 trace，日志会携带链路字段
// 用法：http.Handle("/", middleware.Recoverer(nextHandler))
func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				logger.Error(r.Context(), fmt.Sprintf("panic: %v\n%s", err, debug.Stack()))
				framework.WriteError(w, framework.ErrInternal)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
