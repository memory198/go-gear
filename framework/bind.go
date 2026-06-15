package framework

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// Binder 允许 req 自定义绑定逻辑（用于混合绑定 URL 参数、Query 等）
type Binder interface {
	Bind(r *http.Request) error
}

// Bind 绑定请求参数到 req
// 优先使用自定义 Binder，否则默认解析 JSON body
func Bind(r *http.Request, req any) error {
	if b, ok := req.(Binder); ok {
		return b.Bind(r)
	}
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(req); err != nil {
			return fmt.Errorf("解析请求体失败: %w", err)
		}
	}
	return nil
}

// QueryInt 从 URL 查询参数中读取整数值
// 参数：
//   - r: HTTP 请求
//   - key: 查询参数名
//   - defaultVal: 解析失败或参数不存在时的默认值
//
// 返回：解析后的整数值，失败时返回默认值
func QueryInt(r *http.Request, key string, defaultVal int) int {
	val, err := strconv.Atoi(r.URL.Query().Get(key))
	if err != nil {
		return defaultVal
	}
	return val
}

// QueryString 从 URL 查询参数中读取字符串值
// 参数：
//   - r: HTTP 请求
//   - key: 查询参数名
//   - defaultVal: 参数为空或不存在时的默认值
//
// 返回：查询参数值，为空时返回默认值
func QueryString(r *http.Request, key, defaultVal string) string {
	if val := r.URL.Query().Get(key); val != "" {
		return val
	}
	return defaultVal
}
