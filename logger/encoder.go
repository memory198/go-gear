package logger

import (
	"encoding/json"
	"fmt"
)

// encoder 日志编码器接口：将 entry 转为最终输出字符串
type encoder interface {
	encode(e *entry) string
}

// ---- text 编码器：仅展示 root_trace_id，不输出 middle_span_ids / current_span_id ----

type textEncoder struct{}

func (textEncoder) encode(e *entry) string {
	var callerPart string
	if e.Caller != "" {
		callerPart = e.Caller + " "
	}
	if e.RootTraceID != "" {
		return fmt.Sprintf("%s [%s] [%s] %s%s\n",
			e.Time, e.Level, e.RootTraceID, callerPart, e.Msg,
		)
	}
	return fmt.Sprintf("%s [%s] %s%s\n",
		e.Time, e.Level, callerPart, e.Msg,
	)
}

// ---- json 编码器：完整携带三类链路字段，middle_span_ids 为数组 ----

type jsonEncoder struct{}

func (jsonEncoder) encode(e *entry) string {
	b, err := json.Marshal(e)
	if err != nil {
		return fmt.Sprintf(`{"level":"ERROR","msg":"json marshal failed: %v"}`+"\n", err)
	}
	return string(b) + "\n"
}
