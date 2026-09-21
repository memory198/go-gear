package logger

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"sync"
)

// bufPool 复用日志输出缓冲，避免每行日志重复分配
var bufPool = sync.Pool{
	New: func() any {
		b := make([]byte, 0, 512)
		return &b
	},
}

// encoder 日志编码器：将 entry 序列化后追加到 dst 并返回新切片
type encoder interface {
	appendTo(dst []byte, e *entry) []byte
}

// ---- text 编码器（人读优先，省略 resource / parent_span_ids） ----

type textEncoder struct{}

func (textEncoder) appendTo(b []byte, e *entry) []byte {
	b = append(b, e.Timestamp...)
	b = append(b, " ["...)
	b = append(b, e.SeverityText...)
	b = append(b, ']')
	if e.TraceID != "" {
		b = append(b, " ["...)
		b = append(b, e.TraceID...)
		b = append(b, ']')
	}
	if e.CodeFilepath != "" {
		b = append(b, ' ')
		b = append(b, e.CodeFilepath...)
		b = append(b, ':')
		b = strconv.AppendInt(b, int64(e.CodeLineno), 10)
	}
	b = append(b, ' ')
	b = append(b, e.Body...)
	for _, f := range e.Attrs {
		b = append(b, ' ')
		b = append(b, f.key...)
		b = append(b, '=')
		b = appendTextValue(b, f.value)
	}
	return append(b, '\n')
}

// appendTextValue 追加文本形式的值（等价 fmt.Sprint 的常用类型快路径）
func appendTextValue(b []byte, v any) []byte {
	switch x := v.(type) {
	case nil:
		return append(b, "<nil>"...)
	case string:
		return append(b, x...)
	case bool:
		if x {
			return append(b, "true"...)
		}
		return append(b, "false"...)
	case int:
		return strconv.AppendInt(b, int64(x), 10)
	case int8:
		return strconv.AppendInt(b, int64(x), 10)
	case int16:
		return strconv.AppendInt(b, int64(x), 10)
	case int32:
		return strconv.AppendInt(b, int64(x), 10)
	case int64:
		return strconv.AppendInt(b, x, 10)
	case uint:
		return strconv.AppendUint(b, uint64(x), 10)
	case uint8:
		return strconv.AppendUint(b, uint64(x), 10)
	case uint16:
		return strconv.AppendUint(b, uint64(x), 10)
	case uint32:
		return strconv.AppendUint(b, uint64(x), 10)
	case uint64:
		return strconv.AppendUint(b, x, 10)
	case float32:
		return strconv.AppendFloat(b, float64(x), 'g', -1, 32)
	case float64:
		return strconv.AppendFloat(b, x, 'g', -1, 64)
	case error:
		return append(b, x.Error()...)
	case []byte:
		return append(b, x...)
	default:
		return append(b, fmt.Sprint(x)...) // 复杂值兜底
	}
}

// ---- JSON 编码器（手写，字段对齐 OTel Logs Data Model） ----

type jsonEncoder struct{}

const hexDigits = "0123456789abcdef"

func (jsonEncoder) appendTo(b []byte, e *entry) []byte {
	b = append(b, '{')
	first := true

	// 追加 "key":（自动处理逗号）
	key := func(k string) {
		if first {
			first = false
		} else {
			b = append(b, ',')
		}
		b = appendJSONString(b, k)
		b = append(b, ':')
	}

	key("timestamp")
	b = appendJSONString(b, e.Timestamp)
	key("severity_text")
	b = appendJSONString(b, e.SeverityText)
	key("body")
	b = appendJSONString(b, e.Body)

	if e.CodeFilepath != "" {
		key("code.filepath")
		b = appendJSONString(b, e.CodeFilepath)
		key("code.lineno")
		b = strconv.AppendInt(b, int64(e.CodeLineno), 10)
	}
	if e.TraceID != "" {
		key("trace_id")
		b = appendJSONString(b, e.TraceID)
	}
	if e.SpanID != "" {
		key("span_id")
		b = appendJSONString(b, e.SpanID)
	}
	if len(e.ParentSpanIDs) > 0 {
		key("parent_span_ids")
		b = append(b, '[')
		for i, id := range e.ParentSpanIDs {
			if i > 0 {
				b = append(b, ',')
			}
			b = appendJSONString(b, id)
		}
		b = append(b, ']')
	}
	if !e.Resource.empty() {
		key("resource")
		b = append(b, '{')
		rf := true
		rkey := func(k string) {
			if rf {
				rf = false
			} else {
				b = append(b, ',')
			}
			b = appendJSONString(b, k)
			b = append(b, ':')
		}
		if e.Resource.ServiceName != "" {
			rkey("service.name")
			b = appendJSONString(b, e.Resource.ServiceName)
		}
		if e.Resource.ServiceVersion != "" {
			rkey("service.version")
			b = appendJSONString(b, e.Resource.ServiceVersion)
		}
		if e.Resource.Environment != "" {
			rkey("deployment.environment")
			b = appendJSONString(b, e.Resource.Environment)
		}
		if e.Resource.HostName != "" {
			rkey("host.name")
			b = appendJSONString(b, e.Resource.HostName)
		}
		b = append(b, '}')
	}
	if len(e.Attrs) > 0 {
		key("attributes")
		b = append(b, '{')
		for i, f := range e.Attrs {
			if i > 0 {
				b = append(b, ',')
			}
			b = appendJSONString(b, f.key)
			b = append(b, ':')
			b = appendJSONValue(b, f.value)
		}
		b = append(b, '}')
	}

	return append(b, '}')
}

// appendJSONValue 追加 JSON 值
// 常用类型走快路径；复杂类型（结构体/map/切片等）退回 encoding/json
func appendJSONValue(b []byte, v any) []byte {
	switch x := v.(type) {
	case nil:
		return append(b, "null"...)
	case string:
		return appendJSONString(b, x)
	case bool:
		if x {
			return append(b, "true"...)
		}
		return append(b, "false"...)
	case int:
		return strconv.AppendInt(b, int64(x), 10)
	case int8:
		return strconv.AppendInt(b, int64(x), 10)
	case int16:
		return strconv.AppendInt(b, int64(x), 10)
	case int32:
		return strconv.AppendInt(b, int64(x), 10)
	case int64:
		return strconv.AppendInt(b, x, 10)
	case uint:
		return strconv.AppendUint(b, uint64(x), 10)
	case uint8:
		return strconv.AppendUint(b, uint64(x), 10)
	case uint16:
		return strconv.AppendUint(b, uint64(x), 10)
	case uint32:
		return strconv.AppendUint(b, uint64(x), 10)
	case uint64:
		return strconv.AppendUint(b, x, 10)
	case float32:
		return appendJSONFloat(b, float64(x), 32)
	case float64:
		return appendJSONFloat(b, x, 64)
	case error:
		return appendJSONString(b, x.Error())
	case []byte:
		return appendJSONString(b, string(x))
	default:
		if enc, err := json.Marshal(x); err == nil {
			return append(b, enc...)
		}
		return appendJSONString(b, fmt.Sprint(x))
	}
}

// appendJSONFloat 追加浮点数（NaN/Inf 在 JSON 中非法，输出 null）
func appendJSONFloat(b []byte, f float64, bitSize int) []byte {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return append(b, "null"...)
	}
	return strconv.AppendFloat(b, f, 'g', -1, bitSize)
}

// appendJSONString 追加 JSON 字符串（转义 " \ 与控制字符，UTF-8 原样透传）
func appendJSONString(b []byte, s string) []byte {
	b = append(b, '"')
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"':
			b = append(b, '\\', '"')
		case c == '\\':
			b = append(b, '\\', '\\')
		case c == '\n':
			b = append(b, '\\', 'n')
		case c == '\r':
			b = append(b, '\\', 'r')
		case c == '\t':
			b = append(b, '\\', 't')
		case c < 0x20:
			b = append(b, '\\', 'u', '0', '0', hexDigits[c>>4], hexDigits[c&0xF])
		default:
			b = append(b, c)
		}
	}
	return append(b, '"')
}
