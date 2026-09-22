package testutil

import (
	"bytes"
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"betemplate/internal/account"
	"betemplate/internal/app"
	"betemplate/internal/auth"
	"betemplate/internal/platform/config"
	"betemplate/internal/platform/database"
	"betemplate/internal/platform/email"
	"betemplate/internal/platform/jobs"
	"betemplate/internal/platform/logging"
	"betemplate/internal/platform/metrics"
	"betemplate/internal/platform/password"
	"betemplate/internal/platform/testutil/pgtest"
)

// AllowedOrigin is the browser origin the test configuration trusts.
const AllowedOrigin = "http://app.test"

// DefaultPassword is the password of users created by the factories.
const DefaultPassword = "correct horse battery staple"

// App is a fully wired application on an isolated database: the real
// router, middleware, services and repositories, with a recording email
// sender in place of SMTP.
type App struct {
	t       testing.TB
	Config  config.Config
	Pool    *pgxpool.Pool
	Router  http.Handler
	Email   *email.Recorder
	Worker  *jobs.Worker
	Auth    *auth.Service
	Metrics *metrics.Metrics

	logs *lockedBuffer
}

// Option adjusts the test configuration before the app is wired.
type Option func(*config.Config)

// WithConfig applies fn to the test configuration.
func WithConfig(fn func(cfg *config.Config)) Option { return fn }

// NewApp builds an App on a fresh migrated database. It skips the test when
// TEST_DATABASE_URL is unset.
func NewApp(t testing.TB, opts ...Option) *App {
	t.Helper()
	pool := pgtest.NewPool(t)

	cfg, err := config.LoadFrom(func(key string) (string, bool) {
		v, ok := testEnv[key]
		return v, ok
	})
	if err != nil {
		t.Fatalf("testutil: load test config: %v", err)
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	logs := &lockedBuffer{}
	recorder := &email.Recorder{}
	m := &metrics.Metrics{}
	application, err := app.New(app.Deps{
		Config:  cfg,
		Logger:  logging.New(logs, "debug", "json"),
		Pool:    pool,
		Metrics: m,
		Email:   recorder,
	})
	if err != nil {
		t.Fatalf("testutil: wire app: %v", err)
	}

	return &App{
		t:       t,
		Config:  cfg,
		Pool:    pool,
		Router:  application.Router,
		Email:   recorder,
		Worker:  application.Worker,
		Auth:    application.Auth,
		Metrics: m,
		logs:    logs,
	}
}

// testEnv is the baseline configuration for tests. Rate limits are generous
// so ordinary tests never hit them; rate-limit tests lower them explicitly.
var testEnv = map[string]string{
	"ENV":                      "test",
	"LOG_LEVEL":                "debug",
	"LOG_FORMAT":               "json",
	"DATABASE_URL":             "postgres://unused@localhost/unused",
	"HTTP_ALLOWED_ORIGINS":     AllowedOrigin,
	"AUTH_ENCRYPTION_KEY":      config.DevEncryptionKeyHex,
	"APP_BASE_URL":             AllowedOrigin,
	"EMAIL_PROVIDER":           "log",
	"AUTH_RATE_LIMIT_REQUESTS": "10000",
	"JOBS_POLL_INTERVAL":       "10ms",
	"JOBS_CONCURRENCY":         "2",
}

// Logs returns everything the application logged so far.
func (a *App) Logs() string { return a.logs.String() }

// RunJobs executes every runnable job once and returns how many ran.
func (a *App) RunJobs() int {
	a.t.Helper()
	n, err := a.Worker.RunOnce(context.Background())
	if err != nil {
		a.t.Fatalf("testutil: run jobs: %v", err)
	}
	return n
}

// Jobs returns every job of a type, oldest first.
func (a *App) Jobs(jobType string) []jobs.Job {
	a.t.Helper()
	list, err := jobs.ListByType(context.Background(), a.Pool, jobType)
	if err != nil {
		a.t.Fatalf("testutil: list jobs: %v", err)
	}
	return list
}

// User is a factory-created user with its personal account and the plain
// password that logs it in.
type User struct {
	ID        uuid.UUID
	Email     string
	Password  string
	AccountID uuid.UUID
}

// UserOption adjusts CreateUser.
type UserOption func(*userOptions)

type userOptions struct {
	email      string
	password   string
	unverified bool
}

// WithEmail sets the user's email.
func WithEmail(email string) UserOption { return func(o *userOptions) { o.email = email } }

// WithPassword sets the user's password.
func WithPassword(pw string) UserOption { return func(o *userOptions) { o.password = pw } }

// Unverified leaves the email unverified.
func Unverified() UserOption { return func(o *userOptions) { o.unverified = true } }

var userSeq struct {
	sync.Mutex
	n int
}

func nextEmail() string {
	userSeq.Lock()
	defer userSeq.Unlock()
	userSeq.n++
	return "user" + uuid.NewString()[:8] + "@example.com"
}

// CreateUser inserts a verified user with a personal account and an owner
// membership, bypassing signup so no jobs or emails are produced.
func (a *App) CreateUser(opts ...UserOption) User {
	a.t.Helper()
	o := userOptions{email: nextEmail(), password: DefaultPassword}
	for _, opt := range opts {
		opt(&o)
	}
	hash, err := password.Hash(o.password)
	if err != nil {
		a.t.Fatalf("testutil: hash password: %v", err)
	}

	ctx := context.Background()
	authRepo := auth.NewRepository()
	accountRepo := account.NewRepository()
	var user User
	err = database.InTx(ctx, a.Pool, func(tx pgx.Tx) error {
		u, err := authRepo.CreateUser(ctx, tx, o.email, hash)
		if err != nil {
			return err
		}
		if !o.unverified {
			if _, err := authRepo.MarkEmailVerified(ctx, tx, u.ID, time.Now().UTC()); err != nil {
				return err
			}
		}
		acc, err := accountRepo.Create(ctx, tx, o.email)
		if err != nil {
			return err
		}
		if _, err := accountRepo.AddMembership(ctx, tx, acc.ID, u.ID, account.RoleOwner); err != nil {
			return err
		}
		user = User{ID: u.ID, Email: u.Email, Password: o.password, AccountID: acc.ID}
		return nil
	})
	if err != nil {
		a.t.Fatalf("testutil: create user: %v", err)
	}
	return user
}

// CreateAccount inserts an account without members.
func (a *App) CreateAccount(name string) account.Account {
	a.t.Helper()
	acc, err := account.NewRepository().Create(context.Background(), a.Pool, name)
	if err != nil {
		a.t.Fatalf("testutil: create account: %v", err)
	}
	return acc
}

// AddMembership makes user a member of accountID with role.
func (a *App) AddMembership(user User, accountID uuid.UUID, role account.Role) {
	a.t.Helper()
	if _, err := account.NewRepository().AddMembership(context.Background(), a.Pool, accountID, user.ID, role); err != nil {
		a.t.Fatalf("testutil: add membership: %v", err)
	}
}

// Client returns an unauthenticated HTTP client for the router.
func (a *App) Client() *Client {
	return &Client{
		app:        a,
		Origin:     AllowedOrigin,
		RemoteAddr: "203.0.113.10:12345",
		cookies:    map[string]*http.Cookie{},
	}
}

// AuthenticatedClient returns a client holding a real session cookie for
// user, created through the same service production login uses.
func (a *App) AuthenticatedClient(user User) *Client {
	a.t.Helper()
	sess, token, err := a.Auth.CreateSession(context.Background(), a.Pool, user.ID)
	if err != nil {
		a.t.Fatalf("testutil: create session: %v", err)
	}
	c := a.Client()
	c.SessionID = sess.ID
	c.SetCookie(a.Auth.SessionCookie(token, sess.ExpiresAt))
	return c
}

// lockedBuffer is a bytes.Buffer safe for concurrent writers.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
