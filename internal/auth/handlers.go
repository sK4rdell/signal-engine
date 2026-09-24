package auth

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/sK4rdell/signal-engine/internal/platform/api"
)

// UserResponse is the public shape of a user.
type UserResponse struct {
	ID            uuid.UUID `json:"id"`
	Email         string    `json:"email"`
	EmailVerified bool      `json:"email_verified"`
	CreatedAt     time.Time `json:"created_at"`
}

func toUserResponse(u User) UserResponse {
	return UserResponse{
		ID:            u.ID,
		Email:         u.Email,
		EmailVerified: u.EmailVerifiedAt != nil,
		CreatedAt:     u.CreatedAt,
	}
}

// SessionResponse is returned by signup and login together with the
// session cookie.
type SessionResponse struct {
	User   UserResponse `json:"user"`
	cookie *http.Cookie
}

// ResponseCookies implements api.CookieResponse.
func (r SessionResponse) ResponseCookies() []*http.Cookie { return []*http.Cookie{r.cookie} }

// clearCookieResponse is the empty response of endpoints that end a session.
type clearCookieResponse struct {
	cookie *http.Cookie
}

// ResponseCookies implements api.CookieResponse.
func (r clearCookieResponse) ResponseCookies() []*http.Cookie { return []*http.Cookie{r.cookie} }

// SignupRequest is the body of POST /v1/auth/signup.
type SignupRequest struct {
	Email    string `json:"email" validate:"required,email,max=254"`
	Password string `json:"password" validate:"required,min=12,max=1024"`
}

// Signup handles POST /v1/auth/signup.
func Signup(s *Service) api.HandlerFunc[SignupRequest, SessionResponse] {
	return func(ctx context.Context, req SignupRequest) (SessionResponse, error) {
		result, err := s.Signup(ctx, req.Email, req.Password)
		if err != nil {
			return SessionResponse{}, err
		}
		return SessionResponse{
			User:   toUserResponse(result.User),
			cookie: s.SessionCookie(result.SessionToken, result.Session.ExpiresAt),
		}, nil
	}
}

// LoginRequest is the body of POST /v1/auth/login.
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email,max=254"`
	Password string `json:"password" validate:"required,max=1024"`
}

// Login handles POST /v1/auth/login.
func Login(s *Service) api.HandlerFunc[LoginRequest, SessionResponse] {
	return func(ctx context.Context, req LoginRequest) (SessionResponse, error) {
		result, err := s.Login(ctx, req.Email, req.Password)
		if err != nil {
			return SessionResponse{}, err
		}
		return SessionResponse{
			User:   toUserResponse(result.User),
			cookie: s.SessionCookie(result.SessionToken, result.Session.ExpiresAt),
		}, nil
	}
}

// Logout handles POST /v1/auth/logout: it revokes the current session.
func Logout(s *Service) api.HandlerFunc[struct{}, clearCookieResponse] {
	return func(ctx context.Context, _ struct{}) (clearCookieResponse, error) {
		p := api.MustPrincipal(ctx)
		if err := s.Logout(ctx, p.SessionID); err != nil {
			return clearCookieResponse{}, err
		}
		return clearCookieResponse{cookie: s.ClearSessionCookie()}, nil
	}
}

// LogoutAll handles POST /v1/auth/logout-all: it revokes every session.
func LogoutAll(s *Service) api.HandlerFunc[struct{}, clearCookieResponse] {
	return func(ctx context.Context, _ struct{}) (clearCookieResponse, error) {
		p := api.MustPrincipal(ctx)
		if err := s.LogoutAll(ctx, p.UserID); err != nil {
			return clearCookieResponse{}, err
		}
		return clearCookieResponse{cookie: s.ClearSessionCookie()}, nil
	}
}

// Me handles GET /v1/auth/me.
func Me(s *Service) api.HandlerFunc[struct{}, UserResponse] {
	return func(ctx context.Context, _ struct{}) (UserResponse, error) {
		p := api.MustPrincipal(ctx)
		user, err := s.GetUser(ctx, p.UserID)
		if err != nil {
			return UserResponse{}, err
		}
		return toUserResponse(user), nil
	}
}

// VerifyEmailRequest is the body of POST /v1/auth/verify-email.
type VerifyEmailRequest struct {
	Token string `json:"token" validate:"required,max=128"`
}

// VerifyEmail handles POST /v1/auth/verify-email.
func VerifyEmail(s *Service) api.HandlerFunc[VerifyEmailRequest, api.NoContent] {
	return func(ctx context.Context, req VerifyEmailRequest) (api.NoContent, error) {
		return api.NoContent{}, s.VerifyEmail(ctx, req.Token)
	}
}

// ResendVerification handles POST /v1/auth/resend-verification.
func ResendVerification(s *Service) api.HandlerFunc[struct{}, api.NoContent] {
	return func(ctx context.Context, _ struct{}) (api.NoContent, error) {
		p := api.MustPrincipal(ctx)
		return api.NoContent{}, s.ResendVerification(ctx, p.UserID)
	}
}

// ForgotPasswordRequest is the body of POST /v1/auth/forgot-password.
type ForgotPasswordRequest struct {
	Email string `json:"email" validate:"required,email,max=254"`
}

// ForgotPassword handles POST /v1/auth/forgot-password. It always answers
// 204 so the response does not reveal whether the email is registered.
func ForgotPassword(s *Service) api.HandlerFunc[ForgotPasswordRequest, api.NoContent] {
	return func(ctx context.Context, req ForgotPasswordRequest) (api.NoContent, error) {
		return api.NoContent{}, s.ForgotPassword(ctx, req.Email)
	}
}

// ResetPasswordRequest is the body of POST /v1/auth/reset-password.
type ResetPasswordRequest struct {
	Token    string `json:"token" validate:"required,max=128"`
	Password string `json:"password" validate:"required,min=12,max=1024"`
}

// ResetPassword handles POST /v1/auth/reset-password.
func ResetPassword(s *Service) api.HandlerFunc[ResetPasswordRequest, clearCookieResponse] {
	return func(ctx context.Context, req ResetPasswordRequest) (clearCookieResponse, error) {
		if err := s.ResetPassword(ctx, req.Token, req.Password); err != nil {
			return clearCookieResponse{}, err
		}
		return clearCookieResponse{cookie: s.ClearSessionCookie()}, nil
	}
}
