package logger

import "context"

// ---- context key 类型（避免与其他包冲突） ----

type rootTraceIDKey struct{}
type middleSpanIDsKey struct{}
type currentSpanIDKey struct{}

// ---- 链路字段 context key ----
// 值由调用方自行生成、派生并写入 context，logger 只负责读取。

// RootTraceIDKey context 中存储根链路追踪 ID 的 key
var RootTraceIDKey = rootTraceIDKey{}

// MiddleSpanIDsKey context 中存储中间 span ID 列表的 key
var MiddleSpanIDsKey = middleSpanIDsKey{}

// CurrentSpanIDKey context 中存储当前 span ID 的 key
var CurrentSpanIDKey = currentSpanIDKey{}

// traceInfo 从 context 中取出的链路信息三元组
type traceInfo struct {
	RootID    string
	MiddleIDs []string
	CurrentID string
}

// traceFromCtx 从 context 中提取链路追踪字段
// 字段不存在或类型错误时对应字段置空，不阻断日志输出
func traceFromCtx(ctx context.Context) traceInfo {
	if ctx == nil {
		return traceInfo{}
	}
	return traceInfo{
		RootID:    stringFromCtx(ctx, RootTraceIDKey),
		MiddleIDs: stringSliceFromCtx(ctx, MiddleSpanIDsKey),
		CurrentID: stringFromCtx(ctx, CurrentSpanIDKey),
	}
}

func stringFromCtx(ctx context.Context, key any) string {
	v, _ := ctx.Value(key).(string)
	return v
}

func stringSliceFromCtx(ctx context.Context, key any) []string {
	v, _ := ctx.Value(key).([]string)
	return v
}
