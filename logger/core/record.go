package core

import (
	"context"
	"time"
)

// Record 一条日志的结构化记录
// 字段命名对齐 OTel Logs Data Model；在序列化（JSON/text）之前，
// 由 logger 主包构造并分发给已注册的 Hook（如 OTLP 导出）
type Record struct {
	Timestamp     time.Time // OTel Timestamp
	Level         Level     // OTel SeverityNumber（数值）
	Body          string    // OTel Body
	CodeFilepath  string    // code.filepath（Caller 开启时非空）
	CodeLineno    int       // code.lineno
	TraceID       string    // trace_id（hex）
	SpanID        string    // span_id（hex）
	ParentSpanIDs []string  // parent_span_ids（可选，默认关闭）
	Resource      Resource  // 部署/服务元信息
	Attrs         []Attr    // 自定义键值对（有序）
}

// Attr 单个键值对
type Attr struct {
	Key   string
	Value any
}

// Resource 部署/服务元信息（OTel Resource 语义约定）
type Resource struct {
	ServiceName    string // service.name
	ServiceVersion string // service.version
	Environment    string // deployment.environment
	HostName       string // host.name
}

// Empty 是否所有字段均为空
func (r Resource) Empty() bool {
	return r.ServiceName == "" && r.ServiceVersion == "" &&
		r.Environment == "" && r.HostName == ""
}

// Emitter 结构化日志消费点：在 JSON/text 序列化之前被调用
// 实现方不得保留 Record 指针（其在后续日志写入中可能被复用）
type Emitter interface {
	Emit(ctx context.Context, r *Record)
}

// HookFunc 函数适配器：便于用普通函数作为 Emitter 注册
type HookFunc func(ctx context.Context, r *Record)

// Emit 实现 Emitter
func (f HookFunc) Emit(ctx context.Context, r *Record) { f(ctx, r) }

// Flusher 可由 Emitter 额外实现：在 Fatal 等退出路径上被调用以刷新缓冲
// （如 OTLP 批量导出器需在进程退出前 ForceFlush）
type Flusher interface {
	Flush(ctx context.Context) error
}
