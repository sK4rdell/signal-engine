package account

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"betemplate/internal/platform/database"
)

// Repository persists accounts and memberships. Every method takes the DBTX
// to run on so callers choose between the pool and a transaction.
type Repository struct{}

// NewRepository returns the account repository.
func NewRepository() *Repository { return &Repository{} }

const createAccountSQL = `
	INSERT INTO accounts (id, name)
	VALUES ($1, $2)
	RETURNING id, name, created_at, updated_at
`

// Create inserts a new account.
func (r *Repository) Create(ctx context.Context, db database.DBTX, name string) (Account, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return Account{}, fmt.Errorf("account: generate id: %w", err)
	}
	var a Account
	err = db.QueryRow(ctx, createAccountSQL, id, name).Scan(&a.ID, &a.Name, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return Account{}, fmt.Errorf("account: create: %w", err)
	}
	return a, nil
}

const addMembershipSQL = `
	INSERT INTO account_memberships (account_id, user_id, role)
	VALUES ($1, $2, $3)
	RETURNING account_id, user_id, role, created_at
`

// AddMembership makes user a member of account with role.
func (r *Repository) AddMembership(ctx context.Context, db database.DBTX, accountID, userID uuid.UUID, role Role) (Membership, error) {
	var m Membership
	err := db.QueryRow(ctx, addMembershipSQL, accountID, userID, role).Scan(&m.AccountID, &m.UserID, &m.Role, &m.CreatedAt)
	if err != nil {
		if database.IsUniqueViolation(err, "account_memberships_pkey") {
			return Membership{}, ErrAlreadyMember
		}
		return Membership{}, fmt.Errorf("account: add membership: %w", err)
	}
	return m, nil
}

const getMembershipSQL = `
	SELECT account_id, user_id, role, created_at
	FROM account_memberships
	WHERE account_id = $1
	  AND user_id = $2
`

// GetMembership returns the user's membership in account, or
// ErrMembershipRequired when there is none.
func (r *Repository) GetMembership(ctx context.Context, db database.DBTX, accountID, userID uuid.UUID) (Membership, error) {
	var m Membership
	err := db.QueryRow(ctx, getMembershipSQL, accountID, userID).Scan(&m.AccountID, &m.UserID, &m.Role, &m.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Membership{}, ErrMembershipRequired
	}
	if err != nil {
		return Membership{}, fmt.Errorf("account: get membership: %w", err)
	}
	return m, nil
}

const getForUserSQL = `
	SELECT a.id, a.name, a.created_at, a.updated_at, m.role
	FROM accounts a
	JOIN account_memberships m ON m.account_id = a.id
	WHERE a.id = $1
	  AND m.user_id = $2
`

// GetForUser returns an account the user is a member of. Accounts the user
// does not belong to answer ErrAccountNotFound, so their existence is not
// revealed.
func (r *Repository) GetForUser(ctx context.Context, db database.DBTX, accountID, userID uuid.UUID) (AccountWithRole, error) {
	var a AccountWithRole
	err := db.QueryRow(ctx, getForUserSQL, accountID, userID).Scan(&a.ID, &a.Name, &a.CreatedAt, &a.UpdatedAt, &a.Role)
	if errors.Is(err, pgx.ErrNoRows) {
		return AccountWithRole{}, ErrAccountNotFound
	}
	if err != nil {
		return AccountWithRole{}, fmt.Errorf("account: get for user: %w", err)
	}
	return a, nil
}

const listForUserSQL = `
	SELECT a.id, a.name, a.created_at, a.updated_at, m.role
	FROM accounts a
	JOIN account_memberships m ON m.account_id = a.id
	WHERE m.user_id = $1
	ORDER BY a.created_at, a.id
`

// ListForUser returns every account the user is a member of.
func (r *Repository) ListForUser(ctx context.Context, db database.DBTX, userID uuid.UUID) ([]AccountWithRole, error) {
	rows, err := db.Query(ctx, listForUserSQL, userID)
	if err != nil {
		return nil, fmt.Errorf("account: list for user: %w", err)
	}
	defer rows.Close()

	accounts := []AccountWithRole{}
	for rows.Next() {
		var a AccountWithRole
		if err := rows.Scan(&a.ID, &a.Name, &a.CreatedAt, &a.UpdatedAt, &a.Role); err != nil {
			return nil, fmt.Errorf("account: scan: %w", err)
		}
		accounts = append(accounts, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("account: read rows: %w", err)
	}
	return accounts, nil
}
