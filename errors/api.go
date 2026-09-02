package errors

import (
	"errors"
	"fmt"
)

// Errorf 创建一个新 error 并记录当前堆栈（无 cause）
func Errorf(format string, args ...any) error {
	return &withStack{
		msg:   fmt.Sprintf(format, args...),
		stack: callers(1),
	}
}

// Wrap 包裹 error 并附加本层描述，记录当前堆栈
func Wrap(err error, msg string) error {
	if err == nil {
		return nil
	}
	return &withStack{
		msg:   msg,
		cause: err,
		stack: callers(1),
	}
}

// Wrapf 带格式化消息的 Wrap
func Wrapf(err error, format string, args ...any) error {
	if err == nil {
		return nil
	}
	return &withStack{
		msg:   fmt.Sprintf(format, args...),
		cause: err,
		stack: callers(1),
	}
}

// WithStack 只记录当前堆栈，不添加任何描述
// 适用于：不需要包裹描述，只想标记 error 从哪里透传
func WithStack(err error) error {
	if err == nil {
		return nil
	}
	return &withStack{
		cause: err,
		stack: callers(1),
	}
}

// From 将外部 error 转为项目 error，堆栈记录在转换处
// 如果已是项目 error 则直接返回，不重复包裹
func From(err error) error {
	if err == nil {
		return nil
	}
	var we *withStack
	if errors.As(err, &we) {
		return err
	}
	return &withStack{
		msg:   err.Error(),
		stack: callers(1),
	}
}

// FromMsg 将外部 error 转为项目 error，并附加描述信息
func FromMsg(err error, msg string) error {
	if err == nil {
		return nil
	}
	return &withStack{
		msg:   msg,
		cause: err,
		stack: callers(1),
	}
}

// Stack 获取 error 的完整堆栈字符串（等价于 fmt.Sprintf("%+v", err)）
func Stack(err error) string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf("%+v", err)
}
