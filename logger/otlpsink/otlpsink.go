// Package otlpsink 提供 logger 的 OTLP 日志导出（可选功能）：
// 将结构化日志记录通过 OTLP/HTTP 发送到 OTel Collector，与 span 链路在同一后端关联。
//
// 依赖隔离：本包是 logger 的子包，主包 logger 不引入任何 otel 依赖；
// 不使用本功能时无需引入本包。
package otlpsink

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	otellog "go.opentelemetry.io/otel/log"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/trace"

	"github.com/memory198/go-gear/logger/core"
)

const (
	defaultEndpoint      = "localhost:4318"
	defaultBatchInterval = 5 * time.Second
)

// Config OTLP 导出配置（直接构造传入，零值字段使用默认值）
type Config struct {
	// Endpoint Collector 的 OTLP/HTTP 地址（host:port）；默认 localhost:4318
	Endpoint string
	// Insecure 关闭 TLS（默认启用 TLS）；本地/内网明文传输需显式设为 true
	Insecure bool
	// Headers 额外的请求头（如鉴权 token）
	Headers map[string]string
	// BatchInterval 批量导出间隔；<=0 使用默认 5s
	BatchInterval time.Duration
	// Resource 服务元信息，建议与 logger 保持一致：
	//   ocfg.Resource = loggerCfg.Resource()
	Resource core.Resource
}

// DefaultConfig 返回推荐默认配置（localhost:4318、TLS 开启、批量间隔 5s）
func DefaultConfig() Config {
	return Config{
		Endpoint:      defaultEndpoint,
		BatchInterval: defaultBatchInterval,
	}
}

// Sink OTLP 日志导出器，实现 core.Emitter（并实现 core.Flusher）
type Sink struct {
	provider *sdklog.LoggerProvider
	logger   otellog.Logger
}

// New 按配置创建 Sink 并返回关闭函数
// 关闭函数用于进程退出前刷新缓冲（必须调用，否则最后一批日志可能丢失）
func New(ctx context.Context, cfg Config) (*Sink, func(context.Context) error, error) {
	if cfg.Endpoint == "" {
		cfg.Endpoint = defaultEndpoint
	}
	if cfg.BatchInterval <= 0 {
		cfg.BatchInterval = defaultBatchInterval
	}

	expOpts := []otlploghttp.Option{otlploghttp.WithEndpoint(cfg.Endpoint)}
	if cfg.Insecure {
		expOpts = append(expOpts, otlploghttp.WithInsecure())
	}
	if len(cfg.Headers) > 0 {
		expOpts = append(expOpts, otlploghttp.WithHeaders(cfg.Headers))
	}
	exp, err := otlploghttp.New(ctx, expOpts...)
	if err != nil {
		return nil, nil, fmt.Errorf("otlpsink: create exporter: %w", err)
	}

	provider := sdklog.NewLoggerProvider(
		sdklog.WithResource(buildResource(cfg.Resource)),
		sdklog.WithProcessor(sdklog.NewBatchProcessor(exp,
			sdklog.WithExportInterval(cfg.BatchInterval),
		)),
	)

	return &Sink{provider: provider, logger: provider.Logger("go-gear")}, provider.Shutdown, nil
}

// Emit 实现 core.Emitter：把结构化记录翻译为 OTel LogRecord 并提交批量导出
//
// 链路关联：OTel Logs API 的 trace/span 由 ctx 携带（非 Record 字段）——
// 若 ctx 已含 OTel span context（挂了 middleware.OTel）则天然对齐；
// 否则用 Record 中的 gctx 来源 hex id 构造 SpanContext 注入 ctx。
func (s *Sink) Emit(ctx context.Context, r core.Record) {
	if !trace.SpanContextFromContext(ctx).IsValid() {
		ctx = contextWithTraceIDs(ctx, r.TraceID, r.SpanID)
	}

	var rec otellog.Record
	rec.SetTimestamp(r.Timestamp)
	rec.SetSeverity(otellog.Severity(r.Level)) // Level 数值即 OTel SeverityNumber
	rec.SetSeverityText(r.Level.String())
	rec.SetBody(attribute.StringValue(r.Body))

	if r.CodeFilepath != "" {
		rec.AddAttributes(
			attribute.String("code.filepath", r.CodeFilepath),
			attribute.Int("code.lineno", r.CodeLineno),
		)
	}

	for _, a := range r.Attrs {
		rec.AddAttributes(attrFromAny(a.Key, a.Value))
	}

	s.logger.Emit(ctx, rec)
}

// Flush 强制刷新批量缓冲（实现 core.Flusher，Fatal 退出前会被调用）
func (s *Sink) Flush(ctx context.Context) error {
	return s.provider.ForceFlush(ctx)
}

// contextWithTraceIDs 用 hex trace/span id 构造 SpanContext 注入 ctx
// 使 SDK 导出时能填入 trace_id / span_id（gctx 场景）
func contextWithTraceIDs(ctx context.Context, traceIDHex, spanIDHex string) context.Context {
	if traceIDHex == "" {
		return ctx
	}
	tid, err := trace.TraceIDFromHex(traceIDHex)
	if err != nil {
		return ctx
	}
	cfg := trace.SpanContextConfig{TraceID: tid, TraceFlags: trace.FlagsSampled}
	if sid, err := trace.SpanIDFromHex(spanIDHex); err == nil {
		cfg.SpanID = sid
	}
	// 说明：日志已实际采集，故标记为 sampled（避免后端因未采样而丢弃）
	return trace.ContextWithSpanContext(ctx, trace.NewSpanContext(cfg))
}

// buildResource 构造 OTel Resource（语义约定键）
func buildResource(r core.Resource) *resource.Resource {
	attrs := make([]attribute.KeyValue, 0, 4)
	if r.ServiceName != "" {
		attrs = append(attrs, attribute.String("service.name", r.ServiceName))
	}
	if r.ServiceVersion != "" {
		attrs = append(attrs, attribute.String("service.version", r.ServiceVersion))
	}
	if r.Environment != "" {
		attrs = append(attrs, attribute.String("deployment.environment", r.Environment))
	}
	if r.HostName != "" {
		attrs = append(attrs, attribute.String("host.name", r.HostName))
	}
	return resource.NewWithAttributes("", attrs...)
}

// attrFromAny 将任意值转为 OTel attribute（复杂类型回退为字符串）
func attrFromAny(key string, v any) attribute.KeyValue {
	switch x := v.(type) {
	case nil:
		return attribute.String(key, "<nil>")
	case string:
		return attribute.String(key, x)
	case bool:
		return attribute.Bool(key, x)
	case int:
		return attribute.Int(key, x)
	case int64:
		return attribute.Int64(key, x)
	case float64:
		return attribute.Float64(key, x)
	case []string:
		return attribute.StringSlice(key, x)
	default:
		return attribute.String(key, fmt.Sprint(x))
	}
}
