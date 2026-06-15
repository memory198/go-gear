package errors

import (
	"errors"
	"fmt"
)

// withStack 带调用堆栈的 error 核心结构
type withStack struct {
	msg   string  // 本层描述信息，可为空（WithStack 场景）
	cause error   // 被包裹的原始 error
	stack []frame // 调用堆栈
}

func (e *withStack) Error() string {
	if e.cause != nil {
		if e.msg == "" {
			return e.cause.Error() // WithStack：透传原始消息
		}
		return e.msg + ": " + e.cause.Error()
	}
	return e.msg
}

func (e *withStack) Unwrap() error {
	return e.cause
}

// Format 支持 %+v 打印完整堆栈链
func (e *withStack) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			printChain(s, e)
			return
		}
		fmt.Fprint(s, e.Error())
	case 's':
		fmt.Fprint(s, e.Error())
	}
}

// printChain 递归打印整条 error 链，最底层先打印
func printChain(s fmt.State, err error) {
	var we *withStack
	if errors.As(err, &we) {
		if we.cause != nil {
			if _, ok := we.cause.(*withStack); ok {
				printChain(s, we.cause)
			} else {
				fmt.Fprintf(s, "%s\n", we.cause.Error())
			}
		}
		// msg 为空（WithStack）时只打印堆栈，不打印空行
		if we.msg != "" {
			fmt.Fprintf(s, "%s\n", we.msg)
		}
		for _, f := range we.stack {
			fmt.Fprintf(s, "  at %s:%d (%s)\n", f.file, f.line, f.fn)
		}
	} else {
		fmt.Fprintf(s, "%s\n", err.Error())
	}
}
