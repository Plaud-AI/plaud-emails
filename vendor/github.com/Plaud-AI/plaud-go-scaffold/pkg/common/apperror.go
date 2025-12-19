package common

import (
	"errors"
	"fmt"
)

// AppError 统一业务错误定义
//
// Code: 业务错误码（用于客户端判定）。
// Message: 面向用户/客户端的错误信息。
// Cause: 内部错误链，便于追踪。
// Details: 额外上下文（可选，不会自动对外暴露）。
type AppError struct {
	Code    int
	Message string
	Cause   error
	Details any
}

func (e *AppError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Cause != nil {
		return fmt.Sprintf("code=%d msg=%s cause=%v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("code=%d msg=%s", e.Code, e.Message)
}

// Unwrap 使得 errors.Is / errors.As 可用
func (e *AppError) Unwrap() error { return e.Cause }

// WithDetails 附加额外上下文
func (e *AppError) WithDetails(details any) *AppError {
	e.Details = details
	return e
}

// WithCause 追加底层错误
func (e *AppError) WithCause(err error) *AppError {
	e.Cause = err
	return e
}

// Clone 创建一个副本，避免修改全局模板变量
func (e *AppError) Clone() *AppError {
	if e == nil {
		return nil
	}
	copy := *e
	// 副本默认不携带上一层的 Cause/Details
	copy.Cause = nil
	copy.Details = nil
	return &copy
}

// WithMessage 在副本上设置新消息，避免污染全局模板
func (e *AppError) WithMessage(message string) *AppError {
	clone := e.Clone()
	if clone == nil {
		return nil
	}
	clone.Message = message
	return clone
}

// WithMessagef 在副本上以格式化字符串设置新消息
func (e *AppError) WithMessagef(format string, a ...any) *AppError {
	clone := e.Clone()
	if clone == nil {
		return nil
	}
	clone.Message = fmt.Sprintf(format, a...)
	return clone
}

// NewError 构建一个 AppError
func NewError(code int, message string) *AppError {
	return &AppError{Code: code, Message: message}
}

// NewErrorf 构建一个带格式化消息的 AppError
func NewErrorf(code int, format string, a ...any) *AppError {
	return &AppError{Code: code, Message: fmt.Sprintf(format, a...)}
}

// FromError 将任意 error 规范化为 AppError
func FromError(err error) *AppError {
	if err == nil {
		return nil
	}
	var ae *AppError
	if errors.As(err, &ae) {
		return ae
	}
	// 未知错误统一映射为内部错误
	return Internal("internal error").WithCause(err)
}

// IsErrorCodeSame 判断两个错误是否具有相同的AppError错误码
func IsErrorCodeSame(err1, err2 error) bool {
	if err1 == nil || err2 == nil {
		return false
	}
	appErr1, ok1 := err1.(*AppError)
	appErr2, ok2 := err2.(*AppError)
	if !ok1 || !ok2 {
		return false
	}
	return appErr1.Code == appErr2.Code
}

// ErrorCode 从 err中获取错误码
func ErrorCode(err error) int {
	if err == nil {
		return CodeOK
	}
	if appErr, ok := err.(*AppError); ok {
		return appErr.Code
	}
	var appErr *AppError
	if errors.As(err, &appErr) && appErr != nil {
		return appErr.Code
	}
	return CodeUnknown
}

// HTTPRequestError HTTP请求错误
type HTTPRequestError struct {
	*AppError
	HttpStatus int
}

// NewHTTPRequestError 创建一个HTTP请求错误
func NewHTTPRequestError(httpStatus int, message string, cause error) *HTTPRequestError {
	return &HTTPRequestError{
		AppError:   NewError(CodeHTTPError, fmt.Sprintf("http status:%d, %s", httpStatus, message)).WithCause(cause),
		HttpStatus: httpStatus,
	}
}

// Error 实现 error 接口，复用内部 AppError 的格式
func (e *HTTPRequestError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.AppError != nil {
		return e.AppError.Error()
	}
	return fmt.Sprintf("code=%d msg=%s", CodeHTTPError, "http request error")
}

// Unwrap 让 HTTPRequestError 在 errors.Is / errors.As 中表现为 AppError，
// 这样 FromError 等基于 errors.As 的工具可以把它当成 AppError 处理。
func (e *HTTPRequestError) Unwrap() error {
	if e == nil {
		return nil
	}
	if e.AppError != nil {
		return e.AppError
	}
	return nil
}

// 常用通用错误码定义（按需扩展）
const (
	CodeOK                 = 0
	CodeUnknown            = -1
	CodeInvalidArgument    = -400
	CodeUnauthorized       = -401
	CodeForbidden          = -403
	CodeNotFound           = -404
	CodeConflict           = -409
	CodeLocked             = -423
	CodeTooManyRequests    = -429
	CodeInternal           = -500
	CodeServiceUnavailable = -503
	CodeDeadlineExceeded   = -504
	CodeHTTPError          = -1021
)

// 语义化构造函数
func InvalidArgument(message string) *AppError {
	return NewError(CodeInvalidArgument, message)
}

func InvalidArgumentf(format string, a ...any) *AppError {
	return NewErrorf(CodeInvalidArgument, format, a...)
}

func Unauthorized(message string) *AppError {
	return NewError(CodeUnauthorized, message)
}

func Unauthorizedf(format string, a ...any) *AppError {
	return NewErrorf(CodeUnauthorized, format, a...)
}

func Forbidden(message string) *AppError {
	return NewError(CodeForbidden, message)
}

func Forbiddenf(format string, a ...any) *AppError {
	return NewErrorf(CodeForbidden, format, a...)
}

func NotFound(message string) *AppError {
	return NewError(CodeNotFound, message)
}

func NotFoundf(format string, a ...any) *AppError {
	return NewErrorf(CodeNotFound, format, a...)
}

func Conflict(message string) *AppError {
	return NewError(CodeConflict, message)
}

func Conflictf(format string, a ...any) *AppError {
	return NewErrorf(CodeConflict, format, a...)
}

func TooManyRequests(message string) *AppError {
	return NewError(CodeTooManyRequests, message)
}

func TooManyRequestsf(format string, a ...any) *AppError {
	return NewErrorf(CodeTooManyRequests, format, a...)
}

func Internal(message string) *AppError {
	return NewError(CodeInternal, message)
}

func Internalf(format string, a ...any) *AppError {
	return NewErrorf(CodeInternal, format, a...)
}

func ServiceUnavailable(message string) *AppError {
	return NewError(CodeServiceUnavailable, message)
}

func ServiceUnavailablef(format string, a ...any) *AppError {
	return NewErrorf(CodeServiceUnavailable, format, a...)
}

func DeadlineExceeded(message string) *AppError {
	return NewError(CodeDeadlineExceeded, message)
}

func DeadlineExceededf(format string, a ...any) *AppError {
	return NewErrorf(CodeDeadlineExceeded, format, a...)
}

// 预定义常用错误模板变量（请使用 Clone/WithMessage/WithCause/WithDetails 再定制，不要直接修改）
var (
	ErrInvalidArgument    = NewError(CodeInvalidArgument, "invalid argument")
	ErrUnauthorized       = NewError(CodeUnauthorized, "unauthorized")
	ErrForbidden          = NewError(CodeForbidden, "forbidden")
	ErrNotFound           = NewError(CodeNotFound, "not found")
	ErrConflict           = NewError(CodeConflict, "conflict")
	ErrTooManyRequests    = NewError(CodeTooManyRequests, "too many requests")
	ErrInternal           = NewError(CodeInternal, "internal error")
	ErrServiceUnavailable = NewError(CodeServiceUnavailable, "service unavailable")
	ErrDeadlineExceeded   = NewError(CodeDeadlineExceeded, "deadline exceeded")
)
