package logger

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/memory198/go-gear/logger/core"
)

// ---- text 编码器（人读优先，省略 resource / parent_span_ids） ----

type textEncoder struct{}

func (textEncoder) appendTo(b []byte, r core.Record) []byte {
	b = appendTime(b, r.Timestamp)
	b = append(b, " ["...)
	b = append(b, r.Level.String()...)
	b = append(b, ']')
	if r.TraceID != "" {
		b = append(b, " ["...)
		b = append(b, r.TraceID...)
		b = append(b, ']')
	}
	if r.CodeFilepath != "" {
		b = append(b, ' ')
		b = append(b, r.CodeFilepath...)
		b = append(b, ':')
		b = strconv.AppendInt(b, int64(r.CodeLineno), 10)
	}
	b = append(b, ' ')
	b = appendTextEscaped(b, r.Body)
	for _, a := range r.Attrs {
		b = append(b, ' ')
		b = appendTextEscaped(b, a.Key)
		b = append(b, '=')
		b = appendTextValue(b, a.Value)
	}
	return append(b, '\n')
}

// appendTextEscaped 追加文本并转义换行/回车（保证“一行一条”）
// 绝大多数日志无换行，先走快速路径（IndexByte），避免逐字节循环开销
func appendTextEscaped(b []byte, s string) []byte {
	if strings.IndexByte(s, '\n') < 0 && strings.IndexByte(s, '\r') < 0 {
		return append(b, s...)
	}
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\n':
			b = append(b, '\\', 'n')
		case '\r':
			b = append(b, '\\', 'r')
		default:
			b = append(b, s[i])
		}
	}
	return b
}

// appendTextValue 追加文本形式的值（等价 fmt.Sprint 的常用类型快路径）
func appendTextValue(b []byte, v any) []byte {
	switch x := v.(type) {
	case nil:
		return append(b, "<nil>"...)
	case string:
		return appendTextEscaped(b, x)
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
		return appendTextEscaped(b, x.Error())
	case []byte:
		return appendTextEscaped(b, string(x))
	default:
		return appendTextEscaped(b, fmt.Sprint(x)) // 复杂值兜底
	}
}
