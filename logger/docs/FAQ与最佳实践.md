# FAQ 与最佳实践

> 所属：[logger 模块文档](README.md)

## FAQ

**Q：日志没有输出？**
检查 `Level` 是否过滤掉了该级别（如 `WARN` 会丢弃 `Info`），以及是否配置了可用输出（`Console` 或 `FileDir`）。

**Q：为什么 JSON 里没有 attributes？**
没有传 kv 参数为正常情况；或 key 与内置字段重名被丢弃。

**Q：Fatal 之后有些日志丢了？**
`Fatal` 会 `os.Exit`，`defer` 不执行。若挂了 OTLP，其缓冲会在退出前自动刷新；其他自定义上报请在 `Config.OnError`/关闭流程中自行处理。

**Q：怎么关闭 caller（调用位置）？**
`Config.Caller = false`。规则：**一个文件路径+行号**，不是调用堆栈（需要堆栈用 `errors` + `ErrorStack`）。

**Q：怎么减少日志量/开销？**
提高 `Level`；关闭 `Caller`；保持 `ParentSpanIDs` 关闭；生产环境用 JSON + 文件输出。

**Q：日志文件在哪里？**
`<FileDir>/<Filename|程序名>.<YYYY-MM-DD>.log`；`MaxAge` 控制保留天数。

**Q：写入磁盘失败怎么发现？**
设置 `Config.OnError`（否则错误写 stderr）。

## 最佳实践

1. **生产用 JSON**：便于采集与字段检索；开发用 text 更易读
2. **字段化而非拼字符串**：`logger.Info(ctx, "order paid", "order_id", id)` 优于 `logger.Infof(ctx, "order %d paid", id)`
3. **始终传 ctx**（请求链路上）：否则丢失 `trace_id` 关联
4. **级别按环境配置**：开发 DEBUG、生产 INFO/WARN
5. **避免敏感信息**：不要记录密码、token、完整身份证号等
6. **启动/后台日志用 `Print`**：语义清晰且不假装有上下文
