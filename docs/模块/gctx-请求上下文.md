# gctx 模块说明（请求上下文）

> 所属目录：`gctx/`
> 架构总览见 [架构设计](../架构设计.md) | 使用见 [使用指南](../使用指南.md)

## 职责

提供标准 `context.Context` 不具备的请求级能力：**请求追踪、span 链管理、超时控制、值存储**。

## 核心设计

### 1. 结构：内嵌 context + parent 父链

```go
type Context struct {
    context.Context // 内嵌标准 context：仅用于超时/取消传播
    parent *Context // 父上下文指针：用于 value 回溯
    ...
}
```

### 2. 追踪字段（OTel 兼容格式）

`traceID` / `spanID` / `parentSpanID` / `middleSpanIDs`，全部由 `crypto/rand` 生成 hex 格式：trace 32 位（128-bit）、span 16 位（64-bit），与 OTel/W3C 规范一致。

### 3. 入口提取

`NewContext(r, w)` 提取链路来源，优先级：

1. W3C `traceparent` 头（`version-trace_id-parent_id-flags`，校验长度 32/16）
2. `X-Trace-ID` / `X-Span-ID` 兜底
3. 都没有 → 自动生成新 trace/span ID

### 4. 派生 API

| 方法 | 行为 |
|---|---|
| `StartSpan()` | 新 spanID，旧 spanID 推入中间链，返回子 Context |
| `WithTimeout(d)` / `WithTimeoutFunc(d, cb)` | 带超时的子上下文（回调在超时 goroutine 中执行） |
| `WithDeadline(t)` | 带截止时间的子上下文 |
| `Set(key, value)` | 本层存值，重复 key panic；派生上下文不继承父层 values |
| `Cancel()` | 取消当前上下文及传播 |

### 5. Value 读取顺序

```
logger 的 trace key → 本层 values → 沿 parent 链回溯 → 内嵌标准 context
```

**关键点**：`Value()` 识别 logger 定义的 `RootTraceIDKey` / `MiddleSpanIDsKey` / `CurrentSpanIDKey`，**动态返回自身字段**——`MiddleSpanIDs()` 从父链实时聚合（递归收集各层 spanID），派生上下文无需任何维护即可提供完整 span 链。

## 与其他模块的关系

- 依赖 `logger`：仅引用其 3 个 key 常量（gctx → logger 的轻依赖）
- 被 `framework` 再导出：`framework.Context = gctx.Context`（类型别名）
- **作为 logger 的 trace 数据提供方之一**（不接 OTel 的场景）

## 边界 / 刻意不做

- values 仅支持 **string key**（标准库 context 的 key 惯例不适用本层存储）
- 派生上下文不继承父层 values（可自由覆盖，无拷贝开销）
- 追踪为轻量内置方案，不替代 OTel；接 OTel 时建议用 `framework.Handle` 而非 `HandleContext`（避免两套 ID）

## 相关文档

- [架构设计](../架构设计.md)：§五 trace 链路、§六 请求生命周期
- [使用指南](../使用指南.md)：四、Gctx
- [logger-结构化日志](logger-结构化日志.md)：key 契约的读取方
- [framework-HTTP框架](framework-HTTP框架.md)：Context 别名再导出
