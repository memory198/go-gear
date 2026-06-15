package errors

import (
	"runtime"
	"strings"
)

// frame 单个调用帧信息
type frame struct {
	file string
	line int
	fn   string
}

// callers 获取当前调用堆栈
// skip：额外跳过的帧数，调用方传 1 表示跳过自身
func callers(skip int) []frame {
	const maxDepth = 32
	pcs := make([]uintptr, maxDepth)
	// +3：runtime.Callers 自身 + callers() + 公开 API 函数（Wrap/Errorf 等）
	n := runtime.Callers(skip+3, pcs)
	frames := runtime.CallersFrames(pcs[:n])

	var result []frame
	for {
		f, more := frames.Next()
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

// shortFunc 只保留函数名，去掉完整包路径
func shortFunc(fn string) string {
	if idx := strings.LastIndex(fn, "/"); idx >= 0 {
		fn = fn[idx+1:]
	}
	return fn
}
