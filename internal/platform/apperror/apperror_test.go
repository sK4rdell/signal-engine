package apperror

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
)

func TestError_PreservesCause(t *testing.T) {
	cause := errors.New("connection refused")
	err := Internal(cause)

	wrapped := fmt.Errorf("service: %w", err)

	got, ok := As(wrapped)
	if !ok {
		t.Fatal("As should find the application error")
	}
	if got.HTTPStatus != http.StatusInternalServerError || got.Code != CodeInternal {
		t.Errorf("unexpected error %+v", got)
	}
	if !errors.Is(wrapped, cause) {
		t.Error("cause should be reachable through errors.Is")
	}
	if got.Message != "An unexpected error occurred" {
		t.Errorf("Message = %q", got.Message)
	}
}

func TestWithCause_DoesNotMutateOriginal(t *testing.T) {
	base := NotFound("thing_not_found", "Thing not found")
	withCause := base.WithCause(errors.New("no rows"))
	if base.Cause != nil {
		t.Error("original error must not be mutated")
	}
	if withCause.Cause == nil || withCause.Code != base.Code {
		t.Errorf("unexpected copy %+v", withCause)
	}
}

func TestHasCode(t *testing.T) {
	err := fmt.Errorf("wrap: %w", Conflict("email_already_registered", "Email already registered"))
	if !HasCode(err, "email_already_registered") {
		t.Error("expected code match")
	}
	if HasCode(err, CodeNotFound) {
		t.Error("unexpected code match")
	}
	if HasCode(errors.New("plain"), CodeNotFound) {
		t.Error("plain errors have no code")
	}
}

func TestConstructors(t *testing.T) {
	tests := []struct {
		err    *Error
		status int
		code   string
	}{
		{InvalidRequest("x"), 400, CodeInvalidRequest},
		{Validation(map[string]string{"a": "b"}), 422, CodeValidationFailed},
		{Unauthorized("x"), 401, CodeUnauthorized},
		{Forbidden(CodeForbidden, "x"), 403, CodeForbidden},
		{NotFound(CodeNotFound, "x"), 404, CodeNotFound},
		{Conflict(CodeConflict, "x"), 409, CodeConflict},
		{RateLimited(), 429, CodeRateLimited},
		{RequestTooLarge(), 413, CodeRequestTooLarge},
		{Internal(nil), 500, CodeInternal},
	}
	for _, tc := range tests {
		if tc.err.HTTPStatus != tc.status || tc.err.Code != tc.code {
			t.Errorf("%+v: want status %d code %s", tc.err, tc.status, tc.code)
		}
		if tc.err.Error() == "" {
			t.Error("Error() must not be empty")
		}
	}
}
