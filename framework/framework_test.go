package framework

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBind(t *testing.T) {
	// 测试JSON绑定
	t.Run("JSON binding", func(t *testing.T) {
		type TestStruct struct {
			Name string `json:"name"`
			Age  int    `json:"age"`
		}

		body := `{"name":"test","age":25}`
		req := httptest.NewRequest("POST", "/test", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")

		var result TestStruct
		err := Bind(req, &result)
		if err != nil {
			t.Fatalf("Bind failed: %v", err)
		}

		if result.Name != "test" {
			t.Errorf("Expected name 'test', got %s", result.Name)
		}
		if result.Age != 25 {
			t.Errorf("Expected age 25, got %d", result.Age)
		}
	})

	// 测试空请求体
	t.Run("empty body", func(t *testing.T) {
		type TestStruct struct {
			Name string `json:"name"`
		}

		req := httptest.NewRequest("GET", "/test", nil)
		var result TestStruct
		err := Bind(req, &result)
		if err != nil {
			t.Fatalf("Bind failed: %v", err)
		}

		if result.Name != "" {
			t.Errorf("Expected empty name, got %s", result.Name)
		}
	})

}

func TestQueryInt(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		key        string
		defaultVal int
		expected   int
	}{
		{"valid integer", "page=5", "page", 1, 5},
		{"missing key", "", "page", 1, 1},
		{"invalid integer", "page=abc", "page", 1, 1},
		{"empty value", "page=", "page", 1, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test?"+tt.query, nil)
			result := QueryInt(req, tt.key, tt.defaultVal)
			if result != tt.expected {
				t.Errorf("QueryInt(%s, %d) = %d, want %d", tt.key, tt.defaultVal, result, tt.expected)
			}
		})
	}
}

func TestQueryString(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		key        string
		defaultVal string
		expected   string
	}{
		{"valid string", "name=test", "name", "default", "test"},
		{"missing key", "", "name", "default", "default"},
		{"empty value", "name=", "name", "default", "default"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test?"+tt.query, nil)
			result := QueryString(req, tt.key, tt.defaultVal)
			if result != tt.expected {
				t.Errorf("QueryString(%s, %s) = %s, want %s", tt.key, tt.defaultVal, result, tt.expected)
			}
		})
	}
}

func TestBizError(t *testing.T) {
	// 测试错误消息
	t.Run("error message", func(t *testing.T) {
		err := ErrBadRequest.WithMsg("invalid parameter")
		if err.Error() != "code=40000, message=invalid parameter" {
			t.Errorf("Error() = %s, want 'code=40000, message=invalid parameter'", err.Error())
		}
	})

	// 测试HTTP状态码
	t.Run("HTTP status", func(t *testing.T) {
		if ErrUnauthorized.HTTPStatus != 401 {
			t.Errorf("ErrUnauthorized.HTTPStatus = %d, want 401", ErrUnauthorized.HTTPStatus)
		}
	})

	// 测试业务错误码
	t.Run("business code", func(t *testing.T) {
		if ErrForbidden.Code != 40300 {
			t.Errorf("ErrForbidden.Code = %d, want 40300", ErrForbidden.Code)
		}
	})
}

func TestWriteOK(t *testing.T) {
	w := httptest.NewRecorder()
	data := map[string]string{"key": "value"}

	WriteOK(w, data)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp.Code != 0 {
		t.Errorf("Expected code 0, got %d", resp.Code)
	}
	if resp.Message != "ok" {
		t.Errorf("Expected message 'ok', got %s", resp.Message)
	}
}

func TestWriteError(t *testing.T) {
	// 测试业务错误
	t.Run("business error", func(t *testing.T) {
		w := httptest.NewRecorder()
		err := ErrNotFound.WithMsg("resource not found")

		WriteError(w, err)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", w.Code)
		}

		var resp Response
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		if resp.Code != ErrNotFound.Code {
			t.Errorf("Expected code %d, got %d", ErrNotFound.Code, resp.Code)
		}
		if resp.Message != "resource not found" {
			t.Errorf("Expected message 'resource not found', got %s", resp.Message)
		}
	})

	// 测试非业务错误
	t.Run("non-business error", func(t *testing.T) {
		w := httptest.NewRecorder()
		err := ErrInternal.WithMsg("internal error")

		WriteError(w, err)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("Expected status 500, got %d", w.Code)
		}
	})
}
