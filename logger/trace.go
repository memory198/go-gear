package logger

import "context"

// ctxKey 用于从 context 中取 traceId 的 key 类型（避免与其他包冲突）
type ctxKey struct{}

// TraceIDKey context 中存储 traceId 的 key
var TraceIDKey = ctxKey{}

// WithTraceID 将 traceId 注入 context
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, TraceIDKey, traceID)
}

// traceIDFromCtx 从 context 中取 traceId，不存在返回空串
func traceIDFromCtx(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(TraceIDKey).(string); ok {
		return v
	}
	return ""
}
