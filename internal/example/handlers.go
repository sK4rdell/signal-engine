package example

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sK4rdell/signal-engine/internal/account"
	"github.com/sK4rdell/signal-engine/internal/platform/api"
)

// Response is the public shape of an example.
type Response struct {
	ID        uuid.UUID `json:"id"`
	AccountID uuid.UUID `json:"account_id"`
	Title     string    `json:"title"`
	Note      string    `json:"note"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func toResponse(e Example) Response {
	return Response(e)
}

// CreateRequest is the body of POST /v1/accounts/{accountID}/examples.
type CreateRequest struct {
	Title string `json:"title" validate:"required,min=1,max=200"`
	Note  string `json:"note" validate:"max=2000"`
}

// Create handles POST /v1/accounts/{accountID}/examples.
func Create(repo *Repository, pool *pgxpool.Pool) api.HandlerFunc[CreateRequest, Response] {
	return func(ctx context.Context, req CreateRequest) (Response, error) {
		acc := account.MustFromContext(ctx)
		e, err := repo.Create(ctx, pool, acc.AccountID, req.Title, req.Note)
		if err != nil {
			return Response{}, err
		}
		return toResponse(e), nil
	}
}

// ListRequest binds the query of GET /v1/accounts/{accountID}/examples.
type ListRequest struct {
	Limit  int    `query:"limit" json:"-" validate:"omitempty,min=1,max=100"`
	Cursor string `query:"cursor" json:"-" validate:"omitempty,max=200"`
}

// ListResponse is the body of the listing.
type ListResponse struct {
	Items      []Response `json:"items"`
	NextCursor *string    `json:"next_cursor"`
}

// List handles GET /v1/accounts/{accountID}/examples.
func List(repo *Repository, pool *pgxpool.Pool) api.HandlerFunc[ListRequest, ListResponse] {
	return func(ctx context.Context, req ListRequest) (ListResponse, error) {
		acc := account.MustFromContext(ctx)
		page, err := repo.List(ctx, pool, acc.AccountID, req.Limit, req.Cursor)
		if err != nil {
			return ListResponse{}, err
		}
		res := ListResponse{Items: make([]Response, 0, len(page.Items))}
		for _, e := range page.Items {
			res.Items = append(res.Items, toResponse(e))
		}
		if page.NextCursor != "" {
			res.NextCursor = &page.NextCursor
		}
		return res, nil
	}
}

// GetRequest binds GET /v1/accounts/{accountID}/examples/{exampleID}.
type GetRequest struct {
	ExampleID uuid.UUID `path:"exampleID" json:"-" validate:"required"`
}

// Get handles GET /v1/accounts/{accountID}/examples/{exampleID}.
func Get(repo *Repository, pool *pgxpool.Pool) api.HandlerFunc[GetRequest, Response] {
	return func(ctx context.Context, req GetRequest) (Response, error) {
		acc := account.MustFromContext(ctx)
		e, err := repo.Get(ctx, pool, acc.AccountID, req.ExampleID)
		if err != nil {
			return Response{}, err
		}
		return toResponse(e), nil
	}
}

// UpdateRequest binds PATCH /v1/accounts/{accountID}/examples/{exampleID}.
// Pointer fields distinguish "absent" from "set to empty".
type UpdateRequest struct {
	ExampleID uuid.UUID `path:"exampleID" json:"-" validate:"required"`
	Title     *string   `json:"title" validate:"omitempty,min=1,max=200"`
	Note      *string   `json:"note" validate:"omitempty,max=2000"`
}

// Update handles PATCH /v1/accounts/{accountID}/examples/{exampleID}.
func Update(repo *Repository, pool *pgxpool.Pool) api.HandlerFunc[UpdateRequest, Response] {
	return func(ctx context.Context, req UpdateRequest) (Response, error) {
		acc := account.MustFromContext(ctx)
		e, err := repo.Update(ctx, pool, acc.AccountID, req.ExampleID, Patch{Title: req.Title, Note: req.Note})
		if err != nil {
			return Response{}, err
		}
		return toResponse(e), nil
	}
}

// DeleteRequest binds DELETE /v1/accounts/{accountID}/examples/{exampleID}.
type DeleteRequest struct {
	ExampleID uuid.UUID `path:"exampleID" json:"-" validate:"required"`
}

// Delete handles DELETE /v1/accounts/{accountID}/examples/{exampleID}.
func Delete(repo *Repository, pool *pgxpool.Pool) api.HandlerFunc[DeleteRequest, api.NoContent] {
	return func(ctx context.Context, req DeleteRequest) (api.NoContent, error) {
		acc := account.MustFromContext(ctx)
		return api.NoContent{}, repo.Delete(ctx, pool, acc.AccountID, req.ExampleID)
	}
}

// RegisterRoutes mounts the example endpoints behind authentication and
// account membership.
func RegisterRoutes(r chi.Router, repo *Repository, pool *pgxpool.Pool, authenticated, member api.Middleware) {
	r.Route("/v1/accounts/{accountID}/examples", func(r chi.Router) {
		r.Use(authenticated, member)
		r.Post("/", api.Handle(http.StatusCreated, Create(repo, pool)))
		r.Get("/", api.Handle(http.StatusOK, List(repo, pool)))
		r.Get("/{exampleID}", api.Handle(http.StatusOK, Get(repo, pool)))
		r.Patch("/{exampleID}", api.Handle(http.StatusOK, Update(repo, pool)))
		r.Delete("/{exampleID}", api.Handle(http.StatusNoContent, Delete(repo, pool)))
	})
}
