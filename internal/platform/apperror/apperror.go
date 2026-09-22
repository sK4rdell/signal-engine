// Package apperror defines the application error type shared by every layer.
//
// An Error carries what the client may see (code, message, HTTP status,
// field errors) separately from what only the logs may see (the cause). The
// HTTP boundary converts errors to responses; feature code just returns them.
package apperror

import (
	"errors"
	"fmt"
	"net/http"
)

// Stable public error codes shared across features. Feature packages define
// their own codes (for example "email_already_registered") next to the code
// that produces them.
const (
	CodeInvalidRequest   = "invalid_request"
	CodeValidationFailed = "validation_failed"
	CodeUnauthorized     = "unauthorized"
	CodeForbidden        = "forbidden"
	CodeNotFound         = "not_found"
	CodeConflict         = "conflict"
	CodeRateLimited      = "rate_limited"
	CodeRequestTooLarge  = "request_too_large"
	CodeInternal         = "internal_error"
)

// Error is an application error with a stable public representation.
type Error struct {
	// Code is the machine-readable public error code.
	Code string
	// Message is the human-readable public message.
	Message string
	// HTTPStatus is the status the HTTP boundary responds with.
	HTTPStatus int
	// Cause is the internal cause. It is logged, never sent to clients.
	Cause error
	// Fields holds per-field messages for validation failures.
	Fields map[string]string
}

// Error implements error. The cause is included so logs stay informative.
func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap exposes the cause to errors.Is and errors.As.
func (e *Error) Unwrap() error { return e.Cause }

// WithCause returns a copy of e carrying cause.
func (e *Error) WithCause(cause error) *Error {
	clone := *e
	clone.Cause = cause
	return &clone
}

// WithMessage returns a copy of e with a different public message.
func (e *Error) WithMessage(message string) *Error {
	clone := *e
	clone.Message = message
	return &clone
}

// New builds an error with an explicit status, code and message.
func New(httpStatus int, code, message string) *Error {
	return &Error{Code: code, Message: message, HTTPStatus: httpStatus}
}

// InvalidRequest is a 400: the request is malformed.
func InvalidRequest(message string) *Error {
	return New(http.StatusBadRequest, CodeInvalidRequest, message)
}

// InvalidRequestFields is a 400 with per-field details, used for binding
// failures such as a malformed path parameter.
func InvalidRequestFields(message string, fields map[string]string) *Error {
	e := InvalidRequest(message)
	e.Fields = fields
	return e
}

// Validation is a 422: the request is well-formed but fields are invalid.
func Validation(fields map[string]string) *Error {
	e := New(http.StatusUnprocessableEntity, CodeValidationFailed, "The request contains invalid fields")
	e.Fields = fields
	return e
}

// Unauthorized is a 401 with the shared unauthorized code.
func Unauthorized(message string) *Error {
	return New(http.StatusUnauthorized, CodeUnauthorized, message)
}

// Forbidden is a 403.
func Forbidden(code, message string) *Error {
	return New(http.StatusForbidden, code, message)
}

// NotFound is a 404.
func NotFound(code, message string) *Error {
	return New(http.StatusNotFound, code, message)
}

// Conflict is a 409.
func Conflict(code, message string) *Error {
	return New(http.StatusConflict, code, message)
}

// RateLimited is a 429.
func RateLimited() *Error {
	return New(http.StatusTooManyRequests, CodeRateLimited, "Too many requests, try again later")
}

// RequestTooLarge is a 413.
func RequestTooLarge() *Error {
	return New(http.StatusRequestEntityTooLarge, CodeRequestTooLarge, "The request body is too large")
}

// Internal is a 500 wrapping an unexpected cause. Clients see only the
// generic message.
func Internal(cause error) *Error {
	return &Error{
		Code:       CodeInternal,
		Message:    "An unexpected error occurred",
		HTTPStatus: http.StatusInternalServerError,
		Cause:      cause,
	}
}

// As extracts an *Error from err's chain.
func As(err error) (*Error, bool) {
	var e *Error
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}

// HasCode reports whether err's chain contains an application error with code.
func HasCode(err error, code string) bool {
	e, ok := As(err)
	return ok && e.Code == code
}
