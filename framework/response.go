package framework

import (
	"encoding/json"
	"errors"
	"net/http"
)

// Response 统一响应结构
type Response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func WriteOK(w http.ResponseWriter, data any) {
	writeJSON(w, http.StatusOK, Response{
		Code:    0,
		Message: "ok",
		Data:    data,
	})
}

func WriteError(w http.ResponseWriter, err error) {
	var bizErr *BizError
	if errors.As(err, &bizErr) {
		writeJSON(w, bizErr.HTTPStatus, Response{
			Code:    bizErr.Code,
			Message: bizErr.Message,
		})
		return
	}
	writeJSON(w, http.StatusInternalServerError, Response{
		Code:    ErrInternal.Code,
		Message: ErrInternal.Message,
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
