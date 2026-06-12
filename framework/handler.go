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

// Handler 业务 handler 签名
type Handler[Req, Res any] func(ctx context.Context, req *Req) (*Res, error)

// Handle 将业务 handler 包装成标准 http.HandlerFunc
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
