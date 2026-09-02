# middleware 模块说明（HTTP 中间件）

> 所属目录：`framework/middleware/`
> 架构总览见 [架构设计](../架构设计.md) | 使用见 [使用指南](../使用指南.md)

## 职责

提供 HTTP 横切关注点的中间件，以"函数包函数"风格嵌套使用：

```go
http.Handle("/", middleware.Recoverer(middleware.OTel("my-service")(mux)))
```

## 中间件一览

| 中间件 | 职责 | 说明 |
|---|---|---|
| `Logger` | 访问日志：方法/路径/状态码/耗时 | 输出 `[METHOD] [PATH] [STATUS] [DURATION]`；当前用标准库 `log.Printf`，尚未接入 logger 包（见遗留问题） |
| `Recoverer` | 捕获 handler panic，记录堆栈并返回 500 | 防止单请求崩溃拖垮进程；对已写入部分响应的场景需注意 |
| `OTel(serviceName)` | OpenTelemetry 服务端追踪 | 见下 |

### OTel 中间件处理流程

1. 从请求头经 W3C `traceparent` 提取上游 trace context（`TextMapPropagator.Extract`）
2. 创建 Server Span：span 名为 `METHOD /path`，记录 `http.method` / `http.target` 属性
3. **桥接 logger**：把 OTel span 的 trace/span ID 经 `logger.WithRootTraceID` / `logger.WithCurrentSpanID` 写入 context → 后续所有 `logger.*` 输出自动带链路字段
4. 执行后续处理器（包装 ResponseWriter 捕获状态码）
5. 记录 `http.status_code` 属性；状态码 >= 400 时标记 span 错误（`codes.Error`）

## 与其他模块的关系

- 依赖 `framework`（Recoverer 用 `framework.WriteError` 返回 500）
- `OTel` 依赖 `logger`（桥接 trace 字段）与第三方 `go.opentelemetry.io/otel`
- **logger 的 trace 数据提供方之二**（接 OTel 的场景），与 gctx 动态提供互为替代

## 边界 / 刻意不做

- **不做中间件链管理**：没有 `Use()` 注册机制，靠函数嵌套（与框架"不造轮子"原则一致）
- `Logger` / `Recoverer` 仍用标准库 `log`，未接入项目 logger 包（访问日志无法带 trace 字段 / JSON / 文件输出）——已知遗留问题
- `wrappedWriter` / `statusWriter` 未透传 `http.Flusher`/`Hijacker`，SSE/WebSocket 场景需扩展

## 相关文档

- [架构设计](../架构设计.md)：§五 trace 链路、§六 请求生命周期
- [使用指南](../使用指南.md)：三、中间件
- [OTel接入指南](../OTel接入指南.md)：完整接入流程与采样策略
- [logger-结构化日志](logger-结构化日志.md)：trace key 契约
