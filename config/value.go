package config

import "fmt"

// Value 配置值，支持链式类型转换
type Value struct {
	raw any
	ok  bool // 路径是否存在
}

func newValue(raw any, ok bool) Value {
	return Value{raw: raw, ok: ok}
}

// Exists 路径是否存在
func (v Value) Exists() bool {
	return v.ok
}

// String 转为字符串，不存在或转换失败时返回 defaultVal
func (v Value) String(defaultVal ...string) string {
	def := ""
	if len(defaultVal) > 0 {
		def = defaultVal[0]
	}
	if !v.ok || v.raw == nil {
		return def
	}
	if s, ok := v.raw.(string); ok {
		return s
	}
	return fmt.Sprint(v.raw)
}

// Int 转为 int，不存在或转换失败时返回 defaultVal
func (v Value) Int(defaultVal ...int) int {
	def := 0
	if len(defaultVal) > 0 {
		def = defaultVal[0]
	}
	if !v.ok || v.raw == nil {
		return def
	}
	switch val := v.raw.(type) {
	case int:
		return val
	case int64:
		return int(val)
	case float64:
		return int(val)
	}
	var i int
	fmt.Sscanf(fmt.Sprint(v.raw), "%d", &i)
	return i
}

// Float64 转为 float64
func (v Value) Float64(defaultVal ...float64) float64 {
	def := 0.0
	if len(defaultVal) > 0 {
		def = defaultVal[0]
	}
	if !v.ok || v.raw == nil {
		return def
	}
	switch val := v.raw.(type) {
	case float64:
		return val
	case int:
		return float64(val)
	}
	var f float64
	fmt.Sscanf(fmt.Sprint(v.raw), "%f", &f)
	return f
}

// Bool 转为 bool
func (v Value) Bool(defaultVal ...bool) bool {
	def := false
	if len(defaultVal) > 0 {
		def = defaultVal[0]
	}
	if !v.ok || v.raw == nil {
		return def
	}
	if b, ok := v.raw.(bool); ok {
		return b
	}
	return fmt.Sprint(v.raw) == "true"
}

// Raw 获取原始值
func (v Value) Raw() any {
	return v.raw
}
