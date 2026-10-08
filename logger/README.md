# logger — 结构化日志

> 完整文档见 [logger/docs](docs/README.md)

## 功能概述

提供带级别、调用点、trace 链路字段的结构化日志：

- **五级日志**：DEBUG / INFO / WARN / ERROR / FATAL
- **双格式**：text（人读）/ JSON（机器采集，字段对齐 OTel）
- **双输出**：控制台 + 文件（按天滚动、过期自动清理）
- **链路字段**：自动携带 `trace_id` / `span_id`（来自 gctx 或 OTel 中间件）
- **默认实例开箱即用**，也支持自定义实例与全局替换
- **可选 OTLP 导出**：通过 `logger/otlpsink` 子包上报到 OTel Collector

## 最小示例

```go
import "github.com/memory198/go-gear/logger"

logger.Info(ctx, "service started", "port", 8080)
```

详细用法、配置项、链路追踪集成、OTLP 导出、FAQ 等见 [logger/docs](docs/README.md)。
