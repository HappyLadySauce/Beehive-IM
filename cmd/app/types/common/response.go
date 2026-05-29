package common

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"k8s.io/klog/v2"
)

// BaseResponse is the stable JSON envelope returned by every API response.
// BaseResponse 是所有 API 响应复用的稳定 JSON 包装结构。
type BaseResponse struct {
	Code int         `json:"code"`
	Msg  string      `json:"message"`
	Data interface{} `json:"data"`
}

// AppError maps internal failures to safe public HTTP responses.
// AppError 将内部失败映射为安全的对外 HTTP 响应。
type AppError struct {
	HTTPStatus int
	Code       int
	Message    string
	Err        error
}

// Error returns the safe public message, never the wrapped internal cause.
// Error 返回安全的对外错误信息，不返回被包装的内部原因。
func (e *AppError) Error() string {
	return e.Message
}

// Unwrap returns the internal cause for logging and errors.Is / errors.As.
// Unwrap 返回内部原因，供日志记录和 errors.Is / errors.As 使用。
func (e *AppError) Unwrap() error {
	return e.Err
}

// NewBadRequest returns a 400 public error.
// NewBadRequest 返回 400 对外错误。
func NewBadRequest(message string, err error) *AppError {
	return newAppError(http.StatusBadRequest, message, err)
}

// NewUnauthorized returns a 401 public error.
// NewUnauthorized 返回 401 对外错误。
func NewUnauthorized(message string, err error) *AppError {
	return newAppError(http.StatusUnauthorized, message, err)
}

// NewForbidden returns a 403 public error.
// NewForbidden 返回 403 对外错误。
func NewForbidden(message string, err error) *AppError {
	return newAppError(http.StatusForbidden, message, err)
}

// NewConflict returns a 409 public error.
// NewConflict 返回 409 对外错误。
func NewConflict(message string, err error) *AppError {
	return newAppError(http.StatusConflict, message, err)
}

// NewNotFound returns a 404 public error.
// NewNotFound 返回 404 对外错误。
func NewNotFound(message string, err error) *AppError {
	return newAppError(http.StatusNotFound, message, err)
}

// NewInternal returns a 500 public error with a safe message and a logged cause.
// NewInternal 返回带安全信息和日志原因的 500 对外错误。
func NewInternal(message string, err error) *AppError {
	return newAppError(http.StatusInternalServerError, message, err)
}

func newAppError(status int, message string, err error) *AppError {
	return &AppError{
		HTTPStatus: status,
		Code:       status,
		Message:    message,
		Err:        err,
	}
}

// Success writes a successful response.
// Success 写入成功响应。
func Success(c *gin.Context, obj interface{}) {
	Response(c, nil, obj)
}

// Fail writes a safe failed response.
// Fail 写入安全的失败响应。
func Fail(c *gin.Context, err error) {
	Response(c, err, nil)
}

// Response writes the common JSON envelope with a real HTTP status.
// Response 使用真实 HTTP 状态码写入通用 JSON 包装结构。
func Response(c *gin.Context, err error, data interface{}) {
	status := http.StatusOK
	code := http.StatusOK
	message := "success"
	if err != nil {
		status, code, message = publicError(err)
		if status >= http.StatusInternalServerError || errors.Unwrap(err) != nil {
			klog.ErrorS(err, "Request failed", "status", status)
		}
	}
	c.JSON(status, BaseResponse{
		Code: code,
		Msg:  message,
		Data: data,
	})
}

func publicError(err error) (status int, code int, message string) {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.HTTPStatus, appErr.Code, appErr.Message
	}
	return http.StatusInternalServerError, http.StatusInternalServerError, "internal server error"
}
