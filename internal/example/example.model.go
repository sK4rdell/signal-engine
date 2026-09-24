// Package example is a deliberately small feature that demonstrates the
// template's conventions end to end: an account-scoped resource with
// create, list (cursor pagination), get, update (PATCH presence semantics)
// and delete, all behind authentication and membership checks.
//
// To remove it when starting a real product: delete this package, the
// examples migration and the registration line in internal/app.
package example

import (
	"time"

	"github.com/google/uuid"

	"github.com/sK4rdell/signal-engine/internal/platform/apperror"
)

// Example is the persisted resource.
type Example struct {
	ID        uuid.UUID
	AccountID uuid.UUID
	Title     string
	Note      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// CodeNotFound is answered for examples outside the caller's account as
// well as for unknown IDs, so existence across accounts is never revealed.
const CodeNotFound = "example_not_found"

// ErrNotFound is the tenant-safe not-found error.
var ErrNotFound = apperror.NotFound(CodeNotFound, "Example not found")

// Page-size bounds for listing.
const (
	DefaultLimit = 20
	MaxLimit     = 100
)
