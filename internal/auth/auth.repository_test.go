package auth

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"betemplate/internal/platform/database"
	"betemplate/internal/platform/testutil/pgtest"
)

// issue runs a token creation in its own transaction, as the services do.
func issue(t *testing.T, pool *pgxpool.Pool, kind tokenKind, userID uuid.UUID, now, expiresAt time.Time) string {
	t.Helper()
	var raw string
	err := database.InTx(context.Background(), pool, func(tx pgx.Tx) error {
		var err error
		switch kind {
		case verificationToken:
			raw, err = NewRepository().CreateVerificationToken(context.Background(), tx, userID, now, expiresAt)
		default:
			raw, err = NewRepository().CreatePasswordResetToken(context.Background(), tx, userID, now, expiresAt)
		}
		return err
	})
	if err != nil {
		t.Fatalf("issue %s: %v", kind, err)
	}
	return raw
}

func TestRepository_Users(t *testing.T) {
	pool := pgtest.NewPool(t)
	ctx := context.Background()
	repo := NewRepository()

	u, err := repo.CreateUser(ctx, pool, "  Alice@Example.COM ", "hash")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if u.Email != "  Alice@Example.COM " || u.EmailNormalized != "alice@example.com" || u.EmailVerifiedAt != nil {
		t.Errorf("user = %+v", u)
	}

	_, err = repo.CreateUser(ctx, pool, "alice@example.com", "other")
	if !errors.Is(err, ErrEmailAlreadyRegistered) {
		t.Errorf("duplicate normalized email error = %v", err)
	}

	got, err := repo.GetUserByEmail(ctx, pool, "ALICE@example.com")
	if err != nil || got.ID != u.ID {
		t.Errorf("GetUserByEmail = %+v, %v", got, err)
	}
	if _, err := repo.GetUserByEmail(ctx, pool, "nobody@example.com"); !errors.Is(err, ErrUserNotFound) {
		t.Errorf("unknown email error = %v", err)
	}
	if _, err := repo.GetUserByID(ctx, pool, uuid.New()); !errors.Is(err, ErrUserNotFound) {
		t.Errorf("unknown id error = %v", err)
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
	first, err := repo.MarkEmailVerified(ctx, pool, u.ID, now)
	if err != nil || !first {
		t.Fatalf("MarkEmailVerified = %v, %v", first, err)
	}
	again, err := repo.MarkEmailVerified(ctx, pool, u.ID, now)
	if err != nil || again {
		t.Errorf("second MarkEmailVerified = %v, %v", again, err)
	}
	got, _ = repo.GetUserByID(ctx, pool, u.ID)
	if got.EmailVerifiedAt == nil || !got.EmailVerifiedAt.Equal(now) {
		t.Errorf("EmailVerifiedAt = %v", got.EmailVerifiedAt)
	}

	if err := repo.UpdatePasswordHash(ctx, pool, u.ID, "new"); err != nil {
		t.Fatal(err)
	}
	got, _ = repo.GetUserByID(ctx, pool, u.ID)
	if got.PasswordHash != "new" {
		t.Errorf("PasswordHash = %q", got.PasswordHash)
	}
	if err := repo.UpdatePasswordHash(ctx, pool, uuid.New(), "x"); !errors.Is(err, ErrUserNotFound) {
		t.Errorf("update unknown user error = %v", err)
	}

	// A rehash only applies while the verified hash is still current: a
	// concurrent reset (simulated by the UpdatePasswordHash above, which
	// moved "hash" to "new") must not be overwritten.
	ok, err := repo.RehashPassword(ctx, pool, u.ID, "hash", "upgraded-from-stale")
	if err != nil || ok {
		t.Errorf("stale rehash = %v, %v, want false", ok, err)
	}
	got, _ = repo.GetUserByID(ctx, pool, u.ID)
	if got.PasswordHash != "new" {
		t.Errorf("stale rehash overwrote the password: %q", got.PasswordHash)
	}
	ok, err = repo.RehashPassword(ctx, pool, u.ID, "new", "upgraded")
	if err != nil || !ok {
		t.Errorf("current rehash = %v, %v, want true", ok, err)
	}
	got, _ = repo.GetUserByID(ctx, pool, u.ID)
	if got.PasswordHash != "upgraded" {
		t.Errorf("PasswordHash = %q", got.PasswordHash)
	}
}

func TestRepository_Sessions(t *testing.T) {
	pool := pgtest.NewPool(t)
	ctx := context.Background()
	repo := NewRepository()
	now := time.Now().UTC()

	u, _ := repo.CreateUser(ctx, pool, "alice@example.com", "hash")

	s, raw, err := repo.CreateSession(ctx, pool, u.ID, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if len(raw) < 40 {
		t.Errorf("raw token too short: %q", raw)
	}

	// The raw token is never persisted; only its hash is.
	var stored []byte
	if err := pool.QueryRow(ctx, "SELECT token_hash FROM sessions WHERE id = $1", s.ID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if string(stored) == raw || string(stored) != string(hashToken(raw)) {
		t.Error("stored token hash mismatch or raw token stored")
	}

	got, err := repo.GetSessionByToken(ctx, pool, raw, now)
	if err != nil || got.ID != s.ID || got.UserID != u.ID {
		t.Fatalf("GetSessionByToken = %+v, %v", got, err)
	}
	if _, err := repo.GetSessionByToken(ctx, pool, "bogus", now); !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("unknown token error = %v", err)
	}
	if _, err := repo.GetSessionByToken(ctx, pool, raw, now.Add(2*time.Hour)); !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("expired session error = %v", err)
	}

	// last_seen_at only advances when older than minAge.
	if err := repo.TouchSession(ctx, pool, s.ID, now.Add(time.Minute), 5*time.Minute); err != nil {
		t.Fatal(err)
	}
	got, _ = repo.GetSessionByToken(ctx, pool, raw, now)
	if !got.LastSeenAt.Equal(s.LastSeenAt) {
		t.Error("TouchSession should not write within minAge")
	}
	later := now.Add(10 * time.Minute)
	if err := repo.TouchSession(ctx, pool, s.ID, later, 5*time.Minute); err != nil {
		t.Fatal(err)
	}
	got, _ = repo.GetSessionByToken(ctx, pool, raw, now)
	if !got.LastSeenAt.Equal(later.Truncate(time.Microsecond)) {
		t.Errorf("LastSeenAt = %v, want %v", got.LastSeenAt, later)
	}

	_, raw2, _ := repo.CreateSession(ctx, pool, u.ID, now.Add(time.Hour))
	if err := repo.RevokeSession(ctx, pool, s.ID, now); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetSessionByToken(ctx, pool, raw, now); !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("revoked session error = %v", err)
	}
	if _, err := repo.GetSessionByToken(ctx, pool, raw2, now); err != nil {
		t.Errorf("other session must survive single revoke: %v", err)
	}

	_, raw3, _ := repo.CreateSession(ctx, pool, u.ID, now.Add(time.Hour))
	n, err := repo.RevokeUserSessions(ctx, pool, u.ID, now)
	if err != nil || n != 2 {
		t.Fatalf("RevokeUserSessions = %d, %v", n, err)
	}
	for _, r := range []string{raw2, raw3} {
		if _, err := repo.GetSessionByToken(ctx, pool, r, now); !errors.Is(err, ErrSessionNotFound) {
			t.Errorf("session survived revoke-all: %v", err)
		}
	}
	count, _ := repo.CountLiveSessions(ctx, pool, u.ID, now)
	if count != 0 {
		t.Errorf("live sessions = %d", count)
	}
}

func TestRepository_SecurityTokens(t *testing.T) {
	pool := pgtest.NewPool(t)
	ctx := context.Background()
	repo := NewRepository()
	now := time.Now().UTC()

	u, _ := repo.CreateUser(ctx, pool, "alice@example.com", "hash")

	raw := issue(t, pool, verificationToken, u.ID, now, now.Add(time.Hour))

	var stored []byte
	if err := pool.QueryRow(ctx, "SELECT token_hash FROM email_verification_tokens WHERE user_id = $1", u.ID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if string(stored) == raw {
		t.Error("plaintext token stored")
	}

	at, ok, err := repo.LatestVerificationTokenAt(ctx, pool, u.ID)
	if err != nil || !ok || !at.Equal(now.Truncate(time.Microsecond)) {
		t.Errorf("LatestVerificationTokenAt = %v, %v, %v", at, ok, err)
	}

	// A verification token cannot be used as a reset token.
	if _, err := repo.ConsumePasswordResetToken(ctx, pool, raw, now); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("cross-kind consume error = %v", err)
	}

	userID, err := repo.ConsumeVerificationToken(ctx, pool, raw, now)
	if err != nil || userID != u.ID {
		t.Fatalf("ConsumeVerificationToken = %v, %v", userID, err)
	}
	if _, err := repo.ConsumeVerificationToken(ctx, pool, raw, now); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("reused token error = %v", err)
	}

	expired := issue(t, pool, verificationToken, u.ID, now, now.Add(time.Minute))
	if _, err := repo.ConsumeVerificationToken(ctx, pool, expired, now.Add(2*time.Minute)); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("expired token error = %v", err)
	}

	// Issuing a new token retires the outstanding one.
	first := issue(t, pool, resetToken, u.ID, now, now.Add(time.Hour))
	second := issue(t, pool, resetToken, u.ID, now, now.Add(time.Hour))
	if _, err := repo.ConsumePasswordResetToken(ctx, pool, first, now); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("retired token error = %v", err)
	}
	if id, err := repo.ConsumePasswordResetToken(ctx, pool, second, now); err != nil || id != u.ID {
		t.Errorf("latest token consume = %v, %v", id, err)
	}
	if _, err := repo.ConsumePasswordResetToken(ctx, pool, "not-a-token", now); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("garbage token error = %v", err)
	}
}

func TestRepository_CreateTokenRequiresExistingUser(t *testing.T) {
	pool := pgtest.NewPool(t)
	err := database.InTx(context.Background(), pool, func(tx pgx.Tx) error {
		_, err := NewRepository().CreateVerificationToken(context.Background(), tx, uuid.New(), time.Now(), time.Now().Add(time.Hour))
		return err
	})
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("error = %v, want ErrUserNotFound", err)
	}
}

// TestRepository_ConcurrentTokenIssuanceLeavesOneLiveToken issues tokens
// for the same user from many concurrent transactions and checks the
// invariant the code promises: at most one unconsumed token per user and
// kind, and only the most recently issued token is usable.
func TestRepository_ConcurrentTokenIssuanceLeavesOneLiveToken(t *testing.T) {
	for _, kind := range []tokenKind{verificationToken, resetToken} {
		t.Run(string(kind), func(t *testing.T) {
			pool := pgtest.NewPool(t)
			ctx := context.Background()
			repo := NewRepository()
			now := time.Now().UTC()
			u, _ := repo.CreateUser(ctx, pool, "race@example.com", "hash")

			const n = 16
			var wg sync.WaitGroup
			var mu sync.Mutex
			var raws []string
			var errs []error
			start := make(chan struct{})
			for i := 0; i < n; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					<-start
					var raw string
					err := database.InTx(ctx, pool, func(tx pgx.Tx) error {
						var err error
						if kind == verificationToken {
							raw, err = repo.CreateVerificationToken(ctx, tx, u.ID, now, now.Add(time.Hour))
						} else {
							raw, err = repo.CreatePasswordResetToken(ctx, tx, u.ID, now, now.Add(time.Hour))
						}
						return err
					})
					mu.Lock()
					raws = append(raws, raw)
					errs = append(errs, err)
					mu.Unlock()
				}()
			}
			close(start)
			wg.Wait()
			for _, err := range errs {
				if err != nil {
					t.Fatalf("issuance failed: %v", err)
				}
			}

			var live int
			if err := pool.QueryRow(ctx, `SELECT count(*) FROM `+string(kind)+` WHERE user_id = $1 AND consumed_at IS NULL`, u.ID).Scan(&live); err != nil {
				t.Fatal(err)
			}
			if live != 1 {
				t.Fatalf("live tokens = %d, want exactly 1", live)
			}

			usable := 0
			for _, raw := range raws {
				var err error
				if kind == verificationToken {
					_, err = repo.ConsumeVerificationToken(ctx, pool, raw, now)
				} else {
					_, err = repo.ConsumePasswordResetToken(ctx, pool, raw, now)
				}
				if err == nil {
					usable++
				} else if !errors.Is(err, ErrInvalidToken) {
					t.Fatalf("unexpected consume error: %v", err)
				}
			}
			if usable != 1 {
				t.Fatalf("usable tokens = %d, want exactly 1", usable)
			}
		})
	}
}

func TestNewToken_IsUniqueAndHashed(t *testing.T) {
	a, ha, err := newToken()
	if err != nil {
		t.Fatal(err)
	}
	b, hb, _ := newToken()
	if a == b {
		t.Fatal("tokens must be unique")
	}
	if string(ha) == string(hb) || string(hashToken(a)) != string(ha) {
		t.Error("hash mismatch")
	}
	if NormalizeEmail("  A@B.C ") != "a@b.c" {
		t.Error("NormalizeEmail")
	}
}
