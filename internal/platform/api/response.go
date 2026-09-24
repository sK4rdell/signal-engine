package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/sK4rdell/signal-engine/internal/platform/apperror"
	"github.com/sK4rdell/signal-engine/internal/platform/logging"
)

// ErrorResponse is the public error envelope.
type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

// ErrorBody is the public shape of an application error.
type ErrorBody struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

// WriteJSON encodes v as the response body with the given status.
//
// The body is encoded before any header is written so an encoding failure
// still produces a well-formed 500 instead of a truncated 200.
func WriteJSON(ctx context.Context, w http.ResponseWriter, status int, v any) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(v); err != nil {
		WriteError(ctx, w, apperror.Internal(fmt.Errorf("encode response: %w", err)))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}

// WriteNoContent responds 204 with no body.
func WriteNoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// WriteError converts err to its public representation and writes it.
//
// This is the single place where errors are logged for HTTP: unexpected
// errors (anything that is not an *apperror.Error, and every 5xx) are logged
// once at error level with their cause; expected client errors at debug
// level. The request log line written afterwards is an access log, not a
// second diagnostic.
func WriteError(ctx context.Context, w http.ResponseWriter, err error) {
	appErr := toAppError(err)

	logger := logging.FromContext(ctx)
	switch {
	case appErr.HTTPStatus >= 500:
		logger.Error("request failed",
			"code", appErr.Code,
			"status", appErr.HTTPStatus,
			"error", err,
		)
	default:
		logger.Debug("request rejected",
			"code", appErr.Code,
			"status", appErr.HTTPStatus,
			"error", err,
		)
	}
	writeErrorResponse(w, appErr)
}

// writeErrorResponse writes the public error body without logging. Callers
// that already logged the diagnostic (Recover) use it.
func writeErrorResponse(w http.ResponseWriter, appErr *apperror.Error) {
	body := ErrorResponse{Error: ErrorBody{
		Code:    appErr.Code,
		Message: appErr.Message,
		Fields:  appErr.Fields,
	}}
	var buf bytes.Buffer
	_ = json.NewEncoder(&buf).Encode(body)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(appErr.HTTPStatus)
	_, _ = w.Write(buf.Bytes())
}

// toAppError maps any error to a public application error. Context errors
// become a generic client-closed status; everything unknown is an internal
// error that never exposes its cause.
func toAppError(err error) *apperror.Error {
	if appErr, ok := apperror.As(err); ok {
		if appErr.HTTPStatus == 0 {
			return apperror.Internal(fmt.Errorf("application error without status: %w", err))
		}
		return appErr
	}
	if errors.Is(err, context.Canceled) {
		return apperror.New(499, "request_cancelled", "The request was cancelled")
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return apperror.New(http.StatusGatewayTimeout, "request_timeout", "The request timed out")
	}
	return apperror.Internal(err)
}
