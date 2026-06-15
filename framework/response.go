package framework

import (
	"encoding/json"
	"errors"
	"net/http"
)

// Response 统一 JSON 响应结构
// 所有 API 响应都使用此格式，确保一致性
type Response struct {
	Code    int    `json:"code"`           // 业务状态码，0 表示成功
	Message string `json:"message"`        // 响应消息
	Data    any    `json:"data,omitempty"` // 响应数据，可选
}

// WriteOK 写入成功响应
// 自动设置 code=0, message="ok"
// 用法：framework.WriteOK(w, userData)
func WriteOK(w http.ResponseWriter, data any) {
	writeJSON(w, http.StatusOK, Response{
		Code:    0,
		Message: "ok",
		Data:    data,
	})
}

// WriteError 写入错误响应
// 自动识别 BizError 类型，设置对应的 HTTP 状态码和业务错误码
// 非 BizError 统一返回 500 内部错误
// 用法：framework.WriteError(w, framework.ErrNotFound.WithMsg("用户不存在"))
func WriteError(w http.ResponseWriter, err error) {
	var bizErr *BizError
	if errors.As(err, &bizErr) {
		writeJSON(w, bizErr.HTTPStatus, Response{
			Code:    bizErr.Code,
			Message: bizErr.Message,
		})
		return
	}
	writeJSON(w, http.StatusInternalServerError, Response{
		Code:    ErrInternal.Code,
		Message: ErrInternal.Message,
	})
}

// writeJSON 内部函数，写入 JSON 响应
// 设置 Content-Type 头并编码 JSON 数据
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
