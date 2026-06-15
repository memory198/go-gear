// Package middleware 提供 HTTP 中间件实现
package middleware

import (
	"log"
	"net/http"
	"time"
)

// Logger 请求日志中间件
// 记录每个 HTTP 请求的方法、路径、状态码和处理耗时
// 输出格式：[METHOD] [PATH] [STATUS] [DURATION]
// 用法：http.Handle("/", middleware.Logger(nextHandler))
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := &wrappedWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(ww, r)
		log.Printf("%s %s %d %v", r.Method, r.URL.Path, ww.status, time.Since(start))
	})
}

// wrappedWriter 包装 ResponseWriter 以捕获状态码
type wrappedWriter struct {
	http.ResponseWriter
	status int
}

// WriteHeader 重写 WriteHeader 方法以记录状态码
func (ww *wrappedWriter) WriteHeader(status int) {
	ww.status = status
	ww.ResponseWriter.WriteHeader(status)
}
