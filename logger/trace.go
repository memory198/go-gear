package logger

import "context"

// ---- context key 类型（避免与其他包冲突） ----

type rootTraceIDKey struct{}
type middleSpanIDsKey struct{}
type currentSpanIDKey struct{}

// ---- 链路字段 context key ----
// 值由调用方自行生成、派生并写入 context，logger 只负责读取。

// RootTraceIDKey context 中存储根链路追踪 ID（trace id）的 key
//
// trace id 是整条请求链路的唯一标识，一条链路内只允许一个值：
//   - 子上下文（StartSpan/WithTimeout/后台任务派生等）必须继承，禁止生成新值——
//     否则链路被拆成多条独立 trace，日志按 trace 聚合、跨服务串联都会断裂
//   - 需要表达"新的环节 / 新任务 / 新链路"时用父子 span（StartSpan 派生、
//     current_span_id / middle_span_ids 记录层次），而不是更换 trace id
//   - 新 trace id 仅由链路起点生成：gctx.NewContext 在请求无上游追踪
//     （无 traceparent / X-Trace-ID）时自动生成，其余场景一律透传继承
var RootTraceIDKey = rootTraceIDKey{}

// MiddleSpanIDsKey context 中存储中间 span ID 列表的 key
var MiddleSpanIDsKey = middleSpanIDsKey{}

// CurrentSpanIDKey context 中存储当前 span ID 的 key
var CurrentSpanIDKey = currentSpanIDKey{}

// traceInfo 从 context 中取出的链路信息三元组
type traceInfo struct {
	RootTraceID   string
	MiddleSpanIDs []string
	CurrentSpanID string
}

// traceFromCtx 从 context 中提取链路追踪字段
// withMiddleSpanIDs 为 false 时不读取中间 span 链（避免触发 gctx 的聚合开销）
// 字段不存在或类型错误时对应字段置空，不阻断日志输出
func traceFromCtx(ctx context.Context, withMiddleSpanIDs bool) traceInfo {
	if ctx == nil {
		return traceInfo{}
	}
	ti := traceInfo{
		RootTraceID:   stringFromCtx(ctx, RootTraceIDKey),
		CurrentSpanID: stringFromCtx(ctx, CurrentSpanIDKey),
	}
	if withMiddleSpanIDs {
		ti.MiddleSpanIDs = stringSliceFromCtx(ctx, MiddleSpanIDsKey)
	}
	return ti
}

// WithRootTraceID 将根链路追踪 ID 写入 context
func WithRootTraceID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, RootTraceIDKey, id)
}

// WithCurrentSpanID 将当前 span ID 写入 context
func WithCurrentSpanID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, CurrentSpanIDKey, id)
}

func stringFromCtx(ctx context.Context, key any) string {
	v, _ := ctx.Value(key).(string)
	return v
}

func stringSliceFromCtx(ctx context.Context, key any) []string {
	v, _ := ctx.Value(key).([]string)
	return v
}
