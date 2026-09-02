// Package framework 提供轻量级 HTTP 框架功能
// 包括泛型处理器、请求绑定、验证、统一响应和业务错误处理
package framework

import (
	"context"
	"net/http"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
)

var validate *validator.Validate

func init() {
	validate = validator.New()
	// 使用 json tag 作为字段名，错误提示更友好
	validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})
}

// Handler 业务处理器函数签名
// 泛型参数：
//   - Req: 请求参数类型
//   - Res: 响应数据类型
//
// 函数接收 context 和请求指针，返回响应指针和错误
type Handler[Req, Res any] func(ctx context.Context, req *Req) (*Res, error)

// ContextHandler 带自定义上下文的业务处理器函数签名
// 泛型参数：
//   - Req: 请求参数类型
//   - Res: 响应数据类型
//
// 函数接收自定义上下文和请求指针，返回响应指针和错误
type ContextHandler[Req, Res any] func(ctx *Context, req *Req) (*Res, error)

// Handle 将业务处理器包装为标准 http.HandlerFunc
// 自动处理以下流程：
// 1. 绑定 JSON 请求体到 Req 结构体
// 2. 使用 validator 进行参数验证
// 3. 调用业务处理器
// 4. 统一处理错误响应或成功响应
//
// 用法：
//
//	http.HandleFunc("/api", framework.Handle(myHandler))
func Handle[Req, Res any](h Handler[Req, Res]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req Req

		// 1. 绑定请求参数
		if err := Bind(r, &req); err != nil {
			WriteError(w, ErrBadRequest.WithMsg(err.Error()))
			return
		}

		// 2. 参数校验
		if err := validate.Struct(&req); err != nil {
			WriteError(w, ErrBadRequest.WithMsg(err.Error()))
			return
		}

		// 3. 调用业务逻辑
		res, err := h(r.Context(), &req)
		if err != nil {
			WriteError(w, err)
			return
		}

		// 4. 写响应
		WriteOK(w, res)
	}
}

// HandleContext 将带自定义上下文的业务处理器包装为标准 http.HandlerFunc
// 自动处理以下流程：
// 1. 创建自定义上下文
// 2. 绑定 JSON 请求体到 Req 结构体
// 3. 使用 validator 进行参数验证
// 4. 调用业务处理器
// 5. 统一处理错误响应或成功响应
//
// 用法：
//
//	http.HandleFunc("/api", framework.HandleContext(myHandler))
func HandleContext[Req, Res any](h ContextHandler[Req, Res]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. 创建自定义上下文
		ctx := NewContext(r, w)

		var req Req

		// 2. 绑定请求参数
		if err := Bind(r, &req); err != nil {
			WriteError(w, ErrBadRequest.WithMsg(err.Error()))
			return
		}

		// 3. 参数校验
		if err := validate.Struct(&req); err != nil {
			WriteError(w, ErrBadRequest.WithMsg(err.Error()))
			return
		}

		// 4. 调用业务逻辑
		res, err := h(ctx, &req)
		if err != nil {
			WriteError(w, err)
			return
		}

		// 5. 写响应
		WriteOK(w, res)
	}
}
