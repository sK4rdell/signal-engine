package auth

import (
	"errors"
	"net/http"

	"betemplate/internal/platform/api"
	"betemplate/internal/platform/logging"
)

// RequireSession authenticates requests from the session cookie. Requests
// without a live session answer 401 unauthorized. On success the principal
// is stored in the context and user_id joins the request log line.
func RequireSession(s *Service) api.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(s.CookieName())
			if err != nil || cookie.Value == "" {
				api.WriteError(r.Context(), w, ErrUnauthenticated)
				return
			}
			sess, err := s.ResolveSession(r.Context(), cookie.Value)
			if err != nil {
				if errors.Is(err, ErrSessionNotFound) {
					api.WriteError(r.Context(), w, ErrUnauthenticated.WithCause(err))
					return
				}
				api.WriteError(r.Context(), w, err)
				return
			}
			ctx := api.WithPrincipal(r.Context(), api.Principal{UserID: sess.UserID, SessionID: sess.ID})
			logging.Add(ctx, "user_id", sess.UserID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
