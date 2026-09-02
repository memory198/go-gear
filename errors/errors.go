// Package errors 提供带堆栈信息的 error 封装
// 兼容标准库 errors，直接替换导入即可使用
package errors

import "errors"

// 标准库透传，业务代码只需导入本包
var (
	Is     = errors.Is
	As     = errors.As
	Unwrap = errors.Unwrap
	New    = errors.New
)
