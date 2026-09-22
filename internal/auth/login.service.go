package auth

import (
	"context"
	"errors"
	"fmt"

	"betemplate/internal/platform/logging"
	"betemplate/internal/platform/password"
)

// LoginResult is what a successful login produces.
type LoginResult struct {
	User         User
	Session      Session
	SessionToken string
}

// Login verifies credentials and opens a session. Unknown emails and wrong
// passwords answer the same ErrInvalidCredentials, and the unknown-email
// path still performs a hash verification so its timing matches.
func (s *Service) Login(ctx context.Context, email, plainPassword string) (LoginResult, error) {
	user, err := s.repo.GetUserByEmail(ctx, s.pool, email)
	if errors.Is(err, ErrUserNotFound) {
		_, _ = password.Verify(password.DummyHash, plainPassword)
		return LoginResult{}, ErrInvalidCredentials
	}
	if err != nil {
		return LoginResult{}, err
	}

	ok, err := password.Verify(user.PasswordHash, plainPassword)
	if err != nil {
		return LoginResult{}, fmt.Errorf("auth: verify password for user %s: %w", user.ID, err)
	}
	if !ok {
		return LoginResult{}, ErrInvalidCredentials
	}

	if password.NeedsRehash(user.PasswordHash) {
		// Best effort: a failed upgrade must not block the login.
		if hash, err := password.Hash(plainPassword); err == nil {
			if err := s.repo.UpdatePasswordHash(ctx, s.pool, user.ID, hash); err != nil {
				logging.FromContext(ctx).Warn("password rehash failed", "user_id", user.ID, "error", err)
			}
		}
	}

	sess, token, err := s.CreateSession(ctx, s.pool, user.ID)
	if err != nil {
		return LoginResult{}, err
	}
	return LoginResult{User: user, Session: sess, SessionToken: token}, nil
}
