# chi 路由接入指南

> go-gear 不内置路由层，**推荐搭配 [chi](https://github.com/go-chi/chi)（v5）使用**。
> 本指南讲解如何把 chi 作为路由层与 go-gear 组合，核心结论：**零适配代码**——
> `framework.Handle` 返回标准 `http.HandlerFunc`，go-gear 中间件签名 `func(http.Handler) http.Handler` 与 `chi.Use` 完全一致。

## 为什么选 chi

| 需求 | net/http 标准库 | chi |
|---|---|---|
| 方法路由（POST/GET 分开注册） | Go 1.22+ 才支持 `"POST /path"` 模式 | ✅ 一直支持 |
| 路径参数 `/users/{id}` | Go 1.22+ 支持，取值繁琐 | ✅ `chi.URLParam(r, "id")` |
| 路由分组 / 嵌套 | ❌ | ✅ `r.Route("/api", ...)` |
| 中间件 | 手动包装 | ✅ `r.Use(...)`，签名与 go-gear 一致 |
| 体积 | — | 轻量，只依赖 net/http |

## 安装

```bash
go get github.com/go-chi/chi/v5
```

> chi 是**使用方依赖**，go-gear 本身不 import chi（保持零强制依赖）。你的 go.mod 需要它，go-gear 不需要。

## 最小接入

```go
package main

import (
    "context"
    "net/http"

    "github.com/go-chi/chi/v5"
    "github.com/memory198/go-gear/framework"
    "github.com/memory198/go-gear/framework/middleware"
)

func main() {
    r := chi.NewRouter()
    r.Use(middleware.Recoverer) // go-gear 中间件直接挂 chi

    r.Post("/users", framework.Handle(createUser))

    http.ListenAndServe(":8080", r)
}

type User struct{ ID, Name string }

type CreateUserReq struct {
    Name string `json:"name" validate:"required"`
}

func createUser(ctx context.Context, req *CreateUserReq) (*User, error) {
    return &User{ID: "1", Name: req.Name}, nil
}
```

## 路由能力速览

```go
r := chi.NewRouter()

r.Use(middleware.Recoverer)              // go-gear 中间件（顺序见下节）

r.Get("/health", healthHandler)          // 普通 handler
r.Post("/users", framework.Handle(createUser))          // go-gear 泛型包装
r.Get("/users/{id}", framework.Handle(getUser))         // 路径参数

r.Route("/api", func(r chi.Router) {     // 分组
    r.Route("/v1", func(r chi.Router) {
        r.Put("/users/{id}", framework.Handle(updateUser))
    })
})

r.NotFound(func(w http.ResponseWriter, _ *http.Request) {
    framework.WriteError(w, framework.ErrNotFound)
})
```

## 参数绑定：三种来源如何进 `Req`

`framework.Bind` 的规则：**Req 实现了 `framework.Binder` 就调你的 `Bind`，否则只解析 JSON body**。所以混合绑定的写法是：Req 实现 Binder，在 `Bind` 里手动取路径/Query 参数，再解析 body。

### 路径参数（chi.URLParam + Binder）

```go
type GetUserReq struct {
    ID string `json:"-"`
}

// 实现 framework.Binder：从 chi 路由取 {id}
func (req *GetUserReq) Bind(r *http.Request) error {
    req.ID = chi.URLParam(r, "id") // chi 已把参数放进路由上下文
    return nil
}

func getUser(ctx context.Context, req *GetUserReq) (*User, error) {
    if req.ID == "" {
        return nil, framework.ErrBadRequest.WithMsg("id 不能为空")
    }
    return &User{ID: req.ID, Name: "alice"}, nil
}

// 注册：chi 路由解析 /users/{id}，Bind 时通过 chi.URLParam 取到
r.Get("/users/{id}", framework.Handle(getUser))
```

### Query 参数

```go
// 方式一：内置工具（带默认值）
limit := framework.QueryInt(r, "limit", 20)

// 方式二：Req 实现 Binder 手动取（适合组合进 Req 结构）
func (req *ListUsersReq) Bind(r *http.Request) error {
    req.Page = framework.QueryInt(r, "page", 1)
    return nil
}
```

### JSON body（默认路径，无需额外代码）

不实现 `Binder` 时，`Bind` 自动解析 JSON body 到 Req；`ContentLength == 0`（GET 等无 body 请求）则跳过。

> 注意：实现了 `Binder` 后 body 解析不再自动执行，需要在 `Bind` 里自行 `json.NewDecoder(r.Body).Decode(req)`（见下面完整示例的 updateUser）。

## 中间件配合与顺序

go-gear 的中间件就是 `func(http.Handler) http.Handler`，与 chi 原生中间件同签名，**混用无差别**：

```go
r.Use(middleware.Logger)                      // 1. 访问日志（最外层）
r.Use(middleware.OTel("my-service"))          // 2. OTel 追踪（若有）
r.Use(middleware.Recoverer)                   // 3. panic 恢复（尽量靠内，兜住所有下层）
// r.Use(chiMiddleware.Timeout(5 * time.Second)) // chi 生态中间件同样可用
```

顺序规则：**Use 注册在路由声明之前，按声明顺序从外到内包裹**。Recoverer 放最后 Use 即最内层，能捕获全部后续 panic。

## Handle / HandleContext 怎么选

| 场景 | 用哪个 | 原因 |
|---|---|---|
| 接 OTel（middleware.OTel） | `Handle` | trace ID 来自 OTel，标准 context 即可 |
| 用 gctx 内置追踪（不接 OTel） | `HandleContext` | 业务直接拿到 `*gctx.Context`，自带 TraceID/StartSpan/Set |
| 需要 gctx 的存储/超时能力 | `HandleContext` | `ctx.Set` / `ctx.WithTimeout` |

两种都返回 `http.HandlerFunc`，chi 路由注册方式完全相同。

## 完整示例（含路径参数 + body 混合绑定）

```go
package main

import (
    "context"
    "encoding/json"
    "net/http"

    "github.com/go-chi/chi/v5"
    "github.com/memory198/go-gear/framework"
    "github.com/memory198/go-gear/framework/middleware"
    "github.com/memory198/go-gear/logger"
)

type User struct {
    ID   string `json:"id"`
    Name string `json:"name"`
}

// POST /users —— 纯 body 绑定，走默认路径
type CreateUserReq struct {
    Name string `json:"name" validate:"required"`
}

func createUser(ctx context.Context, req *CreateUserReq) (*User, error) {
    logger.Info(ctx, "creating user")
    return &User{ID: "1", Name: req.Name}, nil
}

// GET /users/{id} —— 路径参数，实现 Binder
type GetUserReq struct {
    ID string
}

func (req *GetUserReq) Bind(r *http.Request) error {
    req.ID = chi.URLParam(r, "id")
    return nil
}

func getUser(ctx context.Context, req *GetUserReq) (*User, error) {
    if req.ID == "" {
        return nil, framework.ErrBadRequest.WithMsg("id 不能为空")
    }
    return &User{ID: req.ID, Name: "alice"}, nil
}

// PUT /users/{id} —— 路径参数 + body 混合：Binder 里手动解析
type UpdateUserReq struct {
    ID   string
    Name string `json:"name" validate:"required"`
}

func (req *UpdateUserReq) Bind(r *http.Request) error {
    req.ID = chi.URLParam(r, "id")
    return json.NewDecoder(r.Body).Decode(req) // 实现 Binder 后需自行解析 body
}

func updateUser(ctx context.Context, req *UpdateUserReq) (*User, error) {
    logger.Infof(ctx, "updating user %s", req.ID)
    return &User{ID: req.ID, Name: req.Name}, nil
}

func main() {
    r := chi.NewRouter()
    r.Use(middleware.Logger)
    r.Use(middleware.Recoverer)

    r.Post("/users", framework.Handle(createUser))
    r.Get("/users/{id}", framework.Handle(getUser))
    r.Put("/users/{id}", framework.Handle(updateUser))

    http.ListenAndServe(":8080", r)
}
```

## 常见问题

1. **Binder 里为什么拿不到路径参数？** 确认该 handler 是经 chi 注册的（`r.Get("/users/{id}", ...)`），`chi.URLParam` 依赖 chi 的路由上下文；在 handler 内调用时一定可用。
2. **实现了 Binder 后 body 不解析了？** 这是设计行为——实现 Binder 代表"绑定方式全权交给你"，需在 `Bind` 里自行 `json.NewDecoder(r.Body).Decode(req)`。
3. **404 返回什么格式？** 默认 chi 返回纯文本。若想统一 JSON 响应，用 `r.NotFound` 配合 `framework.WriteError(w, framework.ErrNotFound)`（见路由速览示例）。
4. **中间件顺序反了会怎样？** Recoverer 若在 Logger 外层，panic 的请求也会先打访问日志但状态码可能为 200（Logger 的 wrappedWriter 没收到 WriteHeader）。推荐顺序：Logger → OTel → Recoverer。

## 相关文档

- [使用指南](使用指南.md)：二、Framework；三、中间件
- [架构设计](架构设计.md)：§六 请求生命周期
- [框架模块说明](模块/framework-HTTP框架.md)：Binder / Handle / HandleContext
- [OTel接入指南](OTel接入指南.md)：接入 OTel 时用 Handle
