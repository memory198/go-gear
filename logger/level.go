package logger

import "github.com/memory198/go-gear/logger/core"

// Level 日志级别（类型别名，定义在 core 包，数值对齐 OTel SeverityNumber）
type Level = core.Level

// 预定义级别
const (
	DEBUG = core.DEBUG
	INFO  = core.INFO
	WARN  = core.WARN
	ERROR = core.ERROR
	FATAL = core.FATAL
)

// ParseLevel 将级别名解析为 Level（不区分大小写，未知值返回 INFO）
// 便于把配置项（如 config 包的 Log.Level 字符串）转为日志级别
func ParseLevel(s string) Level { return core.ParseLevel(s) }
