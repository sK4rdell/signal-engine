// Package auth owns identity: users, sessions, email verification and
// password reset. It exposes the authentication middleware and the
// /v1/auth endpoints.
package auth

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"betemplate/internal/platform/apperror"
)

// User is a registered identity. PasswordHash never leaves this package.
type User struct {
	ID              uuid.UUID
	Email           string
	EmailNormalized string
	PasswordHash    string
	EmailVerifiedAt *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// Session is a server-side browser session. Only the hash of the opaque
// token is stored.
type Session struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	ExpiresAt  time.Time
	CreatedAt  time.Time
	LastSeenAt time.Time
	RevokedAt  *time.Time
}

// Public error codes owned by this package.
const (
	CodeEmailAlreadyRegistered = "email_already_registered"
	CodeInvalidCredentials     = "invalid_credentials"
	CodeInvalidToken           = "invalid_token"
	CodeEmailAlreadyVerified   = "email_already_verified"
)

// Application errors returned to clients.
var (
	ErrEmailAlreadyRegistered = apperror.Conflict(CodeEmailAlreadyRegistered, "An account with this email already exists")
	ErrInvalidCredentials     = apperror.New(http.StatusUnauthorized, CodeInvalidCredentials, "Invalid email or password")
	ErrInvalidToken           = apperror.New(http.StatusUnprocessableEntity, CodeInvalidToken, "The token is invalid or has expired")
	ErrEmailAlreadyVerified   = apperror.Conflict(CodeEmailAlreadyVerified, "The email address is already verified")
	ErrUnauthenticated        = apperror.Unauthorized("Authentication required")
)

// Internal sentinels, translated by services before reaching clients.
var (
	ErrUserNotFound    = errors.New("auth: user not found")
	ErrSessionNotFound = errors.New("auth: session not found")
)

// NormalizeEmail produces the canonical form used for uniqueness and
// lookups: trimmed and lower-cased. The display form is kept as entered.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
