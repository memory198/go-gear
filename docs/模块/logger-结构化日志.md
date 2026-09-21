# logger 模块说明（结构化日志）

> 所属目录：`logger/`
> 架构总览见 [架构设计](../架构设计.md) | 使用见 [使用指南](../使用指南.md)

## 职责

输出带级别、调用点、trace 链路字段的日志：text（人读）与 JSON（机器采集）双格式，控制台 + 文件双输出，按天滚动 + 过期清理。

## 核心设计

### 1. 默认实例 + 包级快捷函数

```go
logger.Info(ctx, "creating user")   // 开箱即用
l, _ := logger.New(cfg)
logger.SetDefault(l)                 // 全局替换（atomic.Pointer，线程安全）
defer logger.Close()
```

### 2. 五级日志

DEBUG / INFO / WARN / ERROR / FATAL。FATAL 输出后 `os.Exit(1)`（defer 不会执行，见注释说明）。每级提供 `Xxx(ctx, msg)`（ctx 必传，自动携带 trace 字段）与 `Xxxf(ctx, format, ...)` 两种形式；`msg` 后可跟 slog 键值对参数：`Info(ctx, "user created", "user_id", 123)`。

### 无 ctx 打印（log 风格）

启动、后台任务等非请求场景没有 ctx，使用 `Print` / `Printf`（INFO 级，不携带 trace 字段）：

```go
logger.Print("server starting", "port", 8080)   // 可带键值对
logger.Printf("listening on %s:%d", "0.0.0.0", 8080)
```

### 3. trace 三元组（与追踪体系对接的唯一契约）

`RootTraceID` / `MiddleSpanIDs` / `CurrentSpanID` 从 context 中读取（key 定义在 `trace.go`，logger **只读不写**）：

| key | 含义 |
|---|---|
| `RootTraceIDKey` | 根链路 trace ID |
| `MiddleSpanIDsKey` | 中间 span ID 链（数组） |
| `CurrentSpanIDKey` | 当前 span ID |

数据由 gctx 或 middleware.OTel 动态提供（见 [架构设计 §五](../架构设计.md)）。

**输出字段映射**：`RootTraceIDKey` → `trace_id`；`CurrentSpanIDKey` → `span_id`；`MiddleSpanIDsKey` → `parent_span_ids`（默认关闭，需 `Config.ParentSpanIDs` 开启）。

### 4. 双编码器（手写实现，性能优于反射序列化）

- `textEncoder`：人读优先，展示 `timestamp [severity_text] [trace_id] code.filepath:lineno body` 与 `k=v` 属性；省略 resource / parent_span_ids
- `jsonEncoder`：**手写 JSON 编码器**（无反射），字段对齐 OTel：
  `timestamp` / `severity_number` / `severity_text` / `body` / `code.filepath` / `code.lineno` / `trace_id` / `span_id` / `parent_span_ids` / `resource{...}` / `attributes{...}`
- 时间格式：RFC3339 + 微秒 + 时区偏移（例 `2026-09-21T10:30:00.123456+08:00`）
- 输出缓冲通过 `sync.Pool` 复用，默认路径仅 3-4 次分配/条

### 5. 输出管理

- console 与 file 可**同时存在**（`Console` / `FileDir` 独立开关）
- 按天滚动（`app.2026-09-02.log`），`MaxAge` 自动清理过期文件，文件名前缀隔离多实例
- 未配置任何输出时兜底写 stdout

### 6. caller 定位

`findCaller` 跳过 logger 包与 runtime 内部帧，输出一行直接调用位置（相对路径:行号），**非调用栈**；同一调用点二次命中缓存，性能开销极小。由 `Config.Caller` 开关控制（默认 true）。

### 7. Hook 与 OTLP 导出（可选）

数据契约与扩展包：

```
logger/core         Level / Record / Attr / Resource / Hook（零依赖，供扩展共享）
    ↑        ↑
logger          logger/otlpsink     主包零 otel 依赖；otlpsink 不依赖主包
```

- **`AddHook(h Hook)`**：在序列化之前分发结构化 `core.Record`（atomic 存储，未注册时热路径零开销）
- **`logger/otlpsink`**（可选）：实现 `core.Hook`，把 Record 翻译为 OTel LogRecord 经 OTLP/HTTP 导出到 Collector；提供 `WithEndpoint/WithInsecure/WithService/...` 选项与 `Shutdown`
- **链路关联**：ctx 已含 OTel span context 时天然对齐；否则用 Record 的 gctx hex id 注入 SpanContext

```go
sink, shutdown, err := otlpsink.New(ctx,
    otlpsink.WithEndpoint("localhost:4318"), otlpsink.WithService("user-api", "1.2.0"))
if err == nil {
    defer shutdown(ctx)
    l.AddHook(sink.Emit)
}
```

## 配置项（`Config`）

`Level` / `Format`(text|json) / `Console` / `FileDir` / `Filename` / `MaxAge` / `Caller`(bool) / `ParentSpanIDs`(bool，默认 false) / `Service` / `Version` / `Env` / `Host`(resource 元信息)

## 与其他模块的关系

- **零项目内依赖**，只依赖标准库——但被 gctx 引用（gctx 仅引用 3 个 key 常量）
- `core` 子包提供数据契约（Level/Record/Attr/Resource/Hook），主包通过类型别名兼容
- `otlpsink` 子包可选实现 OTLP 导出（依赖 core + otel sdk，主包不感知）
- 与 framework 解耦：middleware.OTel 负责桥接，logger 本身不感知 HTTP

## 边界 / 刻意不做

- **不做自动日志上报/采样**：OTLP 导出为可选 Hook，默认不启用；不内置采样/限流
- 时间戳由编码器统一格式化为 RFC3339（微秒 + 时区）

## 相关文档

- [架构设计](../架构设计.md)：§五 trace 链路如何贯通
- [使用指南](../使用指南.md)：一、Logger
- [gctx-请求上下文](gctx-请求上下文.md)：trace 数据提供方之一
- [middleware-中间件](middleware-中间件.md)：OTel 桥接
