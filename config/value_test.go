package config

import (
	"testing"
)

// ============================================================
// Value 类型转换测试
//
// Value 是配置路径查询的返回类型，封装了"是否存在"和"原始值"，
// 提供 String/Int/Float64/Bool/Raw 五种类型转换方法，
// 每种方法均支持可选默认值参数。
// ============================================================

// TestValue_Exists 验证路径存在性标志
func TestValue_Exists(t *testing.T) {
	// 路径存在时 ok=true
	v := newValue("hello", true)
	if !v.Exists() {
		t.Error("期望 Exists()=true，实际为 false")
	}

	// 路径不存在时 ok=false
	v = newValue(nil, false)
	if v.Exists() {
		t.Error("期望 Exists()=false，实际为 true")
	}
}

// ============================================================
// String 测试
// ============================================================

// TestValue_String_NativeString 原始值本身是 string，直接返回
func TestValue_String_NativeString(t *testing.T) {
	v := newValue("hello", true)
	if got := v.String(); got != "hello" {
		t.Errorf("期望 \"hello\"，实际 %q", got)
	}
}

// TestValue_String_NonString 原始值不是 string（如 int），通过 fmt.Sprint 转换
func TestValue_String_NonString(t *testing.T) {
	v := newValue(42, true)
	if got := v.String(); got != "42" {
		t.Errorf("期望 \"42\"，实际 %q", got)
	}
}

// TestValue_String_Missing 路径不存在时返回空字符串（无默认值参数）
func TestValue_String_Missing(t *testing.T) {
	v := newValue(nil, false)
	if got := v.String(); got != "" {
		t.Errorf("期望空字符串，实际 %q", got)
	}
}

// TestValue_String_DefaultVal 路径不存在时返回调用方传入的默认值
func TestValue_String_DefaultVal(t *testing.T) {
	v := newValue(nil, false)
	if got := v.String("fallback"); got != "fallback" {
		t.Errorf("期望 \"fallback\"，实际 %q", got)
	}
}

// TestValue_String_NilRaw raw=nil 但 ok=true 时（路径存在但值为 null），返回默认值
func TestValue_String_NilRaw(t *testing.T) {
	v := newValue(nil, true)
	if got := v.String("def"); got != "def" {
		t.Errorf("期望 \"def\"，实际 %q", got)
	}
}

// ============================================================
// Int 测试
// ============================================================

// TestValue_Int_NativeInt 原始值是 int，直接返回
func TestValue_Int_NativeInt(t *testing.T) {
	v := newValue(99, true)
	if got := v.Int(); got != 99 {
		t.Errorf("期望 99，实际 %d", got)
	}
}

// TestValue_Int_Int64 原始值是 int64，转换为 int 返回
func TestValue_Int_Int64(t *testing.T) {
	v := newValue(int64(1234), true)
	if got := v.Int(); got != 1234 {
		t.Errorf("期望 1234，实际 %d", got)
	}
}

// TestValue_Int_Float64 YAML 数字默认解析为 float64，需能正确截断为 int
func TestValue_Int_Float64(t *testing.T) {
	v := newValue(float64(7), true)
	if got := v.Int(); got != 7 {
		t.Errorf("期望 7，实际 %d", got)
	}
}

// TestValue_Int_StringParseable 原始值是可解析的数字字符串
func TestValue_Int_StringParseable(t *testing.T) {
	v := newValue("100", true)
	if got := v.Int(); got != 100 {
		t.Errorf("期望 100，实际 %d", got)
	}
}

// TestValue_Int_Missing 路径不存在时返回 0（无默认值参数）
func TestValue_Int_Missing(t *testing.T) {
	v := newValue(nil, false)
	if got := v.Int(); got != 0 {
		t.Errorf("期望 0，实际 %d", got)
	}
}

// TestValue_Int_DefaultVal 路径不存在时返回调用方传入的默认值
func TestValue_Int_DefaultVal(t *testing.T) {
	v := newValue(nil, false)
	if got := v.Int(42); got != 42 {
		t.Errorf("期望 42，实际 %d", got)
	}
}

// ============================================================
// Float64 测试
// ============================================================

// TestValue_Float64_NativeFloat64 原始值是 float64，直接返回
func TestValue_Float64_NativeFloat64(t *testing.T) {
	v := newValue(3.14, true)
	if got := v.Float64(); got != 3.14 {
		t.Errorf("期望 3.14，实际 %f", got)
	}
}

// TestValue_Float64_NativeInt 原始值是 int，转换为 float64
func TestValue_Float64_NativeInt(t *testing.T) {
	v := newValue(5, true)
	if got := v.Float64(); got != 5.0 {
		t.Errorf("期望 5.0，实际 %f", got)
	}
}

// TestValue_Float64_Missing 路径不存在时返回 0.0
func TestValue_Float64_Missing(t *testing.T) {
	v := newValue(nil, false)
	if got := v.Float64(); got != 0.0 {
		t.Errorf("期望 0.0，实际 %f", got)
	}
}

// TestValue_Float64_DefaultVal 路径不存在时返回调用方传入的默认值
func TestValue_Float64_DefaultVal(t *testing.T) {
	v := newValue(nil, false)
	if got := v.Float64(1.5); got != 1.5 {
		t.Errorf("期望 1.5，实际 %f", got)
	}
}

// ============================================================
// Bool 测试
// ============================================================

// TestValue_Bool_NativeBool 原始值是 bool，直接返回
func TestValue_Bool_NativeBool(t *testing.T) {
	if !newValue(true, true).Bool() {
		t.Error("期望 true，实际 false")
	}
	if newValue(false, true).Bool() {
		t.Error("期望 false，实际 true")
	}
}

// TestValue_Bool_StringTrue 字符串 "true" 应转换为 true
func TestValue_Bool_StringTrue(t *testing.T) {
	v := newValue("true", true)
	if !v.Bool() {
		t.Error("期望 \"true\" 字符串转换为 bool true，实际 false")
	}
}

// TestValue_Bool_StringFalse 非 "true" 字符串均视为 false
func TestValue_Bool_StringFalse(t *testing.T) {
	for _, s := range []string{"false", "1", "yes", "True", "TRUE"} {
		v := newValue(s, true)
		if v.Bool() {
			t.Errorf("期望字符串 %q 转换为 false，实际为 true", s)
		}
	}
}

// TestValue_Bool_Missing 路径不存在时返回 false（无默认值参数）
func TestValue_Bool_Missing(t *testing.T) {
	v := newValue(nil, false)
	if v.Bool() {
		t.Error("期望 false，实际 true")
	}
}

// TestValue_Bool_DefaultVal 路径不存在时返回调用方传入的默认值
func TestValue_Bool_DefaultVal(t *testing.T) {
	v := newValue(nil, false)
	if !v.Bool(true) {
		t.Error("期望默认值 true，实际 false")
	}
}

// ============================================================
// Raw 测试
// ============================================================

// TestValue_Raw 返回未经转换的原始 any 值
func TestValue_Raw(t *testing.T) {
	want := map[string]any{"k": "v"}
	v := newValue(want, true)
	got, ok := v.Raw().(map[string]any)
	if !ok {
		t.Fatal("期望 Raw() 返回 map[string]any")
	}
	if got["k"] != "v" {
		t.Errorf("期望 Raw()[\"k\"] = \"v\"，实际 %v", got["k"])
	}
}

// TestValue_Raw_Nil raw=nil 时 Raw() 返回 nil
func TestValue_Raw_Nil(t *testing.T) {
	v := newValue(nil, false)
	if v.Raw() != nil {
		t.Errorf("期望 nil，实际 %v", v.Raw())
	}
}
