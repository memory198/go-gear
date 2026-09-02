# go-gear 使用指南

## 概述

go-gear 是一个轻量级 Go Web 服务工具集，提供 **配置管理**、**带堆栈的错误处理**、**泛型 HTTP Handler**、**结构化日志** 和 **请求上下文**。不绑定路由库，推荐搭配 [chi](https://github.com/go-chi/chi) 使用。

---

## 快速开始

### 安装

```bash
go get github.com/memory198/go-gear
```

### 最小示例

```go
package main

import (
    "context"
    "net/http"

    "github.com/go-chi/chi"
    "github.com/memory198/go-gear/framework"
    "github.com/memory198/go-gear/framework/middleware"
    "github.com/memory198/go-gear/logger"
)

func main() {
    r := chi.NewRouter()
    r.Use(middleware.Recoverer)
    r.Post("/users", framework.Handle(createUser))

    http.ListenAndServe(":8080", r)
}

type CreateUserReq struct {
    Name  string `json:"name" validate:"required"`
    Email string `json:"email" validate:"required,email"`
}

func createUser(ctx context.Context, req *CreateUserReq) (*User, error) {
    logger.Info(ctx, "creating user")
    return &User{ID: "1", Name: req.Name}, nil
}
```

---

## 目录结构推荐

```
project/
├── main.go
├── config.yaml
├── handler/       # framework.Handle 的 handler 函数
├── service/       # 业务逻辑
├── repo/          # 数据访问
└── model/         # 请求/响应结构体
```

---

## 各模块详解

### 一、Logger

日志模块，支持 DEBUG/INFO/WARN/ERROR/FATAL 五级、text/json 双格式、控制台+文件双渠道。

**包级快捷调用（开箱即用）**

```go
import "github.com/memory198/go-gear/logger"

logger.Debug(ctx, "debug message")
logger.Info(ctx, "info message")
logger.Warn(ctx, "warning message")
logger.Error(ctx, "error message")
logger.Infof(ctx, "user %s login, age=%d", "alice", 25)
```

**自定义配置**

```go
l, _ := logger.New(logger.Config{
    Level:   logger.DEBUG,          // 最低输出等级
    Format:  logger.JSONFormat,     // text 或 json
    Console: true,                  // 控制台输出
    FileDir: "/var/log/app",        // 文件输出目录（空=不写文件）
    MaxAge:  30,                    // 日志保留天数
    Caller:  true,                  // 打印调用文件:行号
})
logger.SetDefault(l)
defer logger.Close()
```

**日志格式**

```
// text 格式（默认）
2026-07-14 10:30:00.123456 [INFO] handler/user.go:42 creating user
2026-07-14 10:30:00.123456 [INFO] [abc123] handler/user.go:42 creating user  // 有 trace 时

// json 格式
{"time":"...","level":"INFO","msg":"creating user","caller":"handler/user.go:42",
 "root_trace_id":"abc123","middle_span_ids":["s1","s2"],"current_span_id":"s3"}
```

| 配置项 | 说明 | 默认值 |
|--------|------|--------|
| `Level` | 最低输出等级 | DEBUG |
| `Format` | TextFormat 或 JSONFormat | TextFormat |
| `Console` | 是否输出到控制台 | true |
| `FileDir` | 文件输出目录 | "" |
| `Filename` | 文件名（不含扩展名） | 程序名 |
| `MaxAge` | 日志保留天数 | 0 |
| `Caller` | 是否打印调用位置 | true |

---

### 二、Framework — 泛型 HTTP Handler

`framework.Handle` 将业务函数包装为 `http.HandlerFunc`，自动完成 **JSON 绑定 → 参数校验 → 调用业务逻辑 → 统一响应**。

**Handler 签名**

```go
type Handler[Req, Res any] func(ctx context.Context, req *Req) (*Res, error)
```

**完整示例**

```go
type CreateUserReq struct {
    Name  string `json:"name" validate:"required"`
    Email string `json:"email" validate:"required,email"`
}
type User struct {
    ID    string `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email"`
}

func createUser(ctx context.Context, req *CreateUserReq) (*User, error) {
    if req.Name == "admin" {
        return nil, framework.ErrForbidden.WithMsg("name reserved")
    }
    return &User{ID: "123", Name: req.Name, Email: req.Email}, nil
}

// 注册
r.Post("/users", framework.Handle(createUser))
```

**统一响应格式**

```json
// 成功
{"code": 0, "message": "ok", "data": {"id": "123", "name": "alice"}}

// 失败
{"code": 40300, "message": "name reserved"}
```

**内置业务错误码**

| 错误 | HTTP 状态码 | 业务码 |
|------|------------|--------|
| `ErrBadRequest` | 400 | 40000 |
| `ErrUnauthorized` | 401 | 40100 |
| `ErrForbidden` | 403 | 40300 |
| `ErrNotFound` | 404 | 40400 |
| `ErrConflict` | 409 | 40900 |
| `ErrInternal` | 500 | 50000 |

**自定义 Binder（混合绑定）**

当需要从 URL 参数、Query、Header 混合绑定请求参数时，实现 `Binder` 接口：

```go
type GetUserReq struct {
    ID string
}

func (r *GetUserReq) Bind(req *http.Request) error {
    r.ID = chi.URLParam(req, "id")
    return nil
}

r.Get("/users/{id}", framework.Handle(getUser))
```

---

### 三、中间件

#### Recoverer

panic 恢复中间件，捕获后续处理器中的 panic 并返回 500。

```go
r.Use(middleware.Recoverer)
```

#### Logger

请求日志中间件，记录每个请求的 Method、Path、状态码、耗时。

```go
r.Use(middleware.Logger)
```

输出示例：`GET /users 200 12ms`

---

### 四、Gctx — 请求上下文

`gctx.Context` 提供标准 `context.Context` 不具备的能力：**请求追踪、Span 管理、超时控制、类型安全值存取**。通过 `framework.HandleContext` 使用。

```go
type ContextHandler[Req, Res any] func(ctx *Context, req *Req) (*Res, error)

r.Get("/users/{id}", framework.HandleContext(getUserWithCtx))

func getUserWithCtx(ctx *framework.Context, req *GetUserReq) (*User, error) {
    // Trace/Span 信息
    traceID := ctx.TraceID()
    spanID  := ctx.SpanID()

    // 创建子 Span
    childCtx := ctx.StartSpan()
    // 子 Span 调用下游服务...
    childCtx.SetTraceHeaders()  // 设置 X-Trace-ID / X-Span-ID 响应头

    // 超时控制
    timeoutCtx, cancel := ctx.WithTimeout(5 * time.Second)
    defer cancel()

    // 值存取（同一上下文 key 重复设置会 panic，保证不可篡改）
    ctx.Set("user_id", "123")
    val := ctx.Value("user_id")   // "123"

    // 派生上下文可以覆盖同 key 不会 panic
    derived := ctx.StartSpan()
    derived.Set("user_id", "456") // OK，不影响父上下文
}
```

---

### 五、Errors — 带堆栈的错误

直接替换标准库 `errors` 包，额外提供堆栈追踪。

```go
import "github.com/memory198/go-gear/errors"

// 创建带堆栈的错误
err := errors.Errorf("database connection refused")

// 包裹错误（记录堆栈 + 描述）
err = errors.Wrap(err, "query user failed")

// 标准库兼容
errors.Is(err, sql.ErrNoRows)
errors.As(err, &target)

// 完整堆栈链（递进打印，最底层先输出）
fmt.Printf("%+v\n", err)
// 输出：
// database connection refused
//   at repo/user.go:42 (GetUser)
// query user failed
//   at service/user.go:18 (FindUserByID)
```

---

## 路由

go-gear 不内置路由，推荐搭配 [chi](https://github.com/go-chi/chi)。`framework.Handle` 返回标准 `http.HandlerFunc`，中间件签名 `func(http.Handler) http.Handler`，可直接对接 chi：

```go
r := chi.NewRouter()
r.Use(middleware.Recoverer)
r.Route("/api", func(r chi.Router) {
    r.Post("/users", framework.Handle(createUser))
    r.Get("/users/{id}", framework.Handle(getUser))
})
```

---

## 配置

yaml 文件示例：

```yaml
server:
  addr: ":8080"
  read_timeout: 30
  write_timeout: 30

database:
  driver: postgres
  dsn: "host=localhost user=app dbname=app"

log:
  level: debug
  format: json
  console: true
  dir: /var/log/app
  max_age: 30
  caller: true
```

环境变量覆盖：

| 环境变量 | 对应配置 |
|----------|---------|
| `APP_ADDR` | server.addr |
| `APP_LOG_LEVEL` | log.level |
| `APP_LOG_FORMAT` | log.format |
| `APP_LOG_CONSOLE` | log.console |
| `APP_LOG_DIR` | log.dir |
| `APP_LOG_MAX_AGE` | log.max_age |
| `APP_LOG_CALLER` | log.caller |

---

## OpenTelemetry 接入方案

### 架构

go-gear 提供 `middleware.OTel(serviceName)` 中间件，调用方自行配置 OTel Exporter（Jaeger / OTLP / Prometheus）。

```
                       ┌──────────────────┐
                       │   OTel Collector │
                       │   / Jaeger / ... │
                       └────────┬─────────┘
                                │ OTLP / gRPC
                       ┌────────┴─────────┐
                       │  TracerProvider  │ ← main.go 配置
                       │    + Exporter    │
                       └────────┬─────────┘
                                │ Span 创建/结束
┌──────────┐   请求    ┌────────┴─────────┐
│  Client  │ ───────▶ │  OTel 中间件      │
│          │          │  1. 提取 TraceCtx │
│          │          │  2. 创建 Span     │
│          │          │  3. 桥接 logger   │
│          │          └────────┬─────────┘
│          │                   │
│          │          ┌────────┴─────────┐
│          │          │  logger / gctx   │
│          │          │  日志自动带链路ID │
│          │          └──────────────────┘
```

### 使用方式

**1. 安装 OTel SDK 和 Exporter**

```bash
go get go.opentelemetry.io/otel
go get go.opentelemetry.io/otel/exporters/otlp/otlptracehttp
go get go.opentelemetry.io/otel/sdk/trace
```

**2. main.go 配置**

```go
import (
    "context"
    "log"

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
    // 1. 配置 OTel Exporter
    exp, err := otlptracehttp.New(context.Background(),
        otlptracehttp.WithEndpoint("otel-collector:4318"),
        otlptracehttp.WithInsecure(),
    )
    if err != nil {
        log.Fatal(err)
    }

    tp := sdktrace.NewTracerProvider(
        sdktrace.WithBatcher(exp),
        sdktrace.WithSampler(sdktrace.AlwaysSample()), // 生产环境建议用概率采样
    )
    otel.SetTracerProvider(tp)
    otel.SetTextMapPropagator(propagation.TraceContext{})
    defer tp.Shutdown(context.Background())

    // 2. 挂 OTel 中间件
    r := chi.NewRouter()
    r.Use(middleware.OTel("my-service")) // ← 一行接入
    r.Use(middleware.Recoverer)

    // 3. 业务 handler
    r.Post("/users", framework.Handle(createUser))
    http.ListenAndServe(":8080", r)
}
```

**3. 业务代码零侵入**

```go
func createUser(ctx context.Context, req *CreateUserReq) (*User, error) {
    // 日志自动带 OTel trace_id 和 span_id，无需手动处理
    logger.Info(ctx, "creating user")

    // json 格式输出示例：
    // {"time":"...","level":"INFO","msg":"creating user",
    //  "root_trace_id":"a1b2c3...","current_span_id":"d4e5f6..."}

    return &User{ID: "1", Name: req.Name}, nil
}
```

### OTel 中间件做了什么

| 阶段 | 行为 |
|------|------|
| 请求进入 | 从 Header 提取 W3C TraceContext + X-Trace-ID 兜底 |
| Span 创建 | `tracer.Start(ctx, "POST /users", SpanKindServer)` |
| 桥接 logger | 将 `trace_id`、`span_id` 写入 `context.Context` 对应的 key |
| 请求处理 | `next.ServeHTTP()` |
| 记录属性 | `http.method`、`http.status_code`、`http.route` |
| 错误标记 | HTTP >= 400 时 `span.SetStatus(codes.Error, ...)` |
| Span 结束 | `defer span.End()` |

### 自定义 OTel 采集

**Jaeger（gRPC）**

```go
import "go.opentelemetry.io/otel/exporters/jaeger"

exp, _ := jaeger.New(jaeger.WithCollectorEndpoint(
    jaeger.WithEndpoint("http://jaeger:14268/api/traces"),
))
```

**本地调试（控制台输出）**

```go
import "go.opentelemetry.io/otel/exporters/stdout/stdouttrace"

exp, _ := stdouttrace.New(stdouttrace.WithPrettyPrint())
```

**采样策略**

```go
// 全部采样（开发环境）
sampler := sdktrace.AlwaysSample()

// 概率采样（生产环境，10%）
sampler := sdktrace.TraceIDRatioBased(0.1)

// 不采样（压测环境）
sampler := sdktrace.NeverSample()
```

### 没有 OTel 时

不挂 `middleware.OTel` 中间件即可，所有 go-gear 功能正常运行，只是 logger 中链路字段为空。
