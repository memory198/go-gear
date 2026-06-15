# CODEBUDDY.md 本文件为 CodeBuddy 在此代码库中工作提供指导。

## 常用命令

### 构建与测试
- **构建所有包**：`go build ./...`
- **运行所有测试**：`go test ./...`
- **运行单个测试**：`go test ./errors/ -run TestErrorf`（指定包和测试函数）
- **运行测试并显示覆盖率**：`go test -cover ./...`
- **代码检查**：`go vet ./...`
- **格式化代码**：`gofmt -w .` 或 `goimports -w .`
- **清理模块缓存**：`go clean -modcache`
- **整理依赖**：`go mod tidy`
- **下载依赖**：`go mod download`

### 代码检查（如已安装 golangci-lint）
- **运行 linter**：`golangci-lint run`

## 高层架构

本仓库（`github.com/memory198/go-gear`）是一个 Go 工具包，为构建 Web 服务提供核心基础设施。它包含四个主要包：

### 1. `config` - 配置管理
config 包处理 YAML 配置加载，支持：
- **文件合并**：可通过 `include` 指令组合多个 YAML 文件
- **环境变量覆盖**：所有配置值可通过环境变量覆盖（如 `APP_ADDR`、`APP_DB_DRIVER`）
- **热重载**：`Watcher` 监控配置目录变化，并通过通道通知订阅者
- **类型安全访问**：`Value` 类型提供链式方法，如 `.String()`、`.Int()`、`.Bool()`，并支持默认值

关键类型：
- `Config`（全局配置结构，包含 Server、Database、Log 部分）
- `Watcher`（热重载管理器）
- `Value`（类型安全的配置值访问器）

### 2. `errors` - 带堆栈的错误处理
此包包装了标准 `errors` 包，增加了堆栈捕获功能：
- **直接替换**：导入此包代替标准 `errors` 包
- **堆栈追踪**：`Wrap`、`Errorf`、`WithStack` 自动捕获调用堆栈
- **错误链**：完全支持 `errors.Is`、`errors.As`、`errors.Unwrap`
- **堆栈打印**：使用 `fmt.Sprintf("%+v", err)` 打印完整堆栈链

核心函数：`Wrap`、`Wrapf`、`Errorf`、`WithStack`、`From`、`FromMsg`、`Stack`

### 3. `framework` - HTTP 框架
一个轻量级 HTTP 框架，支持泛型处理器和自定义上下文：
- **泛型处理器**：`Handler[Req, Res any]` 函数签名，自动 JSON 绑定/验证
- **自定义上下文**：`ContextHandler[Req, Res any]` 函数签名，使用自定义 `Context` 类型
- **请求绑定**：自动 JSON 请求体解析，支持自定义 `Binder` 接口进行混合绑定
- **验证**：使用 `go-playground/validator`，基于 JSON 标签的字段名
- **统一响应**：标准 JSON 响应格式，包含 `code`、`message`、`data`
- **业务错误**：预定义错误码（`ErrBadRequest`、`ErrUnauthorized` 等），映射 HTTP 状态码
- **中间件**：`framework/middleware` 中包含 Logger 和 Recovery 中间件

关键模式：
- 使用 `framework.Handle(handlerFunc)` 将业务逻辑包装为 `http.HandlerFunc`
- 使用 `framework.HandleContext(handlerFunc)` 使用自定义上下文包装业务逻辑

自定义上下文功能：
- **请求追踪**：自动生成或从请求头获取 trace ID，支持分布式追踪
- **span管理**：通过 `StartSpan()` 创建子 span，支持分布式追踪链
- **超时控制**：`WithTimeout`、`WithTimeoutFunc`、`WithDeadline` 创建带超时的子上下文
- **取消功能**：`Cancel()` 取消当前上下文及其所有子上下文
- **类型安全值存取**：`WithValue`、`Value` 支持类型安全的上下文值管理
- **响应头设置**：`SetTraceHeaders` 设置追踪相关响应头

### 4. `logger` - 结构化日志
一个支持级别和追踪 ID 的日志记录器，带文件轮转功能：
- **日志级别**：DEBUG、INFO、WARN、ERROR
- **追踪 ID**：自动从上下文中提取追踪 ID 并包含在日志条目中
- **文件输出**：写入文件，支持按天轮转
- **调用者信息**：日志条目包含源文件和行号
- **堆栈追踪**：`ErrorStack` 方法打印错误及其完整堆栈追踪

### 包关系
- `config` 和 `logger` 相互独立，均可单独使用
- `framework` 依赖 `errors` 进行错误处理，依赖 `go-playground/validator` 进行验证
- `framework/middleware` 使用 `framework` 进行错误响应
- 所有包遵循 Go 惯例，设计为可单独导入

### 测试方法
- 测试文件位于源代码旁边的 `*_test.go` 文件中
- `errors` 包包含演示错误包装和堆栈追踪行为的测试
- 使用 `go test ./...` 运行所有测试；每个包可独立测试

### 开发注意事项
- Go 版本：1.23（如 go.mod 中指定）
- 配置使用 YAML 格式，通过 `gopkg.in/yaml.v3` 解析
- 热重载使用 `github.com/fsnotify/fsnotify` 进行文件系统事件监控
- 验证使用 `github.com/go-playground/validator/v10`
- 框架专为 JSON API 设计；对于其他内容类型，请实现自定义 `Binder` 接口