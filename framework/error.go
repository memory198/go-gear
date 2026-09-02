package framework

import "fmt"

// BizError 业务错误结构体
// 包含 HTTP 状态码、业务错误码和错误消息
// 用于框架统一错误响应处理
type BizError struct {
	HTTPStatus int    // HTTP 状态码
	Code       int    // 业务错误码
	Message    string // 错误消息
}

// Error 实现 error 接口
func (e *BizError) Error() string {
	return fmt.Sprintf("code=%d, message=%s", e.Code, e.Message)
}

// WithMsg 基于当前错误创建一个带自定义消息的新错误
// 保留原错误的 HTTP 状态码和业务错误码，只替换消息
// 用法：framework.ErrBadRequest.WithMsg("用户名不能为空")
func (e *BizError) WithMsg(msg string) *BizError {
	return &BizError{
		HTTPStatus: e.HTTPStatus,
		Code:       e.Code,
		Message:    msg,
	}
}

// 预定义错误码
var (
	ErrBadRequest   = &BizError{HTTPStatus: 400, Code: 40000, Message: "请求参数错误"}
	ErrUnauthorized = &BizError{HTTPStatus: 401, Code: 40100, Message: "未登录"}
	ErrForbidden    = &BizError{HTTPStatus: 403, Code: 40300, Message: "无权限"}
	ErrNotFound     = &BizError{HTTPStatus: 404, Code: 40400, Message: "资源不存在"}
	ErrConflict     = &BizError{HTTPStatus: 409, Code: 40900, Message: "资源已存在"}
	ErrInternal     = &BizError{HTTPStatus: 500, Code: 50000, Message: "服务器内部错误"}
)
