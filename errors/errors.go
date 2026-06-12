package errors

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
)

// ---- 标准库直接透传，业务代码只需导入本包 ----

var (
	Is     = errors.Is
	As     = errors.As
	Unwrap = errors.Unwrap
	New    = errors.New
)

// ---- 带堆栈的 error ----

// withStack 带调用堆栈的 error
type withStack struct {
	msg   string
	cause error  // 被包裹的原始 error
	stack []frame // 调用堆栈
}

type frame struct {
	file string
	line int
	fn   string
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

// Format 支持 %+v 打印完整堆栈
func (e *withStack) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			// 递归打印 cause 链
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
		// 先递归打印 cause
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

// ---- 公开 API ----

// Wrap 包裹一个 error，附加当前调用位置的堆栈
// 用于每层调用时追加本层的上下文信息
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

// Errorf 创建一个新 error 并附加堆栈（无 cause）
func Errorf(format string, args ...any) error {
	return &withStack{
		msg:   fmt.Sprintf(format, args...),
		stack: callers(1),
	}
}

// WithStack 只在当前位置附加堆栈，不添加任何消息
// 适用于：不需要包裹描述，只是想记录"error 从哪里透传出去的"
// 用法：return errors.WithStack(err)
func WithStack(err error) error {
	if err == nil {
		return nil
	}
	return &withStack{
		cause: err,
		stack: callers(1),
	}
}

// From 将外部 error（标准库、第三方库）转为项目 error，堆栈记录在转换处
// 如果 err 已经是项目 error（withStack），直接返回，不重复包裹
// 用法：err = errors.From(sqlErr)
func From(err error) error {
	if err == nil {
		return nil
	}
	// 已经是项目 error，不重复包裹
	var we *withStack
	if errors.As(err, &we) {
		return err
	}
	return &withStack{
		msg:   err.Error(),
		stack: callers(1),
	}
}

// FromMsg 将外部 error 转为项目 error，并附加额外描述信息
// 用法：err = errors.FromMsg(sqlErr, "query user")
func FromMsg(err error, msg string) error {
	if err == nil {
		return nil
	}
	var we *withStack
	if errors.As(err, &we) {
		// 已是项目 error，直接包裹新信息
		return &withStack{
			msg:   msg,
			cause: err,
			stack: callers(1),
		}
	}
	// 外部 error，转换并附加信息
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

// ---- 内部工具 ----

// callers 获取调用堆栈，skip 表示跳过的帧数（调用者内部帧）
func callers(skip int) []frame {
	const maxDepth = 32
	pcs := make([]uintptr, maxDepth)
	// skip: runtime.Callers(1) 本身 + callers() + Wrap/Errorf = skip+3
	n := runtime.Callers(skip+3, pcs)
	frames := runtime.CallersFrames(pcs[:n])

	var result []frame
	for {
		f, more := frames.Next()
		// 只保留业务代码，过滤 runtime 和标准库
		if !isInternal(f.Function) {
			result = append(result, frame{
				file: shortFile(f.File),
				line: f.Line,
				fn:   shortFunc(f.Function),
			})
		}
		if !more || len(result) >= 8 {
			break
		}
	}
	return result
}

// isInternal 过滤 runtime 和 errors 包自身的帧
func isInternal(fn string) bool {
	return strings.HasPrefix(fn, "runtime.") ||
		strings.Contains(fn, "github.com/memory198/go-gear/errors.")
}

// shortFile 只保留最后两段路径，如 service/user/user.go
func shortFile(file string) string {
	parts := strings.Split(file, "/")
	if len(parts) <= 2 {
		return file
	}
	return strings.Join(parts[len(parts)-2:], "/")
}

// shortFunc 只保留函数名，去掉包路径
func shortFunc(fn string) string {
	if idx := strings.LastIndex(fn, "/"); idx >= 0 {
		fn = fn[idx+1:]
	}
	return fn
}
