// Package httpx HTTP 层：路由、统一响应、错误映射、分页解析、中间件。
package httpx

import (
	"encoding/json"
	"errors"
	"net/http"

	"gowork/internal/domain"
)

// errorBody 统一错误响应体。
type errorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// WriteJSON 写出 JSON 响应。
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// WriteError 统一错误响应，按领域错误映射状态码。
func WriteError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	code := "internal"
	switch {
	case errors.Is(err, domain.ErrInvalid):
		status, code = http.StatusBadRequest, "invalid"
	case errors.Is(err, domain.ErrNotFound):
		status, code = http.StatusNotFound, "not_found"
	case errors.Is(err, domain.ErrConflict):
		status, code = http.StatusConflict, "conflict"
	case errors.Is(err, domain.ErrState):
		status, code = http.StatusConflict, "state_conflict"
	case errors.Is(err, domain.ErrRule):
		status, code = http.StatusUnprocessableEntity, "rule_violation"
	}
	var body errorBody
	body.Error.Code = code
	body.Error.Message = err.Error()
	WriteJSON(w, status, body)
}

// DecodeJSON 解析请求体。
func DecodeJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return domain.Invalidf("请求体 JSON 解析失败: %v", err)
	}
	return nil
}
