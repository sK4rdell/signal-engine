package auth

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"betemplate/internal/platform/api"
	"betemplate/internal/platform/config"
	"betemplate/internal/platform/ratelimit"
)

// RegisterRoutes mounts the /v1/auth endpoints. Every endpoint that can be
// abused (signup, login, verification, password reset) is rate limited per
// client IP with its own limiter.
func RegisterRoutes(r chi.Router, s *Service, cfg config.AuthConfig) {
	limited := func() api.Middleware {
		return api.RateLimit(ratelimit.New(cfg.RateLimit.Requests, cfg.RateLimit.Window))
	}
	authenticated := RequireSession(s)

	r.Route("/v1/auth", func(r chi.Router) {
		r.With(limited()).Post("/signup", api.Handle(http.StatusCreated, Signup(s)))
		r.With(limited()).Post("/login", api.Handle(http.StatusOK, Login(s)))
		r.With(limited()).Post("/verify-email", api.Handle(http.StatusNoContent, VerifyEmail(s)))
		r.With(limited()).Post("/forgot-password", api.Handle(http.StatusNoContent, ForgotPassword(s)))
		r.With(limited()).Post("/reset-password", api.Handle(http.StatusNoContent, ResetPassword(s)))

		r.Group(func(r chi.Router) {
			r.Use(authenticated)
			r.Get("/me", api.Handle(http.StatusOK, Me(s)))
			r.Post("/logout", api.Handle(http.StatusNoContent, Logout(s)))
			r.Post("/logout-all", api.Handle(http.StatusNoContent, LogoutAll(s)))
			r.With(limited()).Post("/resend-verification", api.Handle(http.StatusNoContent, ResendVerification(s)))
		})
	})
}
