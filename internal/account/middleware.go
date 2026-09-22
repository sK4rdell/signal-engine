package account

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"betemplate/internal/platform/api"
	"betemplate/internal/platform/apperror"
	"betemplate/internal/platform/logging"
)

// Context is the resolved account of an account-scoped request.
type Context struct {
	AccountID uuid.UUID
	Role      Role
}

type ctxKey struct{}

// FromContext returns the account context set by RequireMembership.
func FromContext(ctx context.Context) (Context, bool) {
	c, ok := ctx.Value(ctxKey{}).(Context)
	return c, ok
}

// MustFromContext returns the account context and panics when the route is
// not behind RequireMembership: a programming error caught by the first
// request in tests.
func MustFromContext(ctx context.Context) Context {
	c, ok := FromContext(ctx)
	if !ok {
		panic("account: handler requires an account context but the route is not behind RequireMembership")
	}
	return c
}

// RequireMembership resolves the {accountID} route parameter, verifies that
// the authenticated user is a member and stores the account context. It
// must run after the authentication middleware. Non-members get 403
// account_membership_required; account IDs are not secrets.
func RequireMembership(repo *Repository, pool *pgxpool.Pool) api.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p := api.MustPrincipal(r.Context())
			accountID, err := uuid.Parse(chi.URLParam(r, "accountID"))
			if err != nil {
				api.WriteError(r.Context(), w, apperror.InvalidRequestFields("The request contains invalid parameters", map[string]string{"accountID": "must be a valid UUID"}))
				return
			}
			m, err := repo.GetMembership(r.Context(), pool, accountID, p.UserID)
			if err != nil {
				api.WriteError(r.Context(), w, err)
				return
			}
			ctx := context.WithValue(r.Context(), ctxKey{}, Context{AccountID: m.AccountID, Role: m.Role})
			logging.Add(ctx, "account_id", m.AccountID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
