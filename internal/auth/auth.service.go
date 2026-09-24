package auth

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sK4rdell/signal-engine/internal/account"
	"github.com/sK4rdell/signal-engine/internal/platform/config"
	"github.com/sK4rdell/signal-engine/internal/platform/database"
	"github.com/sK4rdell/signal-engine/internal/platform/secretbox"
)

// pgxTx keeps signatures short.
type pgxTx = pgx.Tx

// Service implements the authentication operations. Handlers call it; it
// owns transaction boundaries and calls the repositories.
type Service struct {
	pool     *pgxpool.Pool
	repo     *Repository
	accounts *account.Repository
	cfg      config.AuthConfig
	// box seals raw tokens for the job queue.
	box *secretbox.Box

	// now is overridable by tests.
	now func() time.Time
}

// NewService wires the auth service.
func NewService(pool *pgxpool.Pool, repo *Repository, accounts *account.Repository, cfg config.AuthConfig, box *secretbox.Box) *Service {
	return &Service{
		pool:     pool,
		repo:     repo,
		accounts: accounts,
		cfg:      cfg,
		box:      box,
		now:      func() time.Time { return time.Now().UTC() },
	}
}

// sessionTouchInterval bounds how often last_seen_at is written.
const sessionTouchInterval = 5 * time.Minute

// CreateSession opens a new session for a user and returns it with the raw
// token. It is used by login, signup and the test harness.
func (s *Service) CreateSession(ctx context.Context, db database.DBTX, userID uuid.UUID) (Session, string, error) {
	return s.repo.CreateSession(ctx, db, userID, s.now().Add(s.cfg.SessionTTL))
}

// ResolveSession looks a raw cookie token up and returns the live session.
// Malformed tokens are rejected before touching the database.
func (s *Service) ResolveSession(ctx context.Context, raw string) (Session, error) {
	if !validTokenShape(raw) {
		return Session{}, ErrSessionNotFound
	}
	sess, err := s.repo.GetSessionByToken(ctx, s.pool, raw, s.now())
	if err != nil {
		return Session{}, err
	}
	if err := s.repo.TouchSession(ctx, s.pool, sess.ID, s.now(), sessionTouchInterval); err != nil {
		return Session{}, err
	}
	return sess, nil
}

// GetUser returns a user by ID.
func (s *Service) GetUser(ctx context.Context, id uuid.UUID) (User, error) {
	return s.repo.GetUserByID(ctx, s.pool, id)
}

// Logout revokes one session.
func (s *Service) Logout(ctx context.Context, sessionID uuid.UUID) error {
	return s.repo.RevokeSession(ctx, s.pool, sessionID, s.now())
}

// LogoutAll revokes every session of a user.
func (s *Service) LogoutAll(ctx context.Context, userID uuid.UUID) error {
	_, err := s.repo.RevokeUserSessions(ctx, s.pool, userID, s.now())
	return err
}

// inTx runs fn in a transaction on the service pool.
func (s *Service) inTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	return database.InTx(ctx, s.pool, fn)
}

// validTokenShape reports whether raw looks like a token this package
// issued: base64url of 32 bytes, 43 characters, no padding.
func validTokenShape(raw string) bool {
	if len(raw) != 43 {
		return false
	}
	for _, c := range raw {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '-', c == '_':
		default:
			return false
		}
	}
	return true
}

// SessionCookie builds the cookie carrying a raw session token.
func (s *Service) SessionCookie(token string, expiresAt time.Time) *http.Cookie {
	return &http.Cookie{
		Name:     s.cfg.SessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		MaxAge:   int(time.Until(expiresAt).Seconds()),
		HttpOnly: true,
		Secure:   s.cfg.SessionCookieSecure,
		SameSite: http.SameSiteLaxMode,
	}
}

// ClearSessionCookie builds the cookie that removes the session cookie.
func (s *Service) ClearSessionCookie() *http.Cookie {
	return &http.Cookie{
		Name:     s.cfg.SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.cfg.SessionCookieSecure,
		SameSite: http.SameSiteLaxMode,
	}
}

// CookieName returns the configured session cookie name.
func (s *Service) CookieName() string { return s.cfg.SessionCookieName }
