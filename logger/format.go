package logger

// Format 日志输出格式
type Format int

const (
	TextFormat Format = iota // 非结构化文本，人读优先
	JSONFormat               // 结构化 JSON，字段对齐 OTel Logs Data Model
)

func parseFormat(s string) Format {
	switch s {
	case "json":
		return JSONFormat
	default:
		return TextFormat
	}
}

// entry 一条日志的结构化表示（字段命名对齐 OTel Logs Data Model）
// json 标签仅作说明，实际序列化由手写编码器完成
type entry struct {
	Timestamp     string   // "timestamp"：RFC3339 微秒 + 时区
	SeverityText  string   // "severity_text"：DEBUG/INFO/WARN/ERROR/FATAL
	Body          string   // "body"：日志正文
	CodeFilepath  string   // "code.filepath"：调用位置文件（Caller 开启时非空）
	CodeLineno    int      // "code.lineno"：调用位置行号
	TraceID       string   // "trace_id"：根链路 trace id
	SpanID        string   // "span_id"：当前 span id
	ParentSpanIDs []string // "parent_span_ids"：中间 span 链（默认关闭，见 Config.MiddleSpanIDs）
	Resource      resource // "resource"：部署/服务元信息
	Attrs         []field  // "attributes"：自定义键值对（有序）
}

// resource 部署/服务元信息（OTel Resource 语义约定，输出为点分键）
type resource struct {
	ServiceName    string // service.name
	ServiceVersion string // service.version
	Environment    string // deployment.environment
	HostName       string // host.name
}

// empty 是否所有 resource 字段均为空（全空时输出省略 resource 对象）
func (r resource) empty() bool {
	return r.ServiceName == "" && r.ServiceVersion == "" &&
		r.Environment == "" && r.HostName == ""
}

// field 单个自定义键值对
type field struct {
	key   string
	value any
}

// reservedKeys 内置字段名集合
// 自定义 kv 与内置字段重名时跳过，防止覆盖内置字段导致类型/结构异常
var reservedKeys = map[string]bool{
	"timestamp":              true,
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

// parseArgs 将 kv args 解析为有序字段列表（slog 语义）
// key 必须为非空 string，否则记为 !BADKEY；奇数个参数时末尾裸值记为 !BADKEY
// 与内置字段重名的 kv 直接丢弃
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
		if reservedKeys[key] {
			continue // 内置字段优先，丢弃同名的自定义字段
		}
		fields = append(fields, field{key: key, value: args[i+1]})
	}
	if len(args)%2 == 1 {
		fields = append(fields, field{key: "!BADKEY", value: args[len(args)-1]})
	}
	return fields
}
