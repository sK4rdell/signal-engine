package auth

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"betemplate/internal/platform/config"
	"betemplate/internal/platform/email"
	"betemplate/internal/platform/jobs"
	"betemplate/internal/platform/secretbox"
)

// uuidType keeps the service signatures readable without importing uuid in
// every file.
type uuidType = uuid.UUID

// Job types owned by this package.
const (
	JobSendVerificationEmail  = "auth.send_verification_email"
	JobSendPasswordResetEmail = "auth.send_password_reset_email"
	JobSendWelcomeEmail       = "auth.send_welcome_email"
)

// emailJobPayload is the payload of every auth email job. The raw security
// token never appears in it: SealedToken is the token encrypted with the
// configured AUTH_ENCRYPTION_KEY, so the jobs table holds no plaintext
// token. The jobs table also clears payloads once a job completes.
type emailJobPayload struct {
	UserID      uuid.UUID `json:"user_id"`
	Email       string    `json:"email"`
	SealedToken string    `json:"sealed_token,omitempty"`
}

// RegisterJobs binds the auth email jobs to a worker. box must be built
// from the same key the service seals tokens with.
func RegisterJobs(w *jobs.Worker, sender email.Sender, cfg config.AuthConfig, box *secretbox.Box) {
	verifyLink := func(token string) string { return verificationLink(cfg.AppBaseURL, token) }
	resetLinkFn := func(token string) string { return resetLink(cfg.AppBaseURL, token) }
	baseLink := func(string) string { return cfg.AppBaseURL }

	w.Register(JobSendVerificationEmail, sendEmailJob(sender, box, email.TemplateVerifyEmail, cfg.VerificationTokenTTL.String(), true, verifyLink))
	w.Register(JobSendPasswordResetEmail, sendEmailJob(sender, box, email.TemplateResetPassword, cfg.ResetTokenTTL.String(), true, resetLinkFn))
	w.Register(JobSendWelcomeEmail, sendEmailJob(sender, box, email.TemplateWelcome, "", false, baseLink))
}

type emailTemplateData struct {
	Link      string
	ExpiresIn string
}

// sendEmailJob renders and sends one templated email. With needsToken the
// payload must carry a sealed token, which is opened here, in the worker,
// and only ever exists in memory and in the outbound message.
func sendEmailJob(sender email.Sender, box *secretbox.Box, tmpl email.Template, expiresIn string, needsToken bool, link func(token string) string) jobs.Handler {
	return func(ctx context.Context, job jobs.Job) error {
		var p emailJobPayload
		if err := job.UnmarshalPayload(&p); err != nil {
			return jobs.Permanent(err)
		}
		if p.Email == "" {
			return jobs.Permanent(fmt.Errorf("auth: job %s has no recipient", job.ID))
		}
		var token string
		if needsToken {
			if p.SealedToken == "" {
				return jobs.Permanent(fmt.Errorf("auth: job %s has no sealed token", job.ID))
			}
			raw, err := box.Open(p.SealedToken)
			if err != nil {
				return jobs.Permanent(fmt.Errorf("auth: job %s: %w", job.ID, err))
			}
			token = string(raw)
		}
		msg, err := email.Render(tmpl, []string{p.Email}, emailTemplateData{Link: link(token), ExpiresIn: expiresIn})
		if err != nil {
			return jobs.Permanent(err)
		}
		return sender.Send(ctx, msg)
	}
}

// enqueueTokenEmail seals a raw token and enqueues the job that mails it.
func (s *Service) enqueueTokenEmail(ctx context.Context, tx pgxTx, jobType string, user User, rawToken string) error {
	sealed, err := s.box.Seal([]byte(rawToken))
	if err != nil {
		return err
	}
	_, err = jobs.Enqueue(ctx, tx, jobType, emailJobPayload{UserID: user.ID, Email: user.Email, SealedToken: sealed})
	return err
}

func verificationLink(baseURL, token string) string {
	return fmt.Sprintf("%s/verify-email?token=%s", baseURL, token)
}

func resetLink(baseURL, token string) string {
	return fmt.Sprintf("%s/reset-password?token=%s", baseURL, token)
}
