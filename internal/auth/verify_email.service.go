package auth

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/sK4rdell/signal-engine/internal/platform/apperror"
	"github.com/sK4rdell/signal-engine/internal/platform/jobs"
)

// VerifyEmail consumes a verification token and marks the user verified.
// Consuming and marking happen in one transaction so a token can never be
// spent without its effect. The welcome email is enqueued the first time the
// user is verified.
func (s *Service) VerifyEmail(ctx context.Context, token string) error {
	if !validTokenShape(token) {
		return ErrInvalidToken
	}
	return s.inTx(ctx, func(tx pgx.Tx) error {
		userID, err := s.repo.ConsumeVerificationToken(ctx, tx, token, s.now())
		if err != nil {
			return err
		}
		first, err := s.repo.MarkEmailVerified(ctx, tx, userID, s.now())
		if err != nil {
			return err
		}
		if !first {
			return nil
		}
		user, err := s.repo.GetUserByID(ctx, tx, userID)
		if err != nil {
			return err
		}
		_, err = jobs.Enqueue(ctx, tx, JobSendWelcomeEmail, emailJobPayload{UserID: user.ID, Email: user.Email})
		return err
	})
}

// ResendVerification issues a fresh verification token for an unverified
// user, at most once per configured cooldown. The whole check-and-issue
// runs under the user's row lock, so concurrent resends cannot both pass
// the cooldown and each retire the other's token.
func (s *Service) ResendVerification(ctx context.Context, userID uuidType) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		user, err := s.repo.GetUserForUpdate(ctx, tx, userID)
		if err != nil {
			return err
		}
		if user.EmailVerifiedAt != nil {
			return ErrEmailAlreadyVerified
		}
		last, ok, err := s.repo.LatestVerificationTokenAt(ctx, tx, user.ID)
		if err != nil {
			return err
		}
		if ok && s.now().Sub(last) < s.cfg.ResendCooldown {
			return apperror.RateLimited().WithMessage("A verification email was sent recently, try again later")
		}
		return s.issueVerification(ctx, tx, user)
	})
}
