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

// parseLevel 解析级别名（内部转发到 core.ParseLevel）
func parseLevel(s string) Level { return core.ParseLevel(s) }
