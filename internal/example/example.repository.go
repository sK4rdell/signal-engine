package example

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"betemplate/internal/platform/apperror"
	"betemplate/internal/platform/database"
)

// Repository persists examples. Every query is scoped by account_id so a
// caller can never reach another account's rows.
type Repository struct{}

// NewRepository returns the example repository.
func NewRepository() *Repository { return &Repository{} }

const columns = `id, account_id, title, note, created_at, updated_at`

func scan(row pgx.Row) (Example, error) {
	var e Example
	err := row.Scan(&e.ID, &e.AccountID, &e.Title, &e.Note, &e.CreatedAt, &e.UpdatedAt)
	return e, err
}

const createSQL = `
	INSERT INTO examples (id, account_id, title, note)
	VALUES ($1, $2, $3, $4)
	RETURNING ` + columns

// Create inserts an example owned by accountID.
func (r *Repository) Create(ctx context.Context, db database.DBTX, accountID uuid.UUID, title, note string) (Example, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return Example{}, fmt.Errorf("example: generate id: %w", err)
	}
	e, err := scan(db.QueryRow(ctx, createSQL, id, accountID, title, note))
	if err != nil {
		return Example{}, fmt.Errorf("example: create: %w", err)
	}
	return e, nil
}

const getSQL = `
	SELECT ` + columns + `
	FROM examples
	WHERE id = $1
	  AND account_id = $2
`

// Get returns one example of the account, or ErrNotFound.
func (r *Repository) Get(ctx context.Context, db database.DBTX, accountID, id uuid.UUID) (Example, error) {
	e, err := scan(db.QueryRow(ctx, getSQL, id, accountID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Example{}, ErrNotFound
	}
	if err != nil {
		return Example{}, fmt.Errorf("example: get: %w", err)
	}
	return e, nil
}

// Page is one page of a listing.
type Page struct {
	Items      []Example
	NextCursor string
}

const listSQL = `
	SELECT ` + columns + `
	FROM examples
	WHERE account_id = $1
	  AND ($2::timestamptz IS NULL OR (created_at, id) < ($2::timestamptz, $3::uuid))
	ORDER BY created_at DESC, id DESC
	LIMIT $4
`

// List returns the account's examples newest first, resuming after cursor.
// It fetches one row more than limit to know whether a next page exists.
func (r *Repository) List(ctx context.Context, db database.DBTX, accountID uuid.UUID, limit int, cursor string) (Page, error) {
	if limit < 1 {
		limit = DefaultLimit
	}
	if limit > MaxLimit {
		limit = MaxLimit
	}
	var afterAt *time.Time
	var afterID *uuid.UUID
	if cursor != "" {
		at, id, err := decodeCursor(cursor)
		if err != nil {
			return Page{}, err
		}
		afterAt, afterID = &at, &id
	}

	rows, err := db.Query(ctx, listSQL, accountID, afterAt, afterID, limit+1)
	if err != nil {
		return Page{}, fmt.Errorf("example: list: %w", err)
	}
	defer rows.Close()

	items := make([]Example, 0, limit)
	for rows.Next() {
		e, err := scan(rows)
		if err != nil {
			return Page{}, fmt.Errorf("example: scan: %w", err)
		}
		items = append(items, e)
	}
	if err := rows.Err(); err != nil {
		return Page{}, fmt.Errorf("example: read rows: %w", err)
	}

	page := Page{Items: items}
	if len(items) > limit {
		page.Items = items[:limit]
		last := page.Items[limit-1]
		page.NextCursor = encodeCursor(last.CreatedAt, last.ID)
	}
	return page, nil
}

// Patch carries the optional fields of an update. Nil means "leave as is".
type Patch struct {
	Title *string
	Note  *string
}

const updateSQL = `
	UPDATE examples
	SET title = COALESCE($3, title),
	    note = COALESCE($4, note),
	    updated_at = now()
	WHERE id = $1
	  AND account_id = $2
	RETURNING ` + columns

// Update applies patch to one example of the account, or ErrNotFound.
func (r *Repository) Update(ctx context.Context, db database.DBTX, accountID, id uuid.UUID, patch Patch) (Example, error) {
	e, err := scan(db.QueryRow(ctx, updateSQL, id, accountID, patch.Title, patch.Note))
	if errors.Is(err, pgx.ErrNoRows) {
		return Example{}, ErrNotFound
	}
	if err != nil {
		return Example{}, fmt.Errorf("example: update: %w", err)
	}
	return e, nil
}

const deleteSQL = `
	DELETE FROM examples
	WHERE id = $1
	  AND account_id = $2
`

// Delete removes one example of the account, or answers ErrNotFound.
func (r *Repository) Delete(ctx context.Context, db database.DBTX, accountID, id uuid.UUID) error {
	tag, err := db.Exec(ctx, deleteSQL, id, accountID)
	if err != nil {
		return fmt.Errorf("example: delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Cursors are opaque to clients: base64url of "<RFC3339Nano created_at>|<id>".

func encodeCursor(at time.Time, id uuid.UUID) string {
	return base64.RawURLEncoding.EncodeToString([]byte(at.UTC().Format(time.RFC3339Nano) + "|" + id.String()))
}

var errInvalidCursor = apperror.InvalidRequestFields("The request contains invalid parameters", map[string]string{"cursor": "is invalid"})

func decodeCursor(cursor string) (time.Time, uuid.UUID, error) {
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, uuid.Nil, errInvalidCursor
	}
	atPart, idPart, ok := strings.Cut(string(raw), "|")
	if !ok {
		return time.Time{}, uuid.Nil, errInvalidCursor
	}
	at, err := time.Parse(time.RFC3339Nano, atPart)
	if err != nil {
		return time.Time{}, uuid.Nil, errInvalidCursor
	}
	id, err := uuid.Parse(idPart)
	if err != nil {
		return time.Time{}, uuid.Nil, errInvalidCursor
	}
	return at, id, nil
}
