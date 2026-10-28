# OTLP 导出

> 所属：[logger 模块文档](README.md)

希望日志与 trace 在同一个后端（Loki/Tempo）互跳时，挂载 OTLP 导出。

## 何时需要

- 已在 OTel 生态（有 Collector），希望点开 span 直接看到该 trace 的日志
- 免去在采集端（Promtail/filelog）配置 JSON 解析规则
- 若日志已通过文件采集到 Loki/ELK，可不用此功能

## 接入方式

```go
import "github.com/memory198/go-gear/logger/otlpsink"

lcfg := logger.Config{Service: "user-api", Version: "1.2.0", Env: "prod"}
l, _ := logger.New(lcfg)

sink, shutdown, err := otlpsink.New(ctx, otlpsink.Config{
    Endpoint: "localhost:4318",
    Insecure: true,                  // 默认启用 TLS，本地明文需显式开启
    Resource: lcfg.Resource(),       // 与 logger 的 resource 保持一致
})
if err == nil {
    defer shutdown(ctx)              // 退出前刷新缓冲
    l.AddHook(sink)
}
logger.SetDefault(l)
```

> 零值字段会使用默认值（Endpoint=localhost:4318、批量间隔 5s、TLS 开启）；可用 `otlpsink.DefaultConfig()` 获取完整默认配置。
> `sink` 实现了 `core.Flusher`：`logger.Fatal`/`Fatal` 与 `Logger.Close` 会自动并行刷新缓冲（2 秒超时）。正常退出仍建议 `defer shutdown(ctx)`；请保证 shutdown 在 `Close` 之后执行（defer 先注册者后执行）。

## 说明

- **落盘输出不受影响**：OTLP 是旁路，stdout/file 照常
- **链路关联**：ctx 已含 OTel span context 时直接使用；仅 gctx 时用其 hex id 注入，日志与 span 的 trace_id/span_id 自动对齐
- **severity**：日志级别数值即 OTel SeverityNumber，无需映射
- **失败容错**：批量异步导出，Collector 不可用不会阻塞业务
- **依赖**：`otlpsink` 为可选子包，主包 `logger` 不引入任何 otel 依赖

## 相关文档

- [../../docs/OTel接入指南.md](../../docs/OTel接入指南.md)：完整 OTel 接入步骤（中间件 + 日志导出）
- [架构设计](架构设计.md)：Hook/Emitter 机制
