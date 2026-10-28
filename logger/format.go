package logger

import "github.com/memory198/go-gear/logger/core"

// Format 日志输出格式
type Format int

const (
	TextFormat Format = iota // 非结构化文本，人读优先
	JSONFormat               // 结构化 JSON，字段对齐 OTel Logs Data Model
)

// ParseFormat 将格式名解析为 Format（"json" → JSONFormat，其余 → TextFormat）
// 便于把配置项（如 config 包的 Log.Format 字符串）转为输出格式
func ParseFormat(s string) Format {
	switch s {
	case "json":
		return JSONFormat
	default:
		return TextFormat
	}
}

// 数据契约（Record/Attr/Resource/Emitter/HookFunc/Flusher）
// 直接使用 logger/core 包的类型，不在本包提供别名

// reservedKeys 内置字段名集合
// 自定义 kv 与内置字段重名时跳过，防止覆盖内置字段导致类型/结构异常
var reservedKeys = map[string]bool{
	"timestamp":              true,
	"severity_number":        true,
	"severity_text":          true,
	"body":                   true,
	"code.filepath":          true,
	"code.lineno":            true,
	"trace_id":               true,
	"span_id":                true,
	"parent_span_ids":        true,
	"resource":               true,
	"service.name":           true,
	"service.version":        true,
	"deployment.environment": true,
	"host.name":              true,
}

// parseArgs 将 kv args 解析为有序属性列表（slog 语义）
// key 必须为非空 string，否则记为 !BADKEY；奇数个参数时末尾裸值记为 !BADKEY
// 与内置字段重名的 kv 直接丢弃
// 注意：重复 key 不做去重（与 slog/zap 一致），消费端通常以后一个值为准
func parseArgs(args []any) []core.Attr {
	if len(args) == 0 {
		return nil
	}
	attrs := make([]core.Attr, 0, (len(args)+1)/2)
	for i := 0; i+1 < len(args); i += 2 {
		key, ok := args[i].(string)
		if !ok || key == "" {
			key = "!BADKEY"
		}
		if reservedKeys[key] {
			continue // 内置字段优先，丢弃同名的自定义字段
		}
		attrs = append(attrs, core.Attr{Key: key, Value: args[i+1]})
	}
	if len(args)%2 == 1 {
		attrs = append(attrs, core.Attr{Key: "!BADKEY", Value: args[len(args)-1]})
	}
	return attrs
}
