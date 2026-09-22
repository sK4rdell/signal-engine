package auth_test

import (
	"context"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"betemplate/internal/auth"
	"betemplate/internal/platform/api"
	"betemplate/internal/platform/apperror"
	"betemplate/internal/platform/config"
	"betemplate/internal/platform/testutil"
)

const goodPassword = "correct horse battery staple"

var tokenPattern = regexp.MustCompile(`token=([A-Za-z0-9_-]+)`)

// tokenFromEmail extracts the token from the most recent recorded email.
func tokenFromEmail(t *testing.T, app *testutil.App) string {
	t.Helper()
	msgs := app.Email.Messages()
	if len(msgs) == 0 {
		t.Fatal("no email recorded")
	}
	m := tokenPattern.FindStringSubmatch(msgs[len(msgs)-1].Text)
	if m == nil {
		t.Fatalf("no token in email: %s", msgs[len(msgs)-1].Text)
	}
	return m[1]
}

func signup(t *testing.T, app *testutil.App, email string) (*testutil.Client, auth.SessionResponse) {
	t.Helper()
	client := app.Client()
	res := client.PostJSON("/v1/auth/signup", map[string]string{"email": email, "password": goodPassword})
	res.AssertStatus(t, http.StatusCreated)
	var body auth.SessionResponse
	res.DecodeJSON(t, &body)
	return client, body
}

// --- signup ----------------------------------------------------------------

func TestSignup_CreatesUserAccountTokenAndJobAtomically(t *testing.T) {
	app := testutil.NewApp(t)
	ctx := context.Background()

	client := app.Client()
	res := client.PostJSON("/v1/auth/signup", map[string]string{"email": "New.User@Example.com", "password": goodPassword})
	res.AssertStatus(t, http.StatusCreated)

	var body auth.SessionResponse
	res.DecodeJSON(t, &body)
	if body.User.ID == uuid.Nil || body.User.Email != "New.User@Example.com" || body.User.EmailVerified {
		t.Errorf("user = %+v", body.User)
	}
	if strings.Contains(string(res.Body), "password") {
		t.Error("response must not contain password material")
	}

	cookie := res.Cookie(app.Config.Auth.SessionCookieName)
	if cookie == nil || cookie.Value == "" || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode || cookie.Path != "/" {
		t.Fatalf("session cookie = %+v", cookie)
	}

	// Persisted state.
	var normalized, hash string
	if err := app.Pool.QueryRow(ctx, "SELECT email_normalized, password_hash FROM users WHERE id = $1", body.User.ID).Scan(&normalized, &hash); err != nil {
		t.Fatal(err)
	}
	if normalized != "new.user@example.com" {
		t.Errorf("email_normalized = %q", normalized)
	}
	if !strings.HasPrefix(hash, "$argon2id$") || strings.Contains(hash, goodPassword) {
		t.Errorf("password_hash = %q", hash)
	}

	var memberships int
	if err := app.Pool.QueryRow(ctx, "SELECT count(*) FROM account_memberships WHERE user_id = $1 AND role = 'owner'", body.User.ID).Scan(&memberships); err != nil {
		t.Fatal(err)
	}
	if memberships != 1 {
		t.Errorf("owner memberships = %d", memberships)
	}

	var tokens int
	if err := app.Pool.QueryRow(ctx, "SELECT count(*) FROM email_verification_tokens WHERE user_id = $1 AND consumed_at IS NULL", body.User.ID).Scan(&tokens); err != nil {
		t.Fatal(err)
	}
	if tokens != 1 {
		t.Errorf("verification tokens = %d", tokens)
	}

	jobs := app.Jobs(auth.JobSendVerificationEmail)
	if len(jobs) != 1 {
		t.Fatalf("verification jobs = %d", len(jobs))
	}

	// The session works immediately.
	client.Get("/v1/auth/me").AssertStatus(t, http.StatusOK)

	// Nothing sensitive was logged.
	logs := app.Logs()
	if strings.Contains(logs, goodPassword) || strings.Contains(logs, cookie.Value) {
		t.Error("password or session token found in logs")
	}
}

func TestSignup_RejectsInvalidInput(t *testing.T) {
	app := testutil.NewApp(t)
	tests := []struct {
		name  string
		body  string
		code  string
		field string
	}{
		{"invalid email", `{"email":"nope","password":"` + goodPassword + `"}`, apperror.CodeValidationFailed, "email"},
		{"short password", `{"email":"a@example.com","password":"short"}`, apperror.CodeValidationFailed, "password"},
		{"missing password", `{"email":"a@example.com"}`, apperror.CodeValidationFailed, "password"},
		{"unknown field", `{"emial":"a@example.com","password":"` + goodPassword + `"}`, apperror.CodeInvalidRequest, "emial"},
		{"malformed json", `{"email":`, apperror.CodeInvalidRequest, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res := app.Client().PostJSON("/v1/auth/signup", tc.body)
			status := http.StatusUnprocessableEntity
			if tc.code == apperror.CodeInvalidRequest {
				status = http.StatusBadRequest
			}
			e := res.AssertError(t, status, tc.code)
			if tc.field != "" && e.Fields[tc.field] == "" {
				t.Errorf("fields = %v, want %s", e.Fields, tc.field)
			}
		})
	}
	var users int
	_ = app.Pool.QueryRow(context.Background(), "SELECT count(*) FROM users").Scan(&users)
	if users != 0 {
		t.Errorf("users created by invalid signups: %d", users)
	}
}

func TestSignup_DuplicateEmailConflicts(t *testing.T) {
	app := testutil.NewApp(t)
	signup(t, app, "dup@example.com")

	res := app.Client().PostJSON("/v1/auth/signup", map[string]string{"email": "DUP@example.com", "password": goodPassword})
	res.AssertError(t, http.StatusConflict, auth.CodeEmailAlreadyRegistered)

	var users, accounts, jobs int
	_ = app.Pool.QueryRow(context.Background(), "SELECT count(*) FROM users").Scan(&users)
	_ = app.Pool.QueryRow(context.Background(), "SELECT count(*) FROM accounts").Scan(&accounts)
	_ = app.Pool.QueryRow(context.Background(), "SELECT count(*) FROM jobs").Scan(&jobs)
	if users != 1 || accounts != 1 || jobs != 1 {
		t.Errorf("users=%d accounts=%d jobs=%d, want 1 each (failed signup must roll back)", users, accounts, jobs)
	}
}

// --- email verification ----------------------------------------------------

func TestVerifyEmail_Flow(t *testing.T) {
	app := testutil.NewApp(t)
	client, body := signup(t, app, "verify@example.com")

	if n := app.RunJobs(); n != 1 {
		t.Fatalf("jobs run = %d", n)
	}
	msgs := app.Email.Messages()
	if len(msgs) != 1 || msgs[0].To[0] != "verify@example.com" || msgs[0].Subject != "Verify your email address" ||
		!strings.Contains(msgs[0].Text, app.Config.Auth.AppBaseURL+"/verify-email?token=") {
		t.Fatalf("emails = %+v", msgs)
	}
	token := tokenFromEmail(t, app)

	// The plaintext token is neither stored nor logged.
	var hashes [][]byte
	rows, _ := app.Pool.Query(context.Background(), "SELECT token_hash FROM email_verification_tokens")
	for rows.Next() {
		var h []byte
		_ = rows.Scan(&h)
		hashes = append(hashes, h)
	}
	rows.Close()
	for _, h := range hashes {
		if string(h) == token {
			t.Error("plaintext token stored")
		}
	}
	if strings.Contains(app.Logs(), token) {
		t.Error("token found in logs")
	}
	// The job payload is cleared after delivery.
	if job := app.Jobs(auth.JobSendVerificationEmail)[0]; job.Payload != nil {
		t.Error("completed job still carries its payload")
	}
	assertNoPlaintextTokenInPayloads(t, app, token)

	// Verification works without a session (link opened anywhere).
	app.Client().PostJSON("/v1/auth/verify-email", map[string]string{"token": token}).AssertStatus(t, http.StatusNoContent)

	var me auth.UserResponse
	client.Get("/v1/auth/me").AssertStatus(t, http.StatusOK).DecodeJSON(t, &me)
	if !me.EmailVerified || me.ID != body.User.ID {
		t.Errorf("me = %+v", me)
	}

	// Reuse fails.
	app.Client().PostJSON("/v1/auth/verify-email", map[string]string{"token": token}).AssertError(t, http.StatusUnprocessableEntity, auth.CodeInvalidToken)

	// A welcome email was queued exactly once.
	if len(app.Jobs(auth.JobSendWelcomeEmail)) != 1 {
		t.Error("welcome job missing")
	}
	app.RunJobs()
	if msgs := app.Email.Messages(); len(msgs) != 2 || msgs[1].Subject != "Welcome!" {
		t.Errorf("emails = %+v", msgs)
	}
}

func TestVerifyEmail_RejectsInvalidAndExpiredTokens(t *testing.T) {
	app := testutil.NewApp(t)
	signup(t, app, "expired@example.com")
	app.RunJobs()
	token := tokenFromEmail(t, app)

	for _, bad := range []string{"garbage", strings.Repeat("a", 43), token + "x"} {
		app.Client().PostJSON("/v1/auth/verify-email", map[string]string{"token": bad}).AssertError(t, http.StatusUnprocessableEntity, auth.CodeInvalidToken)
	}
	app.Client().PostJSON("/v1/auth/verify-email", map[string]string{"token": ""}).AssertError(t, http.StatusUnprocessableEntity, apperror.CodeValidationFailed)

	if _, err := app.Pool.Exec(context.Background(), "UPDATE email_verification_tokens SET expires_at = now() - interval '1 minute'"); err != nil {
		t.Fatal(err)
	}
	app.Client().PostJSON("/v1/auth/verify-email", map[string]string{"token": token}).AssertError(t, http.StatusUnprocessableEntity, auth.CodeInvalidToken)

	var verified *time.Time
	_ = app.Pool.QueryRow(context.Background(), "SELECT email_verified_at FROM users").Scan(&verified)
	if verified != nil {
		t.Error("user must not be verified")
	}
}

func TestResendVerification(t *testing.T) {
	app := testutil.NewApp(t, testutil.WithConfig(func(cfg *config.Config) {
		cfg.Auth.ResendCooldown = time.Hour
	}))
	client, _ := signup(t, app, "resend@example.com")

	// Within the cooldown of the signup token.
	client.PostJSON("/v1/auth/resend-verification", nil).AssertError(t, http.StatusTooManyRequests, apperror.CodeRateLimited)

	if _, err := app.Pool.Exec(context.Background(), "UPDATE email_verification_tokens SET created_at = now() - interval '2 hours'"); err != nil {
		t.Fatal(err)
	}
	client.PostJSON("/v1/auth/resend-verification", nil).AssertStatus(t, http.StatusNoContent)
	if len(app.Jobs(auth.JobSendVerificationEmail)) != 2 {
		t.Error("resend should enqueue a second email")
	}

	// The old token was retired by the resend; the new one works.
	app.RunJobs()
	msgs := app.Email.Messages()
	first := tokenPattern.FindStringSubmatch(msgs[0].Text)[1]
	second := tokenPattern.FindStringSubmatch(msgs[1].Text)[1]
	app.Client().PostJSON("/v1/auth/verify-email", map[string]string{"token": first}).AssertError(t, http.StatusUnprocessableEntity, auth.CodeInvalidToken)
	app.Client().PostJSON("/v1/auth/verify-email", map[string]string{"token": second}).AssertStatus(t, http.StatusNoContent)

	client.PostJSON("/v1/auth/resend-verification", nil).AssertError(t, http.StatusConflict, auth.CodeEmailAlreadyVerified)

	app.Client().PostJSON("/v1/auth/resend-verification", nil).AssertError(t, http.StatusUnauthorized, apperror.CodeUnauthorized)
}

// TestResendVerification_ConcurrentRequestsIssueOneToken fires many resend
// requests at once after the cooldown has elapsed: exactly one may issue a
// token and email, the rest are rate limited.
func TestResendVerification_ConcurrentRequestsIssueOneToken(t *testing.T) {
	app := testutil.NewApp(t, testutil.WithConfig(func(cfg *config.Config) {
		cfg.Auth.ResendCooldown = time.Hour
	}))
	_, body := signup(t, app, "race@example.com")
	if _, err := app.Pool.Exec(context.Background(), "UPDATE email_verification_tokens SET created_at = now() - interval '2 hours'"); err != nil {
		t.Fatal(err)
	}
	user := testutil.User{ID: body.User.ID, Email: body.User.Email}

	const n = 8
	var wg sync.WaitGroup
	var mu sync.Mutex
	statuses := map[int]int{}
	clients := make([]*testutil.Client, n)
	for i := range clients {
		clients[i] = app.AuthenticatedClient(user)
	}
	start := make(chan struct{})
	for _, client := range clients {
		wg.Add(1)
		go func(c *testutil.Client) {
			defer wg.Done()
			<-start
			res := c.PostJSON("/v1/auth/resend-verification", nil)
			mu.Lock()
			statuses[res.StatusCode]++
			mu.Unlock()
		}(client)
	}
	close(start)
	wg.Wait()

	if statuses[http.StatusNoContent] != 1 || statuses[http.StatusTooManyRequests] != n-1 {
		t.Fatalf("statuses = %v, want one 204 and %d 429", statuses, n-1)
	}
	if jobs := app.Jobs(auth.JobSendVerificationEmail); len(jobs) != 2 {
		t.Errorf("verification jobs = %d, want 2 (signup + one resend)", len(jobs))
	}
	var live int
	_ = app.Pool.QueryRow(context.Background(), "SELECT count(*) FROM email_verification_tokens WHERE consumed_at IS NULL").Scan(&live)
	if live != 1 {
		t.Errorf("live tokens = %d, want 1", live)
	}
}

// --- login -----------------------------------------------------------------

func TestLogin(t *testing.T) {
	app := testutil.NewApp(t)
	user := app.CreateUser(testutil.WithEmail("login@example.com"))

	client := app.Client()
	res := client.PostJSON("/v1/auth/login", map[string]string{"email": "LOGIN@example.com", "password": user.Password})
	res.AssertStatus(t, http.StatusOK)
	var body auth.SessionResponse
	res.DecodeJSON(t, &body)
	if body.User.ID != user.ID || !body.User.EmailVerified {
		t.Errorf("user = %+v", body.User)
	}
	cookie := res.Cookie(app.Config.Auth.SessionCookieName)
	if cookie == nil || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode || cookie.Path != "/" || cookie.Secure {
		t.Fatalf("cookie = %+v", cookie)
	}
	if cookie.Expires.Before(time.Now().Add(app.Config.Auth.SessionTTL - time.Minute)) {
		t.Errorf("cookie expiry %v too early", cookie.Expires)
	}

	// Only a hash of the token is stored.
	var stored int
	if err := app.Pool.QueryRow(context.Background(), "SELECT count(*) FROM sessions WHERE token_hash = $1", []byte(cookie.Value)).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored != 0 {
		t.Error("raw session token stored in database")
	}
	var sessions int
	_ = app.Pool.QueryRow(context.Background(), "SELECT count(*) FROM sessions WHERE user_id = $1", user.ID).Scan(&sessions)
	if sessions != 1 {
		t.Errorf("sessions = %d", sessions)
	}

	client.Get("/v1/auth/me").AssertStatus(t, http.StatusOK)
	if strings.Contains(app.Logs(), cookie.Value) || strings.Contains(app.Logs(), user.Password) {
		t.Error("session token or password logged")
	}
}

func TestLogin_InvalidCredentialsAreIndistinguishable(t *testing.T) {
	app := testutil.NewApp(t)
	user := app.CreateUser()

	wrong := app.Client().PostJSON("/v1/auth/login", map[string]string{"email": user.Email, "password": "not the password"})
	unknown := app.Client().PostJSON("/v1/auth/login", map[string]string{"email": "nobody@example.com", "password": "not the password"})
	wrong.AssertError(t, http.StatusUnauthorized, auth.CodeInvalidCredentials)
	unknown.AssertError(t, http.StatusUnauthorized, auth.CodeInvalidCredentials)
	if string(wrong.Body) != string(unknown.Body) {
		t.Errorf("bodies differ:\n%s\n%s", wrong.Body, unknown.Body)
	}
	if wrong.Cookie(app.Config.Auth.SessionCookieName) != nil {
		t.Error("no cookie on failed login")
	}
	var sessions int
	_ = app.Pool.QueryRow(context.Background(), "SELECT count(*) FROM sessions").Scan(&sessions)
	if sessions != 0 {
		t.Errorf("sessions after failed logins = %d", sessions)
	}

	app.Client().PostJSON("/v1/auth/login", map[string]string{"email": "nope", "password": "x"}).AssertError(t, http.StatusUnprocessableEntity, apperror.CodeValidationFailed)
}

func TestLogin_RateLimited(t *testing.T) {
	app := testutil.NewApp(t, testutil.WithConfig(func(cfg *config.Config) {
		cfg.Auth.RateLimit = config.RateLimitConfig{Requests: 3, Window: time.Minute}
	}))
	body := map[string]string{"email": "nobody@example.com", "password": "not the password"}

	client := app.Client()
	for i := 0; i < 3; i++ {
		client.PostJSON("/v1/auth/login", body).AssertStatus(t, http.StatusUnauthorized)
	}
	res := client.PostJSON("/v1/auth/login", body)
	res.AssertError(t, http.StatusTooManyRequests, apperror.CodeRateLimited)
	if res.Header.Get("Retry-After") == "" {
		t.Error("Retry-After missing")
	}

	// Another client IP is not affected, and other endpoints have their own budget.
	other := app.Client()
	other.RemoteAddr = "203.0.113.99:1"
	other.PostJSON("/v1/auth/login", body).AssertStatus(t, http.StatusUnauthorized)
	client.PostJSON("/v1/auth/forgot-password", map[string]string{"email": "nobody@example.com"}).AssertStatus(t, http.StatusNoContent)

	// Signup shares the same configured limit.
	for i := 0; i < 3; i++ {
		client.PostJSON("/v1/auth/signup", map[string]string{"email": "bad", "password": "x"}).AssertStatus(t, http.StatusUnprocessableEntity)
	}
	client.PostJSON("/v1/auth/signup", map[string]string{"email": "bad", "password": "x"}).AssertStatus(t, http.StatusTooManyRequests)
}

// --- sessions --------------------------------------------------------------

func TestSession_Protection(t *testing.T) {
	app := testutil.NewApp(t)
	user := app.CreateUser()
	cookieName := app.Config.Auth.SessionCookieName

	t.Run("missing cookie", func(t *testing.T) {
		app.Client().Get("/v1/auth/me").AssertError(t, http.StatusUnauthorized, apperror.CodeUnauthorized)
	})
	t.Run("malformed cookie", func(t *testing.T) {
		for _, value := range []string{"garbage", strings.Repeat("A", 43), "../../etc", strings.Repeat("A", 5000)} {
			client := app.Client()
			client.SetCookie(&http.Cookie{Name: cookieName, Value: value})
			client.Get("/v1/auth/me").AssertError(t, http.StatusUnauthorized, apperror.CodeUnauthorized)
		}
	})
	t.Run("valid session", func(t *testing.T) {
		app.AuthenticatedClient(user).Get("/v1/auth/me").AssertStatus(t, http.StatusOK)
	})
	t.Run("expired session", func(t *testing.T) {
		client := app.AuthenticatedClient(user)
		if _, err := app.Pool.Exec(context.Background(), "UPDATE sessions SET expires_at = now() - interval '1 second' WHERE id = $1", client.SessionID); err != nil {
			t.Fatal(err)
		}
		client.Get("/v1/auth/me").AssertError(t, http.StatusUnauthorized, apperror.CodeUnauthorized)
	})
	t.Run("revoked session", func(t *testing.T) {
		client := app.AuthenticatedClient(user)
		if _, err := app.Pool.Exec(context.Background(), "UPDATE sessions SET revoked_at = now() WHERE id = $1", client.SessionID); err != nil {
			t.Fatal(err)
		}
		client.Get("/v1/auth/me").AssertError(t, http.StatusUnauthorized, apperror.CodeUnauthorized)
	})
	t.Run("untrusted origin on unsafe request", func(t *testing.T) {
		client := app.AuthenticatedClient(user)
		client.Origin = "http://evil.test"
		client.PostJSON("/v1/auth/logout", nil).AssertError(t, http.StatusForbidden, api.CodeOriginNotAllowed)
		client.Get("/v1/auth/me").AssertStatus(t, http.StatusOK)
	})
}

func TestLogout_RevokesOnlyCurrentSession(t *testing.T) {
	app := testutil.NewApp(t)
	user := app.CreateUser()
	first := app.AuthenticatedClient(user)
	second := app.AuthenticatedClient(user)

	res := first.PostJSON("/v1/auth/logout", nil)
	res.AssertStatus(t, http.StatusNoContent)
	cookie := res.Cookie(app.Config.Auth.SessionCookieName)
	if cookie == nil || cookie.MaxAge != -1 || cookie.Value != "" {
		t.Errorf("logout must clear the cookie, got %+v", cookie)
	}
	if _, ok := first.Cookie(app.Config.Auth.SessionCookieName); ok {
		t.Error("client still holds the cookie")
	}
	first.PostJSON("/v1/auth/logout", nil).AssertError(t, http.StatusUnauthorized, apperror.CodeUnauthorized)
	second.Get("/v1/auth/me").AssertStatus(t, http.StatusOK)
}

func TestLogoutAll_RevokesEverySession(t *testing.T) {
	app := testutil.NewApp(t)
	user := app.CreateUser()
	clients := []*testutil.Client{app.AuthenticatedClient(user), app.AuthenticatedClient(user), app.AuthenticatedClient(user)}
	otherUser := app.AuthenticatedClient(app.CreateUser())

	clients[0].PostJSON("/v1/auth/logout-all", nil).AssertStatus(t, http.StatusNoContent)
	for i, c := range clients {
		if c.Get("/v1/auth/me").StatusCode != http.StatusUnauthorized {
			t.Errorf("session %d survived logout-all", i)
		}
	}
	otherUser.Get("/v1/auth/me").AssertStatus(t, http.StatusOK)
}

// --- password reset --------------------------------------------------------

func TestForgotPassword_DoesNotEnumerateUsers(t *testing.T) {
	app := testutil.NewApp(t)
	user := app.CreateUser(testutil.WithEmail("known@example.com"))

	known := app.Client().PostJSON("/v1/auth/forgot-password", map[string]string{"email": "KNOWN@example.com"})
	unknown := app.Client().PostJSON("/v1/auth/forgot-password", map[string]string{"email": "unknown@example.com"})
	known.AssertStatus(t, http.StatusNoContent)
	unknown.AssertStatus(t, http.StatusNoContent)
	if string(known.Body) != string(unknown.Body) {
		t.Error("responses differ between known and unknown emails")
	}

	jobs := app.Jobs(auth.JobSendPasswordResetEmail)
	if len(jobs) != 1 {
		t.Fatalf("reset jobs = %d, want exactly one (known user only)", len(jobs))
	}
	pendingPayload := string(jobs[0].Payload)
	app.RunJobs()
	msgs := app.Email.Messages()
	if len(msgs) != 1 || msgs[0].To[0] != user.Email || msgs[0].Subject != "Reset your password" || !strings.Contains(msgs[0].Text, "/reset-password?token=") {
		t.Errorf("emails = %+v", msgs)
	}
	token := tokenFromEmail(t, app)
	if strings.Contains(app.Logs(), token) {
		t.Error("reset token logged")
	}
	if strings.Contains(pendingPayload, token) || strings.Contains(pendingPayload, "token=") || strings.Contains(pendingPayload, "/reset-password") {
		t.Errorf("pending job payload exposes the raw token or link: %s", pendingPayload)
	}
	assertNoPlaintextTokenInPayloads(t, app, token)
}

// assertNoPlaintextTokenInPayloads reads the jobs table directly and checks
// that no payload, pending or historical, contains the raw token or a link.
func assertNoPlaintextTokenInPayloads(t *testing.T, app *testutil.App, token string) {
	t.Helper()
	rows, err := app.Pool.Query(context.Background(), "SELECT coalesce(payload::text, '') FROM jobs")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var payload string
		if err := rows.Scan(&payload); err != nil {
			t.Fatal(err)
		}
		for _, forbidden := range []string{token, "token=", "/verify-email", "/reset-password", "\"link\""} {
			if strings.Contains(payload, forbidden) {
				t.Errorf("job payload contains %q: %s", forbidden, payload)
			}
		}
	}
}

// TestSignup_PendingJobPayloadHoldsNoPlaintextToken inspects the payload
// while the job is still pending, before the worker clears it.
func TestSignup_PendingJobPayloadHoldsNoPlaintextToken(t *testing.T) {
	app := testutil.NewApp(t)
	signup(t, app, "sealed@example.com")

	pending := app.Jobs(auth.JobSendVerificationEmail)
	if len(pending) != 1 || pending[0].Payload == nil {
		t.Fatalf("jobs = %+v", pending)
	}
	payload := string(pending[0].Payload)
	if strings.Contains(payload, "token=") || strings.Contains(payload, "/verify-email") || strings.Contains(payload, "\"link\"") {
		t.Fatalf("pending payload exposes a link: %s", payload)
	}
	if !strings.Contains(payload, "sealed_token") {
		t.Fatalf("pending payload lacks the sealed token: %s", payload)
	}

	app.RunJobs()
	token := tokenFromEmail(t, app)
	if strings.Contains(payload, token) {
		t.Fatalf("pending payload contained the raw token: %s", payload)
	}
	// The sealed value is not the token in disguise: the token the email
	// carries verifies, proving the worker opened the sealed value.
	app.Client().PostJSON("/v1/auth/verify-email", map[string]string{"token": token}).AssertStatus(t, http.StatusNoContent)
}

func TestResetPassword_Flow(t *testing.T) {
	app := testutil.NewApp(t)
	user := app.CreateUser()
	oldSession := app.AuthenticatedClient(user)
	otherUserSession := app.AuthenticatedClient(app.CreateUser())

	app.Client().PostJSON("/v1/auth/forgot-password", map[string]string{"email": user.Email}).AssertStatus(t, http.StatusNoContent)
	app.RunJobs()
	token := tokenFromEmail(t, app)
	const newPassword = "a brand new passphrase"

	// Weak new password is rejected before the token is consumed.
	app.Client().PostJSON("/v1/auth/reset-password", map[string]string{"token": token, "password": "short"}).AssertError(t, http.StatusUnprocessableEntity, apperror.CodeValidationFailed)

	res := app.Client().PostJSON("/v1/auth/reset-password", map[string]string{"token": token, "password": newPassword})
	res.AssertStatus(t, http.StatusNoContent)

	// Token is single-use.
	app.Client().PostJSON("/v1/auth/reset-password", map[string]string{"token": token, "password": newPassword}).AssertError(t, http.StatusUnprocessableEntity, auth.CodeInvalidToken)

	// Old password fails, new password works.
	app.Client().PostJSON("/v1/auth/login", map[string]string{"email": user.Email, "password": user.Password}).AssertError(t, http.StatusUnauthorized, auth.CodeInvalidCredentials)
	app.Client().PostJSON("/v1/auth/login", map[string]string{"email": user.Email, "password": newPassword}).AssertStatus(t, http.StatusOK)

	// Existing sessions are invalidated; other users are untouched.
	oldSession.Get("/v1/auth/me").AssertError(t, http.StatusUnauthorized, apperror.CodeUnauthorized)
	otherUserSession.Get("/v1/auth/me").AssertStatus(t, http.StatusOK)
}

func TestResetPassword_RejectsInvalidAndExpiredTokens(t *testing.T) {
	app := testutil.NewApp(t)
	user := app.CreateUser()
	app.Client().PostJSON("/v1/auth/forgot-password", map[string]string{"email": user.Email}).AssertStatus(t, http.StatusNoContent)
	app.RunJobs()
	token := tokenFromEmail(t, app)

	app.Client().PostJSON("/v1/auth/reset-password", map[string]string{"token": "bogus", "password": "a brand new passphrase"}).AssertError(t, http.StatusUnprocessableEntity, auth.CodeInvalidToken)

	if _, err := app.Pool.Exec(context.Background(), "UPDATE password_reset_tokens SET expires_at = now() - interval '1 minute'"); err != nil {
		t.Fatal(err)
	}
	app.Client().PostJSON("/v1/auth/reset-password", map[string]string{"token": token, "password": "a brand new passphrase"}).AssertError(t, http.StatusUnprocessableEntity, auth.CodeInvalidToken)

	// Password unchanged.
	app.Client().PostJSON("/v1/auth/login", map[string]string{"email": user.Email, "password": user.Password}).AssertStatus(t, http.StatusOK)
}

// --- end to end ------------------------------------------------------------

// TestAuth_DefinitionOfDoneFlow walks the whole lifecycle the template
// promises: sign up, receive the verification email, verify, log in, use a
// protected endpoint, log out, reset the password, log in again.
func TestAuth_DefinitionOfDoneFlow(t *testing.T) {
	app := testutil.NewApp(t)
	client, _ := signup(t, app, "flow@example.com")
	app.RunJobs()
	app.Client().PostJSON("/v1/auth/verify-email", map[string]string{"token": tokenFromEmail(t, app)}).AssertStatus(t, http.StatusNoContent)
	client.PostJSON("/v1/auth/logout", nil).AssertStatus(t, http.StatusNoContent)

	client = app.Client()
	client.PostJSON("/v1/auth/login", map[string]string{"email": "flow@example.com", "password": goodPassword}).AssertStatus(t, http.StatusOK)
	var me auth.UserResponse
	client.Get("/v1/auth/me").AssertStatus(t, http.StatusOK).DecodeJSON(t, &me)
	if !me.EmailVerified {
		t.Error("expected verified user")
	}
	client.PostJSON("/v1/auth/logout", nil).AssertStatus(t, http.StatusNoContent)

	app.Client().PostJSON("/v1/auth/forgot-password", map[string]string{"email": "flow@example.com"}).AssertStatus(t, http.StatusNoContent)
	app.RunJobs()
	app.Client().PostJSON("/v1/auth/reset-password", map[string]string{"token": tokenFromEmail(t, app), "password": "yet another passphrase"}).AssertStatus(t, http.StatusNoContent)
	app.Client().PostJSON("/v1/auth/login", map[string]string{"email": "flow@example.com", "password": "yet another passphrase"}).AssertStatus(t, http.StatusOK)
}
