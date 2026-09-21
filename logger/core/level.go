// Package core 定义日志数据契约（Level / Record / Attr / Resource / Hook），
// 供 logger 主包与各类导出实现（如 otlpsink）共享，自身仅依赖标准库。
package core

import (
	"strconv"
	"strings"
)

// Level 日志级别
// 数值对齐 OTel SeverityNumber 代表值，便于 OTLP 导出与跨系统一致
type Level int

const (
	DEBUG Level = 5  // OTel SeverityDebug
	INFO  Level = 9  // OTel SeverityInfo
	WARN  Level = 13 // OTel SeverityWarn
	ERROR Level = 17 // OTel SeverityError
	FATAL Level = 21 // OTel SeverityFatal
)

// levelNames 可读级别名（text 输出与 severity_text 使用）
var levelNames = map[Level]string{
	DEBUG: "DEBUG",
	INFO:  "INFO",
	WARN:  "WARN",
	ERROR: "ERROR",
	FATAL: "FATAL",
}

// String 返回可读级别名；未知级别返回 "LEVEL(n)"
func (l Level) String() string {
	if s, ok := levelNames[l]; ok {
		return s
	}
	return "LEVEL(" + strconv.Itoa(int(l)) + ")"
}

// ParseLevel 将级别名解析为 Level（不区分大小写，未知值返回 INFO）
func ParseLevel(s string) Level {
	switch strings.ToLower(s) {
	case "debug":
		return DEBUG
	case "warn":
		return WARN
	case "error":
		return ERROR
	case "fatal":
		return FATAL
	default:
		return INFO
	}
}
