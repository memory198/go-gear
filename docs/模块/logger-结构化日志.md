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

### 4. 双编码器（`encoder` 接口）

- `textEncoder`：只展示 root_trace_id，控制人读噪音
- `jsonEncoder`：完整携带三元组，`middle_span_ids` 为数组（`{"time","level","msg","caller","root_trace_id","middle_span_ids","current_span_id"}`）

### 5. 输出管理

- console 与 file 可**同时存在**（`Console` / `FileDir` 独立开关）
- 按天滚动（`app.2026-09-02.log`），`MaxAge` 自动清理过期文件，文件名前缀隔离多实例
- 未配置任何输出时兜底写 stdout

### 6. caller 定位

`findCaller` 跳过 logger 包与 runtime 内部帧，输出一行直接调用位置（相对路径:行号），**非调用栈**；同一调用点二次命中缓存，性能开销极小。由 `Config.Caller` 开关控制（默认 true）。

## 配置项（`Config`）

`Level` / `Format`(text|json) / `Console` / `FileDir` / `Filename` / `MaxAge` / `Caller`(bool) / `MiddleSpanIDs`(bool，默认 false)

## 与其他模块的关系

- **零项目内依赖**，只依赖标准库——但被 gctx 引用（gctx 仅引用 3 个 key 常量）
- 与 framework 解耦：middleware.OTel 负责桥接，logger 本身不感知 HTTP

## 边界 / 刻意不做

- **不提供 kv 字段 API**（如 slog 风格），维持 `Info(ctx, msg)` 简洁签名；结构化定位由 trace 字段 + JSON 承担，若未来需要字段级检索，计划迁移标准库 `log/slog` 底层
- 时间格式为固定字符串（非 RFC3339）

## 相关文档

- [架构设计](../架构设计.md)：§五 trace 链路如何贯通
- [使用指南](../使用指南.md)：一、Logger
- [gctx-请求上下文](gctx-请求上下文.md)：trace 数据提供方之一
- [middleware-中间件](middleware-中间件.md)：OTel 桥接
