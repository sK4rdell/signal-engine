package api

import (
	"context"

	"github.com/google/uuid"
)

// Principal is the authenticated identity attached to a request by the
// authentication middleware. It carries only what most handlers need.
type Principal struct {
	UserID    uuid.UUID
	SessionID uuid.UUID
}

type principalKey struct{}

// WithPrincipal returns a context carrying p.
func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

// PrincipalFromContext returns the request principal, if authenticated.
func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(Principal)
	return p, ok
}

// MustPrincipal returns the principal and panics when the route is not
// behind the authentication middleware. That is a programming error caught
// by the first request in tests, never a runtime condition.
func MustPrincipal(ctx context.Context) Principal {
	p, ok := PrincipalFromContext(ctx)
	if !ok {
		panic("api: handler requires an authenticated principal but the route is not protected")
	}
	return p
}
