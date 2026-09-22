package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"betemplate/internal/platform/database"
)

// Repository persists users, sessions and security tokens. Every method
// takes the DBTX to run on so services choose pool or transaction.
type Repository struct{}

// NewRepository returns the auth repository.
func NewRepository() *Repository { return &Repository{} }

// --- users -----------------------------------------------------------------

const userColumns = `id, email, email_normalized, password_hash, email_verified_at, created_at, updated_at`

func scanUser(row pgx.Row) (User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Email, &u.EmailNormalized, &u.PasswordHash, &u.EmailVerifiedAt, &u.CreatedAt, &u.UpdatedAt)
	return u, err
}

const createUserSQL = `
	INSERT INTO users (id, email, email_normalized, password_hash)
	VALUES ($1, $2, $3, $4)
	RETURNING ` + userColumns

// CreateUser inserts a user. The email is stored as entered and normalized
// for uniqueness; a normalized duplicate answers ErrEmailAlreadyRegistered.
func (r *Repository) CreateUser(ctx context.Context, db database.DBTX, email, passwordHash string) (User, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return User{}, fmt.Errorf("auth: generate id: %w", err)
	}
	u, err := scanUser(db.QueryRow(ctx, createUserSQL, id, email, NormalizeEmail(email), passwordHash))
	if err != nil {
		if database.IsUniqueViolation(err, "users_email_normalized_key") {
			return User{}, ErrEmailAlreadyRegistered
		}
		return User{}, fmt.Errorf("auth: create user: %w", err)
	}
	return u, nil
}

const getUserByEmailSQL = `SELECT ` + userColumns + ` FROM users WHERE email_normalized = $1`

// GetUserByEmail looks a user up by normalized email.
func (r *Repository) GetUserByEmail(ctx context.Context, db database.DBTX, email string) (User, error) {
	u, err := scanUser(db.QueryRow(ctx, getUserByEmailSQL, NormalizeEmail(email)))
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrUserNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("auth: get user by email: %w", err)
	}
	return u, nil
}

const getUserByIDSQL = `SELECT ` + userColumns + ` FROM users WHERE id = $1`

// GetUserByID looks a user up by ID.
func (r *Repository) GetUserByID(ctx context.Context, db database.DBTX, id uuid.UUID) (User, error) {
	u, err := scanUser(db.QueryRow(ctx, getUserByIDSQL, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrUserNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("auth: get user by id: %w", err)
	}
	return u, nil
}

const markEmailVerifiedSQL = `
	UPDATE users
	SET email_verified_at = $2, updated_at = now()
	WHERE id = $1
	  AND email_verified_at IS NULL
`

// MarkEmailVerified records verification. It reports false when the user was
// already verified.
func (r *Repository) MarkEmailVerified(ctx context.Context, db database.DBTX, userID uuid.UUID, at time.Time) (bool, error) {
	tag, err := db.Exec(ctx, markEmailVerifiedSQL, userID, at)
	if err != nil {
		return false, fmt.Errorf("auth: mark email verified: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

const updatePasswordHashSQL = `
	UPDATE users
	SET password_hash = $2, updated_at = now()
	WHERE id = $1
`

// UpdatePasswordHash replaces the user's password hash.
func (r *Repository) UpdatePasswordHash(ctx context.Context, db database.DBTX, userID uuid.UUID, hash string) error {
	tag, err := db.Exec(ctx, updatePasswordHashSQL, userID, hash)
	if err != nil {
		return fmt.Errorf("auth: update password: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return ErrUserNotFound
	}
	return nil
}

// --- sessions --------------------------------------------------------------

const sessionColumns = `id, user_id, expires_at, created_at, last_seen_at, revoked_at`

func scanSession(row pgx.Row) (Session, error) {
	var s Session
	err := row.Scan(&s.ID, &s.UserID, &s.ExpiresAt, &s.CreatedAt, &s.LastSeenAt, &s.RevokedAt)
	return s, err
}

const createSessionSQL = `
	INSERT INTO sessions (id, user_id, token_hash, expires_at)
	VALUES ($1, $2, $3, $4)
	RETURNING ` + sessionColumns

// CreateSession creates a session and returns the raw token to hand to the
// client. The raw token is not stored.
func (r *Repository) CreateSession(ctx context.Context, db database.DBTX, userID uuid.UUID, expiresAt time.Time) (Session, string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return Session{}, "", fmt.Errorf("auth: generate id: %w", err)
	}
	raw, hash, err := newToken()
	if err != nil {
		return Session{}, "", err
	}
	s, err := scanSession(db.QueryRow(ctx, createSessionSQL, id, userID, hash, expiresAt))
	if err != nil {
		return Session{}, "", fmt.Errorf("auth: create session: %w", err)
	}
	return s, raw, nil
}

const getSessionByTokenSQL = `
	SELECT ` + sessionColumns + `
	FROM sessions
	WHERE token_hash = $1
	  AND revoked_at IS NULL
	  AND expires_at > $2
`

// GetSessionByToken resolves a live session from a raw token. Expired and
// revoked sessions answer ErrSessionNotFound.
func (r *Repository) GetSessionByToken(ctx context.Context, db database.DBTX, raw string, now time.Time) (Session, error) {
	s, err := scanSession(db.QueryRow(ctx, getSessionByTokenSQL, hashToken(raw), now))
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrSessionNotFound
	}
	if err != nil {
		return Session{}, fmt.Errorf("auth: get session: %w", err)
	}
	return s, nil
}

const touchSessionSQL = `
	UPDATE sessions
	SET last_seen_at = $2
	WHERE id = $1
	  AND last_seen_at < $2::timestamptz - $3::interval
`

// TouchSession advances last_seen_at when it is older than minAge, so a
// busy session does not write on every request.
func (r *Repository) TouchSession(ctx context.Context, db database.DBTX, id uuid.UUID, now time.Time, minAge time.Duration) error {
	if _, err := db.Exec(ctx, touchSessionSQL, id, now, minAge); err != nil {
		return fmt.Errorf("auth: touch session: %w", err)
	}
	return nil
}

const revokeSessionSQL = `
	UPDATE sessions
	SET revoked_at = $2
	WHERE id = $1
	  AND revoked_at IS NULL
`

// RevokeSession revokes one session. Revoking an already revoked or unknown
// session is not an error.
func (r *Repository) RevokeSession(ctx context.Context, db database.DBTX, id uuid.UUID, now time.Time) error {
	if _, err := db.Exec(ctx, revokeSessionSQL, id, now); err != nil {
		return fmt.Errorf("auth: revoke session: %w", err)
	}
	return nil
}

const revokeUserSessionsSQL = `
	UPDATE sessions
	SET revoked_at = $2
	WHERE user_id = $1
	  AND revoked_at IS NULL
`

// RevokeUserSessions revokes every live session of a user and returns how
// many were revoked.
func (r *Repository) RevokeUserSessions(ctx context.Context, db database.DBTX, userID uuid.UUID, now time.Time) (int64, error) {
	tag, err := db.Exec(ctx, revokeUserSessionsSQL, userID, now)
	if err != nil {
		return 0, fmt.Errorf("auth: revoke user sessions: %w", err)
	}
	return tag.RowsAffected(), nil
}

const countLiveSessionsSQL = `
	SELECT count(*)
	FROM sessions
	WHERE user_id = $1
	  AND revoked_at IS NULL
	  AND expires_at > $2
`

// CountLiveSessions returns the number of usable sessions for a user.
func (r *Repository) CountLiveSessions(ctx context.Context, db database.DBTX, userID uuid.UUID, now time.Time) (int, error) {
	var n int
	if err := db.QueryRow(ctx, countLiveSessionsSQL, userID, now).Scan(&n); err != nil {
		return 0, fmt.Errorf("auth: count sessions: %w", err)
	}
	return n, nil
}

// --- security tokens -------------------------------------------------------

// tokenKind selects the table a security token lives in. Both tables share
// the same shape; the kind keeps verification and reset tokens apart so one
// can never be used as the other.
type tokenKind string

const (
	verificationToken tokenKind = "email_verification_tokens"
	resetToken        tokenKind = "password_reset_tokens"
)

// CreateVerificationToken issues a new email verification token and retires
// any outstanding one for the user, so exactly one link is valid at a time.
// It must run inside the caller's transaction: issuance is serialised per
// user by locking the user row until the transaction ends.
func (r *Repository) CreateVerificationToken(ctx context.Context, tx pgx.Tx, userID uuid.UUID, now, expiresAt time.Time) (string, error) {
	return r.createToken(ctx, tx, verificationToken, userID, now, expiresAt)
}

// ConsumeVerificationToken atomically consumes a verification token and
// returns its user. Unknown, expired and already consumed tokens answer
// ErrInvalidToken.
func (r *Repository) ConsumeVerificationToken(ctx context.Context, db database.DBTX, raw string, now time.Time) (uuid.UUID, error) {
	return r.consumeToken(ctx, db, verificationToken, raw, now)
}

// LatestVerificationTokenAt returns when the user's most recent verification
// token was created, for resend throttling.
func (r *Repository) LatestVerificationTokenAt(ctx context.Context, db database.DBTX, userID uuid.UUID) (time.Time, bool, error) {
	var at time.Time
	err := db.QueryRow(ctx, `SELECT created_at FROM email_verification_tokens WHERE user_id = $1 ORDER BY created_at DESC LIMIT 1`, userID).Scan(&at)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, fmt.Errorf("auth: latest verification token: %w", err)
	}
	return at, true, nil
}

// CreatePasswordResetToken issues a new reset token and retires outstanding
// ones for the user. Like CreateVerificationToken it must run inside the
// caller's transaction.
func (r *Repository) CreatePasswordResetToken(ctx context.Context, tx pgx.Tx, userID uuid.UUID, now, expiresAt time.Time) (string, error) {
	return r.createToken(ctx, tx, resetToken, userID, now, expiresAt)
}

// ConsumePasswordResetToken atomically consumes a reset token and returns
// its user.
func (r *Repository) ConsumePasswordResetToken(ctx context.Context, db database.DBTX, raw string, now time.Time) (uuid.UUID, error) {
	return r.consumeToken(ctx, db, resetToken, raw, now)
}

// lockUserSQL serialises token issuance per user: concurrent transactions
// queue on the user row, so "retire old, insert new" never interleaves and
// at most one unconsumed token exists per user and kind. NO KEY UPDATE does
// not block inserts of rows that merely reference the user.
const lockUserSQL = `SELECT 1 FROM users WHERE id = $1 FOR NO KEY UPDATE`

func (r *Repository) createToken(ctx context.Context, tx pgx.Tx, kind tokenKind, userID uuid.UUID, now, expiresAt time.Time) (string, error) {
	var one int
	if err := tx.QueryRow(ctx, lockUserSQL, userID).Scan(&one); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrUserNotFound
		}
		return "", fmt.Errorf("auth: lock user for %s: %w", kind, err)
	}
	db := tx
	// kind is one of two constants, never client input.
	retire := `UPDATE ` + string(kind) + ` SET consumed_at = $2 WHERE user_id = $1 AND consumed_at IS NULL`
	if _, err := db.Exec(ctx, retire, userID, now); err != nil {
		return "", fmt.Errorf("auth: retire %s: %w", kind, err)
	}
	id, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("auth: generate id: %w", err)
	}
	raw, hash, err := newToken()
	if err != nil {
		return "", err
	}
	insert := `INSERT INTO ` + string(kind) + ` (id, user_id, token_hash, expires_at, created_at) VALUES ($1, $2, $3, $4, $5)`
	if _, err := db.Exec(ctx, insert, id, userID, hash, expiresAt, now); err != nil {
		return "", fmt.Errorf("auth: create %s: %w", kind, err)
	}
	return raw, nil
}

func (r *Repository) consumeToken(ctx context.Context, db database.DBTX, kind tokenKind, raw string, now time.Time) (uuid.UUID, error) {
	consume := `
		UPDATE ` + string(kind) + `
		SET consumed_at = $2
		WHERE token_hash = $1
		  AND consumed_at IS NULL
		  AND expires_at > $2
		RETURNING user_id`
	var userID uuid.UUID
	err := db.QueryRow(ctx, consume, hashToken(raw), now).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, ErrInvalidToken
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("auth: consume %s: %w", kind, err)
	}
	return userID, nil
}
