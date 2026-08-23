// Package domain 定义领域模型、状态枚举与领域错误。
package domain

import (
	"errors"
	"fmt"
)

// 领域错误哨兵，HTTP 层据此映射状态码。
var (
	// ErrNotFound 资源不存在。
	ErrNotFound = errors.New("资源不存在")
	// ErrConflict 乐观锁冲突或唯一约束冲突。
	ErrConflict = errors.New("数据冲突")
	// ErrInvalid 参数非法。
	ErrInvalid = errors.New("参数非法")
	// ErrState 状态机不允许的转换。
	ErrState = errors.New("状态不允许该操作")
	// ErrRule 业务规则不满足。
	ErrRule = errors.New("业务规则不满足")
)

// Error 带错误码的领域错误。
type Error struct {
	// Kind 归属的哨兵错误。
	Kind error
	// Msg 具体描述。
	Msg string
}

// Error 实现 error 接口。
func (e *Error) Error() string { return fmt.Sprintf("%s: %s", e.Kind, e.Msg) }

// Unwrap 支持 errors.Is。
func (e *Error) Unwrap() error { return e.Kind }

// 各类领域错误的构造函数。

// NotFoundf 构造资源不存在错误。
func NotFoundf(format string, args ...any) *Error {
	return &Error{Kind: ErrNotFound, Msg: fmt.Sprintf(format, args...)}
}

// Conflictf 构造冲突错误。
func Conflictf(format string, args ...any) *Error {
	return &Error{Kind: ErrConflict, Msg: fmt.Sprintf(format, args...)}
}

// Invalidf 构造参数错误。
func Invalidf(format string, args ...any) *Error {
	return &Error{Kind: ErrInvalid, Msg: fmt.Sprintf(format, args...)}
}

// Statef 构造状态错误。
func Statef(format string, args ...any) *Error {
	return &Error{Kind: ErrState, Msg: fmt.Sprintf(format, args...)}
}

// Rulef 构造业务规则错误。
func Rulef(format string, args ...any) *Error {
	return &Error{Kind: ErrRule, Msg: fmt.Sprintf(format, args...)}
}
