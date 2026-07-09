package logger

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
	Caller        string   `json:"caller"`
	RootTraceID   string   `json:"root_trace_id,omitempty"`
	MiddleSpanIDs []string `json:"middle_span_ids,omitempty"`
	CurrentSpanID string   `json:"current_span_id,omitempty"`
}
