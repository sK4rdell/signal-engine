package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"betemplate/internal/platform/password"
)

// ForgotPassword issues a reset token and mails it when the email belongs
// to a user. It reports nothing about whether the user exists.
func (s *Service) ForgotPassword(ctx context.Context, email string) error {
	user, err := s.repo.GetUserByEmail(ctx, s.pool, email)
	if errors.Is(err, ErrUserNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return s.inTx(ctx, func(tx pgx.Tx) error {
		now := s.now()
		token, err := s.repo.CreatePasswordResetToken(ctx, tx, user.ID, now, now.Add(s.cfg.ResetTokenTTL))
		if err != nil {
			return err
		}
		return s.enqueueTokenEmail(ctx, tx, JobSendPasswordResetEmail, user, token)
	})
}

// ResetPassword consumes a reset token, replaces the password and revokes
// every existing session of the user, in one transaction.
func (s *Service) ResetPassword(ctx context.Context, token, plainPassword string) error {
	if !validTokenShape(token) {
		return ErrInvalidToken
	}
	hash, err := password.Hash(plainPassword)
	if err != nil {
		return fmt.Errorf("auth: hash password: %w", err)
	}
	return s.inTx(ctx, func(tx pgx.Tx) error {
		userID, err := s.repo.ConsumePasswordResetToken(ctx, tx, token, s.now())
		if err != nil {
			return err
		}
		if err := s.repo.UpdatePasswordHash(ctx, tx, userID, hash); err != nil {
			return err
		}
		_, err = s.repo.RevokeUserSessions(ctx, tx, userID, s.now())
		return err
	})
}
