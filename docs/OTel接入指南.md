# OpenTelemetry 接入指南

## 概述

go-gear 通过 `middleware.OTel` 中间件接入 OpenTelemetry，实现请求链路的自动追踪和上报。

核心思路：**框架提供中间件，开发者配置 Exporter**。中间件负责创建 Span、桥接 logger 链路字段；Exporter 由开发者按需选择（Jaeger / OTLP / 控制台）。

---

## 架构

```
  Client
    │  HTTP 请求（Header 携带 W3C TraceContext 或 X-Trace-ID）
    ▼
┌─────────────────┐
│  middleware.OTel │  ← 一行代码接入
│                 │
│ 1. 提取上游 Trace
│ 2. 创建 Server Span
│ 3. 写入 logger 链路字段
│ 4. 调用 next.ServeHTTP()
│ 5. 记录状态码/错误 + Span.End()
└────────┬────────┘
         │
┌────────┴────────┐
│  framework.Handle│  ← 业务代码零侵入
│  logger.Debug/   │     日志自动带 trace_id / span_id
│  Info/Error...   │
└────────┬────────┘
         │
┌────────┴────────┐
│  OTel SDK        │
│  TracerProvider  │  ← 开发者配置
│  + Exporter      │
└────────┬────────┘
         │ OTLP / gRPC / HTTP
         ▼
┌────────┐  ┌────────┐  ┌────────────┐
│ Jaeger │  │ OTLP   │  │ Prometheus │
│        │  │Collector│  │            │
└────────┘  └────────┘  └────────────┘
```

---

## 开发者需要做的工作

### 第一步：安装依赖

```bash
go get go.opentelemetry.io/otel
go get go.opentelemetry.io/otel/exporters/otlp/otlptracehttp
go get go.opentelemetry.io/otel/sdk/trace
```

### 第二步：配置 TracerProvider 和 Exporter

在 `main.go` 中初始化 OTel，选择一种 Exporter：

**方案 A：OTLP Collector（生产环境推荐）**

```go
import (
    "context"
    "log"

    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/otlp/otlptracehttp"
    "go.opentelemetry.io/otel/propagation"
    sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func initOTel() *sdktrace.TracerProvider {
    exp, err := otlptracehttp.New(context.Background(),
        otlptracehttp.WithEndpoint("otel-collector:4318"),
        otlptracehttp.WithInsecure(),
    )
    if err != nil {
        log.Fatal(err)
    }

    tp := sdktrace.NewTracerProvider(
        sdktrace.WithBatcher(exp),
        sdktrace.WithSampler(sdktrace.TraceIDRatioBased(0.1)), // 生产环境 10% 采样
    )
    otel.SetTracerProvider(tp)
    otel.SetTextMapPropagator(propagation.TraceContext{})
    return tp
}
```

**方案 B：Jaeger（gRPC）**

```go
import "go.opentelemetry.io/otel/exporters/jaeger"

exp, _ := jaeger.New(jaeger.WithCollectorEndpoint(
    jaeger.WithEndpoint("http://jaeger:14268/api/traces"),
))
```

**方案 C：控制台输出（本地调试）**

```go
import "go.opentelemetry.io/otel/exporters/stdout/stdouttrace"

exp, _ := stdouttrace.New(stdouttrace.WithPrettyPrint())
```

### 第三步：挂中间件

```go
func main() {
    tp := initOTel()
    defer func() {
        if err := tp.Shutdown(context.Background()); err != nil {
            log.Printf("OTel shutdown: %v", err)
        }
    }()

    r := chi.NewRouter()
    r.Use(middleware.OTel("my-service"))  // ← 一行接入
    r.Use(middleware.Recoverer)

    r.Post("/users", framework.Handle(createUser))
    http.ListenAndServe(":8080", r)
}
```

### 第四步：业务代码无需改动

```go
func createUser(ctx context.Context, req *CreateUserReq) (*User, error) {
    logger.Info(ctx, "creating user")   // 日志自动带 OTel trace_id
    user, err := repo.Create(ctx, req)  // ctx 透传即可，下游调用可继续传播
    if err != nil {
        logger.Error(ctx, "create user failed")
        return nil, err
    }
    return user, nil
}
```

### 完整 main.go 示例

```go
package main

import (
    "context"
    "log"
    "net/http"

    "github.com/go-chi/chi"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/otlp/otlptracehttp"
    "go.opentelemetry.io/otel/propagation"
    sdktrace "go.opentelemetry.io/otel/sdk/trace"

    "github.com/memory198/go-gear/framework"
    "github.com/memory198/go-gear/framework/middleware"
    "github.com/memory198/go-gear/logger"
)

func main() {
    // ===== OTel 初始化 =====
    exp, err := otlptracehttp.New(context.Background(),
        otlptracehttp.WithEndpoint("localhost:4318"),
        otlptracehttp.WithInsecure(),
    )
    if err != nil {
        log.Fatal(err)
    }
    tp := sdktrace.NewTracerProvider(
        sdktrace.WithBatcher(exp),
        sdktrace.WithSampler(sdktrace.AlwaysSample()),
    )
    otel.SetTracerProvider(tp)
    otel.SetTextMapPropagator(propagation.TraceContext{})
    defer tp.Shutdown(context.Background())

    // ===== Router + 中间件 =====
    r := chi.NewRouter()
    r.Use(middleware.OTel("my-service"))
    r.Use(middleware.Recoverer)

    r.Post("/users", framework.Handle(createUser))

    logger.Info(context.Background(), "server starting on :8080")
    log.Fatal(http.ListenAndServe(":8080", r))
}

type CreateUserReq struct {
    Name string `json:"name" validate:"required"`
}
type User struct{ ID, Name string }

func createUser(ctx context.Context, req *CreateUserReq) (*User, error) {
    logger.Info(ctx, "creating user")
    return &User{ID: "1", Name: req.Name}, nil
}
```

---

## 中间件做了什么

| 阶段 | 行为 |
|------|------|
| 请求进入 | 从 Header 提取 W3C TraceContext + X-Trace-ID 兜底 |
| 创建 Span | `tracer.Start(ctx, "POST /users", SpanKindServer)` |
| 桥接 logger | 写入 `RootTraceIDKey`/`CurrentSpanIDKey` → 日志自动带链路字段 |
| 请求处理 | `next.ServeHTTP()` |
| 记录属性 | `http.method`、`http.status_code`、`http.route` |
| 错误标记 | HTTP >= 400 时 `span.SetStatus(codes.Error, ...)` |
| 结束 Span | `defer span.End()` |

开发者**不需要**手动在 handler 里创建 Span、设置 trace ID 或传递链路信息，这些全部由中间件自动完成。

---

## 日志中的链路字段

OTel 中间件挂上后，logger 输出自动包含 trace 链路字段：

**text 格式**：

```
2026-09-21T10:30:00.123456+08:00 [INFO] [4bf92f3577b34da6a3ce929d0e0e4736] handler/user.go:42 creating user
```

**json 格式**：

```json
{"timestamp":"2026-09-21T10:30:00.123456+08:00","severity_text":"INFO","body":"creating user",
 "code.filepath":"handler/user.go","code.lineno":42,
 "trace_id":"4bf92f3577b34da6a3ce929d0e0e4736","span_id":"00f067aa0ba902b7"}
```

通过 `trace_id` 即可在 Jaeger 等平台精确定位整条请求链路的所有日志。

---

## 日志 OTLP 导出（可选）

除 span 外，日志也可通过 OTLP 导出到同一个 Collector，实现**日志 ↔ trace 同后端互跳**。

### 何时需要

- 已在 OTel 生态（有 Collector），希望点开 span 直接看到该 trace 的日志
- 免去在采集端（Promtail/filelog）配置 JSON 解析规则
- 若日志已通过文件采集到 Loki/ELK，可不用此功能

### 接入方式

```go
import "github.com/memory198/go-gear/logger/otlpsink"

l, _ := logger.New(logger.Config{
    Format: logger.JSONFormat, Console: true,
    Service: "user-api", Version: "1.2.0", Env: "prod",   // resource 与 span 保持一致
})

sink, shutdown, err := otlpsink.New(ctx,
    otlpsink.WithEndpoint("localhost:4318"),   // Collector 的 OTLP/HTTP 端口
    otlpsink.WithInsecure(),                   // 本地/内网可关闭 TLS
    otlpsink.WithService("user-api", "1.2.0"),
    otlpsink.WithEnvironment("prod"),
)
if err != nil {
    logger.Print("otlp sink disabled: ", err)  // 导出不可用不影响落盘日志
} else {
    defer shutdown(ctx)                        // 进程退出前必须调用，刷新批量缓冲
    l.AddHook(sink)
}
logger.SetDefault(l)
```

### 说明

- **落盘输出不受影响**：OTLP 是旁路，stdout/file 照常
- **链路关联**：ctx 已含 OTel span context 时直接使用；仅 gctx 时用其 hex id 注入，日志与 span 的 trace_id/span_id 自动对齐
- **severity**：日志级别数值即 OTel SeverityNumber，无需映射
- **失败容错**：批量异步导出，Collector 不可用不会阻塞业务
- **依赖**：`otlpsink` 为可选子包，主包 logger 不引入任何 otel 依赖

---

## 采样策略

| 策略 | 用法 | 适用场景 |
|------|------|---------|
| 全量 | `AlwaysSample()` | 开发环境、调试 |
| 概率 | `TraceIDRatioBased(0.1)` | 生产环境（10%） |
| 关闭 | `NeverSample()` | 压测环境 |

```go
tp := sdktrace.NewTracerProvider(
    sdktrace.WithBatcher(exp),
    sdktrace.WithSampler(sdktrace.TraceIDRatioBased(0.1)),
)
```

---

## 不接入 OTel 时

不挂 `middleware.OTel` 中间件，所有 go-gear 功能正常运行，仅 logger 中链路字段为空。go-gear 对 OTel 是零强制依赖。
