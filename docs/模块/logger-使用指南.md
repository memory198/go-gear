# logger 使用指南

> 本文面向**使用者**，只讲"怎么用好"，不涉及内部实现。
> 设计原理与包结构见 [logger-结构化日志](logger-结构化日志.md)。

## 一、快速开始

```go
import (
    "context"
    "github.com/memory198/go-gear/logger"
)

func main() {
    ctx := context.Background()
    logger.Info(ctx, "service started", "port", 8080)   // 开箱即用
}
```

包级方法直接可用（内部默认实例：控制台输出、text 格式、DEBUG 级别）。输出示例：

```
2026-09-22T10:30:00.123456+08:00 [INFO] main.go:12 service started port=8080
```

### 带 ctx 与不带 ctx 怎么选

| 场景 | 用法 | 说明 |
|------|------|------|
| 请求处理、有 trace 上下文 | `logger.Info(ctx, ...)` | 自动带上 `trace_id` / `span_id` |
| 启动、后台任务、无上下文 | `logger.Print(...)` / `logger.Printf(...)` | INFO 级，不带 trace 字段 |

```go
// 请求内（推荐：始终传递 ctx）
func handler(ctx context.Context) {
    logger.Info(ctx, "handling request", "user_id", 123)
}

// 启动 / 后台（没有 ctx 可用）
logger.Print("server starting", "port", 8080)
logger.Printf("listening on %s:%d", "0.0.0.0", 8080)
```

## 二、日志级别

| 级别 | 方法 | 说明 |
|------|------|------|
| DEBUG | `logger.Debug(ctx, msg, kv...)` | 调试细节 |
| INFO | `logger.Info(ctx, msg, kv...)` | 常规信息 |
| WARN | `logger.Warn(ctx, msg, kv...)` | 警告 |
| ERROR | `logger.Error(ctx, msg, kv...)` | 错误 |
| FATAL | `logger.Fatal(ctx, msg, kv...)` | **输出后 `os.Exit(1)` 退出进程** |

低于配置级别（`Config.Level`）的日志会被丢弃，例如配置 `Level: logger.WARN` 时 `Info` 不输出。

> ⚠️ `Fatal` 会直接退出进程：`defer` 不会执行、已注册的上报缓冲会被自动刷新（见 §十）。生产代码慎用，服务内部错误建议用 `Error`。

## 三、携带字段（键值对）

`msg` 之后可跟任意多个 `key, value` 对（slog 语义）：

```go
logger.Info(ctx, "user created", "user_id", 123, "source", "api", "duration_ms", 12.5)
```

输出效果：

```
# text：以 key=value 追加在正文后
... [INFO] handler/user.go:42 user created user_id=123 source=api duration_ms=12.5

# json：归入 attributes 对象
{"body":"user created", ..., "attributes":{"user_id":123,"source":"api","duration_ms":12.5}}
```

规则说明：

- **与内置字段重名的 key 会被丢弃**（内置字段：`timestamp`、`severity_number`、`severity_text`、`body`、`code.filepath`、`code.lineno`、`trace_id`、`span_id`、`parent_span_ids`、`resource` 及 resource 下的键）
- **重复 key 不去重**，输出多次（消费端通常以后一个为准）
- key 不是字符串或为空 → 记为 `!BADKEY`；参数个数为奇数时，末尾裸值也记为 `!BADKEY`

## 四、格式化输出

需要拼接字符串时用 `Xxxf`（等价于 `fmt.Sprintf` 后再打日志）：

```go
logger.Infof(ctx, "user %s login, age=%d", "alice", 25)   // 格式化到正文
logger.Info(ctx, "user login", "name", "alice", "age", 25) // 推荐：字段化，便于检索
```

无 ctx 版本：`logger.Printf("...%s...", v)`。

> `Print` 与 `Printf` 的参数语义不同：`Print(msg, kv...)` 是字段，`Printf(format, args...)` 是格式化。

## 五、自定义实例与全局设置

需要自定义输出/级别时创建实例并替换默认：

```go
l, err := logger.New(logger.Config{
    Level:   logger.INFO,
    Format:  logger.JSONFormat,
    Console: true,
    FileDir: "/var/log/app",
    MaxAge:  30,
})
if err != nil {
    panic(err)
}
logger.SetDefault(l)
defer logger.Close()

// 之后所有包级调用（logger.Info 等）都走这个实例
logger.Info(context.Background(), "configured")
```

也可以持有实例单独使用（多实例互不影响）：

```go
auditLog, _ := logger.New(logger.Config{FileDir: "/var/log/audit", Filename: "audit"})
auditLog.Info(ctx, "user login", "user_id", 123)
```

## 六、配置项速查

### Config 全部字段

| 字段 | 类型 | 说明 | 默认 |
|------|------|------|------|
| `Level` | `Level` | 最低输出级别 | 默认实例 DEBUG |
| `Format` | `Format` | `TextFormat` / `JSONFormat` | TextFormat |
| `Console` | `bool` | 输出到控制台 | false（无其他输出时兜底 stdout） |
| `FileDir` | `string` | 日志目录，空则不写文件 | "" |
| `Filename` | `string` | 文件名前缀（不含扩展名） | 程序名 |
| `MaxAge` | `int` | 日志保留天数，`<=0` 不清理 | 0 |
| `Caller` | `bool` | 是否记录一行调用位置 | false（默认实例 true） |
| `ParentSpanIDs` | `bool` | 是否输出中间 span 链（开启有聚合开销） | false |
| `Service` | `string` | resource: `service.name` | "" |
| `Version` | `string` | resource: `service.version` | "" |
| `Env` | `string` | resource: `deployment.environment` | "" |
| `Host` | `string` | resource: `host.name` | 本机主机名 |
| `OnError` | `func(error)` | 写入/滚动失败回调（磁盘满等问题排查） | nil（写 stderr） |

### 通过 config 包（yaml + 环境变量）

```yaml
log:
  level: info          # debug / info / warn / error / fatal
  format: json
  console: true
  dir: /var/log/app
  filename: app
  max_age: 30
  caller: true
  parent_span_ids: false
  service: user-api
  version: "1.2.0"
  env: prod
  host: node-1
```

```go
cfg, err := config.Load("config.yaml")
if err != nil {
    panic(err)
}
l, _ := logger.New(logger.Config{
    Level:   logger.ParseLevel(cfg.Log.Level),   // "debug"/"info"/... → Level
    Format:  logger.ParseFormat(cfg.Log.Format), // "json" → JSONFormat，其余 → TextFormat
    Console: cfg.Log.Console,
    FileDir: cfg.Log.Dir,
    Filename: cfg.Log.Filename,
    MaxAge:  cfg.Log.MaxAge,
    Caller:  cfg.Log.Caller,
    ParentSpanIDs: cfg.Log.ParentSpanIDs,
    Service: cfg.Log.Service,
    Version: cfg.Log.Version,
    Env:     cfg.Log.Env,
    Host:    cfg.Log.Host,
})
logger.SetDefault(l)
```

对应环境变量：`APP_LOG_LEVEL`、`APP_LOG_FORMAT`、`APP_LOG_CONSOLE`、`APP_LOG_DIR`、`APP_LOG_FILENAME`、`APP_LOG_MAX_AGE`、`APP_LOG_CALLER`、`APP_LOG_PARENT_SPAN_IDS`、`APP_LOG_SERVICE`、`APP_LOG_VERSION`、`APP_LOG_ENV`、`APP_LOG_HOST`（环境变量优先于配置文件）。

## 七、输出目标与文件滚动

```go
// 仅控制台
logger.New(logger.Config{Console: true})

// 仅文件（按天滚动：app.2026-09-22.log）
logger.New(logger.Config{FileDir: "/var/log/app", Filename: "app"})

// 控制台 + 文件同时
logger.New(logger.Config{Console: true, FileDir: "/var/log/app", MaxAge: 7})
```

- 文件名规则：`<Filename>.<YYYY-MM-DD>.log`（`Filename` 为空则取程序名）
- `MaxAge` 自动清理过期文件（仅清理本实例前缀的文件，不会误删其他实例）

## 八、链路字段（trace_id / span_id）

以下任一方式都能让日志自动带链路字段，**业务代码无需手动处理**：

| 方式 | 使用 | 得到 |
|------|------|------|
| gctx（内置轻量追踪） | `framework.HandleContext` 或直接用 `gctx.NewContext` | `trace_id` / `span_id`（+ `parent_span_ids` 可选） |
| OTel 中间件 | 挂 `middleware.OTel("service")` | 来自 OTel span 的 `trace_id` / `span_id` |

```go
// 用 gctx 时
func createUser(ctx *framework.Context, req *Req) (*Res, error) {
    logger.Info(ctx, "creating user")   // 自动带 trace_id/span_id
    sub := ctx.StartSpan()             // 开启子 span
    logger.Info(sub, "calling repo")   // span_id 变为子 span
    ...
}

// 接 OTel 时（middleware.OTel 已注入 span 上下文）
func createUser(ctx context.Context, req *Req) (*Res, error) {
    logger.Info(ctx, "creating user")   // 自动带 OTel trace/span
    ...
}
```

说明：
- `parent_span_ids` 默认关闭，需要时 `Config.ParentSpanIDs = true`
- 上游传入 W3C `traceparent` 时会自动沿用（跨服务串联）

## 九、错误与堆栈

配合 `errors` 包记录完整堆栈：

```go
import (
    "github.com/memory198/go-gear/errors"
    "github.com/memory198/go-gear/logger"
)

if err := repo.Create(ctx, req); err != nil {
    wrapped := errors.Wrap(err, "create user failed")
    logger.ErrorStack(ctx, wrapped)   // 输出错误链与堆栈（等价 printf("%+v")）
    return nil, framework.ErrInternal
}
```

也可直接打印：

```go
logger.Error(ctx, "db error", "err", err)   // 仅错误消息作为字段
```

## 十、上报到 OTel Collector（可选）

希望日志与 trace 在同一个后端（Loki/Tempo）互跳时，挂载 OTLP 导出：

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

- 落盘输出不受影响（OTLP 是旁路）
- `logger.Fatal` 退出前会自动刷新缓冲；正常退出请 `defer shutdown(ctx)`
- 完整接入步骤见 [OTel接入指南](../OTel接入指南.md)

## 十一、常见场景

```go
// HTTP handler（框架包装的泛型 handler）
func createUser(ctx context.Context, req *CreateUserReq) (*User, error) {
    logger.Info(ctx, "creating user", "name", req.Name)
    u, err := svc.Create(ctx, req)
    if err != nil {
        logger.ErrorStack(ctx, err)
        return nil, framework.ErrInternal
    }
    return u, nil
}

// 启动 / 关闭
func main() {
    logger.Print("server starting", "addr", ":8080")
    ...
    logger.Print("server stopped")
}

// 后台任务（无请求 ctx）
func worker() {
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    logger.Info(ctx, "job started", "job", "cleanup")
}
```

## 十二、FAQ

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

## 十三、最佳实践

1. **生产用 JSON**：便于采集与字段检索；开发用 text 更易读
2. **字段化而非拼字符串**：`logger.Info(ctx, "order paid", "order_id", id)` 优于 `logger.Infof(ctx, "order %d paid", id)`
3. **始终传 ctx**（请求链路上）：否则丢失 `trace_id` 关联
4. **级别按环境配置**：开发 DEBUG、生产 INFO/WARN
5. **避免敏感信息**：不要记录密码、token、完整身份证号等
6. **启动/后台日志用 `Print`**：语义清晰且不假装有上下文

## 相关文档

- [logger-结构化日志](logger-结构化日志.md)：模块设计与包结构（进阶）
- [使用指南](../使用指南.md)：一、Logger（快速上手）
- [OTel接入指南](../OTel接入指南.md)：日志 OTLP 导出与 trace 关联
- [gctx-请求上下文](gctx-请求上下文.md)：trace/span 的来源与用法
