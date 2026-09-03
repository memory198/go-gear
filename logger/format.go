package logger

import "encoding/json"

// Format 日志输出格式
type Format int

const (
	TextFormat Format = iota // 非结构化文本，仅展示 root_trace_id
	JSONFormat               // 结构化 JSON，携带完整链路字段
)

func parseFormat(s string) Format {
	switch s {
	case "json":
		return JSONFormat
	default:
		return TextFormat
	}
}

// entry 一条日志的结构化表示
// textEncoder 和 jsonEncoder 共用此结构体组装数据
type entry struct {
	Time          string   `json:"time"`
	Level         string   `json:"level"`
	Msg           string   `json:"msg"`
	Caller        string   `json:"caller,omitempty"`
	RootTraceID   string   `json:"root_trace_id,omitempty"`
	MiddleSpanIDs []string `json:"middle_span_ids,omitempty"`
	CurrentSpanID string   `json:"current_span_id,omitempty"`

	fields []field // kv args 解析出的有序键值对，不参与默认 json tag 序列化
}

// field 单个键值对字段（slog 语义）
type field struct {
	key   string
	value any
}

// MarshalJSON 将 entry 与 kv 字段平铺为单个 JSON 对象
// kv 的 key 与内置字段同名时忽略（避免覆盖 time/level/msg 等结构字段）
func (e *entry) MarshalJSON() ([]byte, error) {
	m := make(map[string]any, 8+len(e.fields))
	m["time"] = e.Time
	m["level"] = e.Level
	m["msg"] = e.Msg
	if e.Caller != "" {
		m["caller"] = e.Caller
	}
	if e.RootTraceID != "" {
		m["root_trace_id"] = e.RootTraceID
	}
	if len(e.MiddleSpanIDs) > 0 {
		m["middle_span_ids"] = e.MiddleSpanIDs
	}
	if e.CurrentSpanID != "" {
		m["current_span_id"] = e.CurrentSpanID
	}
	for _, f := range e.fields {
		if _, reserved := m[f.key]; reserved {
			continue // 跳过与内置字段同名的 kv
		}
		m[f.key] = f.value
	}
	return json.Marshal(m)
}

// parseArgs 将 kv args 解析为有序字段列表（slog 语义）
// key 必须为非空 string，否则记为 !BADKEY；奇数个参数时末尾裸值记为 !BADKEY
func parseArgs(args []any) []field {
	if len(args) == 0 {
		return nil
	}
	fields := make([]field, 0, (len(args)+1)/2)
	for i := 0; i+1 < len(args); i += 2 {
		key, ok := args[i].(string)
		if !ok || key == "" {
			key = "!BADKEY"
		}
		fields = append(fields, field{key: key, value: args[i+1]})
	}
	if len(args)%2 == 1 {
		fields = append(fields, field{key: "!BADKEY", value: args[len(args)-1]})
	}
	return fields
}
