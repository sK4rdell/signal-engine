// Package account owns accounts (workspaces) and their memberships. Product
// resources belong to an account, never directly to a user.
package account

import (
	"time"

	"github.com/google/uuid"

	"betemplate/internal/platform/apperror"
)

// Role is a membership role. The set is intentionally small.
type Role string

// Membership roles.
const (
	RoleOwner  Role = "owner"
	RoleMember Role = "member"
)

// Account is a workspace that owns product resources.
type Account struct {
	ID        uuid.UUID
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Membership links a user to an account with a role.
type Membership struct {
	AccountID uuid.UUID
	UserID    uuid.UUID
	Role      Role
	CreatedAt time.Time
}

// AccountWithRole is an account seen through one user's membership.
type AccountWithRole struct {
	Account
	Role Role
}

// Public error codes.
const (
	CodeMembershipRequired = "account_membership_required"
	CodeAccountNotFound    = "account_not_found"
	CodeAlreadyMember      = "already_member"
)

// Errors returned by this package.
var (
	ErrMembershipRequired = apperror.Forbidden(CodeMembershipRequired, "You are not a member of this account")
	ErrAccountNotFound    = apperror.NotFound(CodeAccountNotFound, "Account not found")
	ErrAlreadyMember      = apperror.Conflict(CodeAlreadyMember, "User is already a member of this account")
)
