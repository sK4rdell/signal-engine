package auth

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/sK4rdell/signal-engine/internal/account"
	"github.com/sK4rdell/signal-engine/internal/platform/password"
)

// SignupResult is what a successful signup produces.
type SignupResult struct {
	User         User
	Account      account.Account
	Session      Session
	SessionToken string
}

// Signup registers a user with a personal account, an owner membership, a
// verification token and the job that mails it, all in one transaction, and
// opens a session. Unverified users may log in; products that require
// verification first can check User.EmailVerifiedAt where it matters.
func (s *Service) Signup(ctx context.Context, email, plainPassword string) (SignupResult, error) {
	// Hashing is CPU-bound; keep it outside the transaction.
	hash, err := password.Hash(plainPassword)
	if err != nil {
		return SignupResult{}, fmt.Errorf("auth: hash password: %w", err)
	}

	var result SignupResult
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		user, err := s.repo.CreateUser(ctx, tx, email, hash)
		if err != nil {
			return err
		}
		acc, err := s.accounts.Create(ctx, tx, user.Email)
		if err != nil {
			return err
		}
		if _, err := s.accounts.AddMembership(ctx, tx, acc.ID, user.ID, account.RoleOwner); err != nil {
			return err
		}
		if err := s.issueVerification(ctx, tx, user); err != nil {
			return err
		}
		sess, token, err := s.CreateSession(ctx, tx, user.ID)
		if err != nil {
			return err
		}
		result = SignupResult{User: user, Account: acc, Session: sess, SessionToken: token}
		return nil
	})
	if err != nil {
		return SignupResult{}, err
	}
	return result, nil
}

// issueVerification creates a verification token and enqueues the email
// inside tx, so both exist exactly when the surrounding change commits.
func (s *Service) issueVerification(ctx context.Context, tx pgx.Tx, user User) error {
	now := s.now()
	token, err := s.repo.CreateVerificationToken(ctx, tx, user.ID, now, now.Add(s.cfg.VerificationTokenTTL))
	if err != nil {
		return err
	}
	return s.enqueueTokenEmail(ctx, tx, JobSendVerificationEmail, user, token)
}
