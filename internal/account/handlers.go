package account

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"betemplate/internal/platform/api"
)

// AccountResponse is the public shape of an account as seen by a member.
type AccountResponse struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Role      Role      `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

func toResponse(a AccountWithRole) AccountResponse {
	return AccountResponse{ID: a.ID, Name: a.Name, Role: a.Role, CreatedAt: a.CreatedAt}
}

// ListResponse is the body of GET /v1/accounts.
type ListResponse struct {
	Items []AccountResponse `json:"items"`
}

// List handles GET /v1/accounts: the caller's accounts.
func List(repo *Repository, pool *pgxpool.Pool) api.HandlerFunc[struct{}, ListResponse] {
	return func(ctx context.Context, _ struct{}) (ListResponse, error) {
		p := api.MustPrincipal(ctx)
		accounts, err := repo.ListForUser(ctx, pool, p.UserID)
		if err != nil {
			return ListResponse{}, err
		}
		items := make([]AccountResponse, 0, len(accounts))
		for _, a := range accounts {
			items = append(items, toResponse(a))
		}
		return ListResponse{Items: items}, nil
	}
}

// GetRequest binds GET /v1/accounts/{accountID}.
type GetRequest struct {
	AccountID uuid.UUID `path:"accountID" json:"-" validate:"required"`
}

// Get handles GET /v1/accounts/{accountID}.
func Get(repo *Repository, pool *pgxpool.Pool) api.HandlerFunc[GetRequest, AccountResponse] {
	return func(ctx context.Context, req GetRequest) (AccountResponse, error) {
		p := api.MustPrincipal(ctx)
		a, err := repo.GetForUser(ctx, pool, req.AccountID, p.UserID)
		if err != nil {
			return AccountResponse{}, err
		}
		return toResponse(a), nil
	}
}

// RegisterRoutes mounts the /v1/accounts endpoints behind authenticated.
func RegisterRoutes(r chi.Router, repo *Repository, pool *pgxpool.Pool, authenticated api.Middleware) {
	r.Route("/v1/accounts", func(r chi.Router) {
		r.Use(authenticated)
		r.Get("/", api.Handle(http.StatusOK, List(repo, pool)))
		r.Get("/{accountID}", api.Handle(http.StatusOK, Get(repo, pool)))
	})
}
