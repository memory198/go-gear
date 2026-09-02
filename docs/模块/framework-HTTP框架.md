# framework 模块说明（泛型 HTTP 框架）

> 所属目录：`framework/`
> 架构总览见 [架构设计](../架构设计.md) | 使用见 [使用指南](../使用指南.md)

## 职责

定义业务 Handler 的签名约定，封装"**绑定 → 校验 → 业务 → 统一响应**"的样板流程，输出标准 `http.HandlerFunc`，直接挂到 `net/http`。

## 核心设计

### 1. 两个泛型业务签名

```go
type Handler[Req, Res any]        func(ctx context.Context, req *Req) (*Res, error)
type ContextHandler[Req, Res any] func(ctx *Context, req *Req) (*Res, error)
```

Req/Res 即入参出参模型，编译期类型安全。

### 2. 两个包装函数

| 函数 | 区别 |
|---|---|
| `Handle(h)` | 业务拿到标准 `context.Context`（适合接 OTel 的入口） |
| `HandleContext(h)` | 业务拿到 `*gctx.Context`（自带追踪/超时/存储能力） |

两者内部流程相同：① 绑定请求体 → ② validator 校验 → ③ 调用业务 → ④ 统一响应。

### 3. Context 是 gctx 的类型别名

`type Context = gctx.Context`，不是包装——gctx 的全部能力（TraceID/StartSpan/Set/WithTimeout...）原样可用，framework 只做**再导出**，业务代码无需直接 import gctx。

### 4. 请求绑定（bind.go）

- 默认解析 JSON body（`ContentLength > 0` 时）
- **`Binder` 接口是扩展点**：Req 实现 `Bind(r *http.Request) error` 即可混合绑定 URL 参数 / Query / Body
- 辅助工具：`QueryInt` / `QueryString`（带默认值）

### 5. 参数校验

validator 注册了 **json tag 名称映射**，校验错误提示用 json 字段名，对前端友好。

### 6. 统一响应（response.go）

```json
{ "code": 0, "message": "ok", "data": ... }
```

- `WriteOK(w, data)`：code=0, message="ok"
- `WriteError(w, err)`：用 `errors.As` 识别 `BizError` → 输出其业务码 + HTTP 状态码；**其他任何错误统一 500**（不泄露内部细节）

### 7. BizError 业务错误（error.go）

`{HTTPStatus, Code, Message}` + `WithMsg`（不可变，返回新错误），预定义 6 个：

| 错误 | HTTP | 业务码 | 语义 |
|---|---|---|---|
| `ErrBadRequest` | 400 | 40000 | 请求参数错误 |
| `ErrUnauthorized` | 401 | 40100 | 未登录 |
| `ErrForbidden` | 403 | 40300 | 无权限 |
| `ErrNotFound` | 404 | 40400 | 资源不存在 |
| `ErrConflict` | 409 | 40900 | 资源已存在 |
| `ErrInternal` | 500 | 50000 | 服务器内部错误 |

## 与其他模块的关系

- 依赖 `gctx`（Context 别名）与第三方 `go-playground/validator`
- 与项目 `errors` 包不强制绑定：`BizError` 独立，但可被带堆栈的包装穿透（`errors.As`）
- 被 `framework/middleware` 依赖（Recoverer 调用 `framework.WriteError`）

## 边界 / 刻意不做

- **不做路由**：`Handle/HandleContext` 返回标准 `http.HandlerFunc`，方法/路径匹配交给标准库或将来的路由（指南推荐 chi）
- **不提供中间件链注册**：middleware 是"函数包函数"嵌套风格，由调用方组合
- 绑定只覆盖 JSON body，其他内容类型需自行实现 `Binder`

## 相关文档

- [架构设计](../架构设计.md)：§六 请求生命周期
- [使用指南](../使用指南.md)：二、Framework
- [gctx-请求上下文](gctx-请求上下文.md)：Context 的实现
- [middleware-中间件](middleware-中间件.md)：外层横切
