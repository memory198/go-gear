package logger

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"

	"github.com/memory198/go-gear/logger/core"
)

// ---- JSON 编码器（手写，字段对齐 OTel Logs Data Model） ----

type jsonEncoder struct{}

const hexDigits = "0123456789abcdef"

func (jsonEncoder) appendTo(b []byte, r core.Record) []byte {
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
	b = append(b, '"')
	b = appendTime(b, r.Timestamp)
	b = append(b, '"')
	key("severity_number")
	b = strconv.AppendInt(b, int64(r.Level), 10)
	key("severity_text")
	b = appendJSONString(b, r.Level.String())
	key("body")
	b = appendJSONString(b, r.Body)

	if r.CodeFilepath != "" {
		key("code.filepath")
		b = appendJSONString(b, r.CodeFilepath)
		key("code.lineno")
		b = strconv.AppendInt(b, int64(r.CodeLineno), 10)
	}
	if r.TraceID != "" {
		key("trace_id")
		b = appendJSONString(b, r.TraceID)
	}
	if r.SpanID != "" {
		key("span_id")
		b = appendJSONString(b, r.SpanID)
	}
	if len(r.ParentSpanIDs) > 0 {
		key("parent_span_ids")
		b = append(b, '[')
		for i, id := range r.ParentSpanIDs {
			if i > 0 {
				b = append(b, ',')
			}
			b = appendJSONString(b, id)
		}
		b = append(b, ']')
	}
	if !r.Resource.Empty() {
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
		if r.Resource.ServiceName != "" {
			rkey("service.name")
			b = appendJSONString(b, r.Resource.ServiceName)
		}
		if r.Resource.ServiceVersion != "" {
			rkey("service.version")
			b = appendJSONString(b, r.Resource.ServiceVersion)
		}
		if r.Resource.Environment != "" {
			rkey("deployment.environment")
			b = appendJSONString(b, r.Resource.Environment)
		}
		if r.Resource.HostName != "" {
			rkey("host.name")
			b = appendJSONString(b, r.Resource.HostName)
		}
		b = append(b, '}')
	}
	if len(r.Attrs) > 0 {
		key("attributes")
		b = append(b, '{')
		for i, a := range r.Attrs {
			if i > 0 {
				b = append(b, ',')
			}
			b = appendJSONString(b, a.Key)
			b = append(b, ':')
			b = appendJSONValue(b, a.Value)
		}
		b = append(b, '}')
	}

	// 以换行结尾，保证“一行一条”（JSON Lines），与 text 编码器一致，便于按行采集
	return append(b, '}', '\n')
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
