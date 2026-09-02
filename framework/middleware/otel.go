package middleware

import (
	"net/http"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/memory198/go-gear/logger"
)

// OTel 返回 OpenTelemetry 追踪中间件
// 自动从请求头提取 W3C traceparent，创建 Server Span，桥接 logger 链路字段
// serviceName 为服务名，显示在 Jaeger/Collector 中
func OTel(serviceName string) func(http.Handler) http.Handler {
	tracer := otel.Tracer(serviceName)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 1. 从 Header 提取上游 trace context
			ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))

			// 2. 创建 Server Span
			spanName := r.Method + " " + r.URL.Path
			ctx, span := tracer.Start(ctx, spanName,
				trace.WithSpanKind(trace.SpanKindServer),
				trace.WithAttributes(
					attribute.String("http.method", r.Method),
					attribute.String("http.target", r.URL.Path),
				),
			)
			defer span.End()

			// 3. 桥接 logger：写入 RootTraceID / CurrentSpanID
			sc := span.SpanContext()
			if sc.HasTraceID() {
				ctx = logger.WithRootTraceID(ctx, sc.TraceID().String())
			}
			if sc.HasSpanID() {
				ctx = logger.WithCurrentSpanID(ctx, sc.SpanID().String())
			}

			// 4. 执行后续处理器
			sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			r = r.WithContext(ctx)
			next.ServeHTTP(sw, r)

			// 5. 记录响应属性
			span.SetAttributes(attribute.Int("http.status_code", sw.status))
			if sw.status >= 400 {
				span.SetStatus(codes.Error, http.StatusText(sw.status))
				span.SetAttributes(attribute.String("error", "true"))
			}
		})
	}
}

// statusWriter 包装 ResponseWriter 以捕获 HTTP 状态码
type statusWriter struct {
	http.ResponseWriter
	status int
}

// WriteHeader 记录状态码
func (sw *statusWriter) WriteHeader(status int) {
	sw.status = status
	sw.ResponseWriter.WriteHeader(status)
}
