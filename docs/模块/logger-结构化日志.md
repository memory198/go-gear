# logger 模块说明（结构化日志）

> 所属目录：`logger/`
> 架构总览见 [架构设计](../架构设计.md) | 使用见 [使用指南](../使用指南.md)

## 职责

输出带级别、调用点、trace 链路字段的日志：text（人读）与 JSON（机器采集）双格式，控制台 + 文件双输出，按天滚动 + 过期清理。

> logger 模块的完整文档（快速开始、核心概念、配置参考、链路追踪集成、OTLP 导出、架构设计、FAQ）已迁移至 **[logger/docs](../../logger/docs/README.md)**，本文件仅作为 `docs/模块/` 索引下的跳转入口，不再维护重复内容。

## 相关文档

- [logger/docs](../../logger/docs/README.md)：logger 模块完整文档（推荐先看）
- [架构设计](../架构设计.md)：§五 trace 链路如何贯通
- [使用指南](../使用指南.md)：一、Logger
- [gctx-请求上下文](gctx-请求上下文.md)：trace 数据提供方之一
- [middleware-中间件](middleware-中间件.md)：OTel 桥接
