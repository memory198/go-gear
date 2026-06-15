// Package middleware 提供 HTTP 中间件实现
package middleware

import (
	"log"
	"net/http"
	"runtime/debug"

	"github.com/memory198/go-gear/framework"
)

// Recoverer panic 恢复中间件
// 捕获后续处理器中的 panic，记录错误日志和堆栈信息，并返回 500 错误响应
// 用法：http.Handle("/", middleware.Recoverer(nextHandler))
func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("panic: %v\n%s", err, debug.Stack())
				framework.WriteError(w, framework.ErrInternal)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
