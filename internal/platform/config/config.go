// Package config loads the process configuration from environment variables.
//
// Configuration is read once at startup and passed explicitly to the code
// that needs it. There is no package-level singleton and no automatic .env
// loading: the shell, the compose file, the Makefile or the orchestrator
// provides the values. Malformed values are startup errors, never silent
// fallbacks, and production refuses insecure defaults.
package config

import (
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Environment names the deployment context.
type Environment string

// Supported environments.
const (
	Development Environment = "development"
	Test        Environment = "test"
	Production  Environment = "production"
)

// Config is the complete configuration surface of every binary.
//
// Config must never be logged as a whole: it contains secrets.
type Config struct {
	Environment Environment
	Log         LogConfig
	HTTP        HTTPConfig
	Database    DatabaseConfig
	Auth        AuthConfig
	Email       EmailConfig
	Jobs        JobsConfig
	Sources     SourcesConfig
}

// LogConfig controls the structured logger.
type LogConfig struct {
	// Level is one of debug, info, warn, error.
	Level string
	// Format is json or text.
	Format string
}

// HTTPConfig controls the API server and its middleware.
type HTTPConfig struct {
	Addr              string
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
	// MaxBodyBytes is the default request body limit applied to every
	// request. Endpoints that need more opt in explicitly.
	MaxBodyBytes int64
	// AllowedOrigins lists browser origins allowed for CORS and accepted in
	// the Origin header of unsafe (state-changing) requests.
	AllowedOrigins []string
	// TrustedProxies lists CIDR ranges of reverse proxies whose
	// X-Forwarded-For header may be used to determine the client IP.
	TrustedProxies []*net.IPNet
	// HSTS emits Strict-Transport-Security. Only enable when the service is
	// reached exclusively over HTTPS.
	HSTS bool
}

// DatabaseConfig controls the PostgreSQL pool.
type DatabaseConfig struct {
	// URL is the connection string. It contains credentials: never log it.
	URL               string
	MaxConns          int32
	MinConns          int32
	MaxConnLifetime   time.Duration
	MaxConnIdleTime   time.Duration
	HealthCheckPeriod time.Duration
}

// DevEncryptionKeyHex is the well-known key in .env.example for local use.
// Production refuses it.
const DevEncryptionKeyHex = "626574656d706c6174652d6465762d6b65792d646f2d6e6f742d757365212121"

// AuthConfig controls sessions, security tokens and rate limits.
type AuthConfig struct {
	// EncryptionKey (32 bytes, AES-256-GCM) seals raw security tokens while
	// they travel through the job queue. Secret: never log it.
	EncryptionKey        []byte
	SessionTTL           time.Duration
	SessionCookieName    string
	SessionCookieSecure  bool
	VerificationTokenTTL time.Duration
	ResetTokenTTL        time.Duration
	// ResendCooldown is the minimum time between two verification emails
	// for the same user.
	ResendCooldown time.Duration
	// AppBaseURL is the frontend origin that verification and reset links
	// are built on.
	AppBaseURL string
	// RateLimit bounds requests per client IP on sensitive endpoints.
	RateLimit RateLimitConfig
}

// RateLimitConfig describes a fixed-window limit per client IP.
type RateLimitConfig struct {
	// Requests allowed per Window per client IP on each sensitive endpoint.
	Requests int
	Window   time.Duration
}

// EmailConfig controls outbound email.
type EmailConfig struct {
	// Provider is smtp or log. log writes messages to the logger and is only
	// allowed outside production.
	Provider string
	From     string
	SMTP     SMTPConfig
}

// SMTPConfig configures the SMTP sender.
type SMTPConfig struct {
	Host string
	Port int
	User string
	// Password is a secret: never log it.
	Password string
	// RequireTLS refuses to speak to a server without STARTTLS. Required in
	// production.
	RequireTLS bool
}

// JobsConfig controls the background job worker.
type JobsConfig struct {
	PollInterval time.Duration
	Concurrency  int
	// JobTimeout bounds one job handler execution.
	JobTimeout time.Duration
	// LockTimeout is how long a claimed job may stay running before another
	// worker may reclaim it (a crashed worker never releases its claim).
	LockTimeout time.Duration
	// ShutdownTimeout bounds how long in-flight jobs may finish on shutdown.
	ShutdownTimeout time.Duration
	// WorkerInAPI runs the worker loop inside the API process. Convenient
	// for local development; production normally runs cmd/worker.
	WorkerInAPI bool
}

// SourcesConfig configures the public data sources read by cmd/ingest.
type SourcesConfig struct {
	Arbetsmiljoverket ArbetsmiljoverketConfig
}

// ArbetsmiljoverketConfig configures the Arbetsmiljöverket web diary client.
type ArbetsmiljoverketConfig struct {
	// BaseURL is the origin of the web diary. It exists so tests can point
	// the client at a local server; production uses the public site.
	BaseURL string
	// RequestInterval is the minimum time between two requests to the
	// source, so a full ingestion run stays polite.
	RequestInterval time.Duration
}

// Load reads the configuration from the process environment.
func Load() (Config, error) {
	return LoadFrom(os.LookupEnv)
}

// Lookup resolves one environment variable, like os.LookupEnv.
type Lookup func(key string) (string, bool)

// LoadFrom reads the configuration through lookup. It exists so tests can
// load configuration without touching the process environment.
func LoadFrom(lookup Lookup) (Config, error) {
	e := &env{lookup: lookup}

	cfg := Config{}
	cfg.Environment = Environment(e.str("ENV", string(Development)))

	cfg.Log = LogConfig{
		Level:  e.str("LOG_LEVEL", "info"),
		Format: e.str("LOG_FORMAT", "text"),
	}

	cfg.HTTP = HTTPConfig{
		Addr:              e.str("HTTP_ADDR", ":8080"),
		ReadHeaderTimeout: e.duration("HTTP_READ_HEADER_TIMEOUT", 5*time.Second),
		ReadTimeout:       e.duration("HTTP_READ_TIMEOUT", 15*time.Second),
		WriteTimeout:      e.duration("HTTP_WRITE_TIMEOUT", 30*time.Second),
		IdleTimeout:       e.duration("HTTP_IDLE_TIMEOUT", 60*time.Second),
		ShutdownTimeout:   e.duration("HTTP_SHUTDOWN_TIMEOUT", 15*time.Second),
		MaxBodyBytes:      e.int64("HTTP_MAX_BODY_BYTES", 1<<20),
		AllowedOrigins:    e.list("HTTP_ALLOWED_ORIGINS", "http://localhost:3000,http://localhost:5173"),
		TrustedProxies:    e.cidrs("HTTP_TRUSTED_PROXIES", ""),
		HSTS:              e.bool("HTTP_HSTS", false),
	}

	cfg.Database = DatabaseConfig{
		URL:               e.str("DATABASE_URL", ""),
		MaxConns:          int32(e.int("DB_MAX_CONNS", 10)),
		MinConns:          int32(e.int("DB_MIN_CONNS", 0)),
		MaxConnLifetime:   e.duration("DB_MAX_CONN_LIFETIME", time.Hour),
		MaxConnIdleTime:   e.duration("DB_MAX_CONN_IDLE_TIME", 30*time.Minute),
		HealthCheckPeriod: e.duration("DB_HEALTH_CHECK_PERIOD", time.Minute),
	}

	cfg.Auth = AuthConfig{
		EncryptionKey:        e.hexKey("AUTH_ENCRYPTION_KEY", 32),
		SessionTTL:           e.duration("AUTH_SESSION_TTL", 30*24*time.Hour),
		SessionCookieName:    e.str("AUTH_SESSION_COOKIE_NAME", "session"),
		SessionCookieSecure:  e.bool("AUTH_SESSION_COOKIE_SECURE", cfg.Environment == Production),
		VerificationTokenTTL: e.duration("AUTH_VERIFICATION_TOKEN_TTL", 24*time.Hour),
		ResetTokenTTL:        e.duration("AUTH_RESET_TOKEN_TTL", time.Hour),
		ResendCooldown:       e.duration("AUTH_RESEND_COOLDOWN", time.Minute),
		AppBaseURL:           e.str("APP_BASE_URL", "http://localhost:3000"),
		RateLimit: RateLimitConfig{
			Requests: e.int("AUTH_RATE_LIMIT_REQUESTS", 10),
			Window:   e.duration("AUTH_RATE_LIMIT_WINDOW", time.Minute),
		},
	}

	cfg.Email = EmailConfig{
		Provider: e.str("EMAIL_PROVIDER", "smtp"),
		From:     e.str("EMAIL_FROM", "no-reply@localhost"),
		SMTP: SMTPConfig{
			Host:       e.str("SMTP_HOST", "localhost"),
			Port:       e.int("SMTP_PORT", 1025),
			User:       e.str("SMTP_USER", ""),
			Password:   e.str("SMTP_PASSWORD", ""),
			RequireTLS: e.bool("SMTP_REQUIRE_TLS", cfg.Environment == Production),
		},
	}

	cfg.Jobs = JobsConfig{
		PollInterval:    e.duration("JOBS_POLL_INTERVAL", time.Second),
		Concurrency:     e.int("JOBS_CONCURRENCY", 4),
		JobTimeout:      e.duration("JOBS_JOB_TIMEOUT", time.Minute),
		LockTimeout:     e.duration("JOBS_LOCK_TIMEOUT", 5*time.Minute),
		ShutdownTimeout: e.duration("JOBS_SHUTDOWN_TIMEOUT", 30*time.Second),
		WorkerInAPI:     e.bool("JOBS_WORKER_IN_API", false),
	}

	cfg.Sources = SourcesConfig{
		Arbetsmiljoverket: ArbetsmiljoverketConfig{
			BaseURL:         e.str("ARBETSMILJOVERKET_BASE_URL", "https://www.av.se"),
			RequestInterval: e.duration("ARBETSMILJOVERKET_REQUEST_INTERVAL", time.Second),
		},
	}

	if len(e.errs) > 0 {
		return Config{}, errors.Join(e.errs...)
	}
	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// IsProduction reports whether the process runs in production.
func (c Config) IsProduction() bool { return c.Environment == Production }

func (c Config) validate() error {
	var errs []error
	fail := func(format string, args ...any) {
		errs = append(errs, fmt.Errorf(format, args...))
	}

	switch c.Environment {
	case Development, Test, Production:
	default:
		fail("ENV must be development, test or production, got %q", c.Environment)
	}

	switch c.Log.Level {
	case "debug", "info", "warn", "error":
	default:
		fail("LOG_LEVEL must be debug, info, warn or error, got %q", c.Log.Level)
	}
	switch c.Log.Format {
	case "json", "text":
	default:
		fail("LOG_FORMAT must be json or text, got %q", c.Log.Format)
	}

	if c.HTTP.Addr == "" {
		fail("HTTP_ADDR must not be empty")
	}
	if c.HTTP.MaxBodyBytes <= 0 {
		fail("HTTP_MAX_BODY_BYTES must be positive")
	}
	for _, origin := range c.HTTP.AllowedOrigins {
		if err := validateOrigin(origin); err != nil {
			fail("HTTP_ALLOWED_ORIGINS: %w", err)
		}
	}

	if c.Database.URL == "" {
		fail("DATABASE_URL is required")
	} else if _, err := url.Parse(c.Database.URL); err != nil {
		fail("DATABASE_URL is not a valid URL")
	}
	if c.Database.MaxConns < 1 {
		fail("DB_MAX_CONNS must be at least 1")
	}
	if c.Database.MinConns < 0 || c.Database.MinConns > c.Database.MaxConns {
		fail("DB_MIN_CONNS must be between 0 and DB_MAX_CONNS")
	}

	if len(c.Auth.EncryptionKey) == 0 {
		fail("AUTH_ENCRYPTION_KEY is required (32 bytes, hex encoded)")
	}
	if c.Auth.SessionTTL <= 0 {
		fail("AUTH_SESSION_TTL must be positive")
	}
	if c.Auth.SessionCookieName == "" {
		fail("AUTH_SESSION_COOKIE_NAME must not be empty")
	}
	if c.Auth.VerificationTokenTTL <= 0 || c.Auth.ResetTokenTTL <= 0 {
		fail("AUTH_VERIFICATION_TOKEN_TTL and AUTH_RESET_TOKEN_TTL must be positive")
	}
	if c.Auth.RateLimit.Requests < 1 || c.Auth.RateLimit.Window <= 0 {
		fail("AUTH_RATE_LIMIT_REQUESTS must be at least 1 and AUTH_RATE_LIMIT_WINDOW positive")
	}
	if err := validateOrigin(c.Auth.AppBaseURL); err != nil {
		fail("APP_BASE_URL: %w", err)
	}

	switch c.Email.Provider {
	case "smtp":
		if c.Email.SMTP.Host == "" || c.Email.SMTP.Port < 1 || c.Email.SMTP.Port > 65535 {
			fail("SMTP_HOST and SMTP_PORT are required when EMAIL_PROVIDER=smtp")
		}
	case "log":
	default:
		fail("EMAIL_PROVIDER must be smtp or log, got %q", c.Email.Provider)
	}
	if c.Email.From == "" {
		fail("EMAIL_FROM must not be empty")
	}

	if c.Jobs.PollInterval <= 0 {
		fail("JOBS_POLL_INTERVAL must be positive")
	}
	if c.Jobs.Concurrency < 1 {
		fail("JOBS_CONCURRENCY must be at least 1")
	}
	if c.Jobs.JobTimeout <= 0 || c.Jobs.LockTimeout <= 0 || c.Jobs.ShutdownTimeout <= 0 {
		fail("JOBS_JOB_TIMEOUT, JOBS_LOCK_TIMEOUT and JOBS_SHUTDOWN_TIMEOUT must be positive")
	}
	if c.Jobs.LockTimeout < c.Jobs.JobTimeout {
		fail("JOBS_LOCK_TIMEOUT must not be shorter than JOBS_JOB_TIMEOUT")
	}

	if err := validateOrigin(c.Sources.Arbetsmiljoverket.BaseURL); err != nil {
		fail("ARBETSMILJOVERKET_BASE_URL: %w", err)
	}
	if c.Sources.Arbetsmiljoverket.RequestInterval < 0 {
		fail("ARBETSMILJOVERKET_REQUEST_INTERVAL must not be negative")
	}

	if c.Environment == Production {
		errs = append(errs, c.validateProduction()...)
	}

	return errors.Join(errs...)
}

// validateProduction refuses defaults that are only acceptable locally.
func (c Config) validateProduction() []error {
	var errs []error
	fail := func(format string, args ...any) {
		errs = append(errs, fmt.Errorf(format, args...))
	}

	if !c.Auth.SessionCookieSecure {
		fail("AUTH_SESSION_COOKIE_SECURE must be true in production")
	}
	if hex.EncodeToString(c.Auth.EncryptionKey) == DevEncryptionKeyHex {
		fail("AUTH_ENCRYPTION_KEY must not be the development key in production")
	}
	if c.Jobs.WorkerInAPI {
		fail("JOBS_WORKER_IN_API must be false in production; run cmd/worker separately")
	}
	if !strings.HasPrefix(c.Auth.AppBaseURL, "https://") {
		fail("APP_BASE_URL must use https in production")
	}
	for _, origin := range c.HTTP.AllowedOrigins {
		if !strings.HasPrefix(origin, "https://") {
			fail("HTTP_ALLOWED_ORIGINS must only contain https origins in production, got %q", origin)
		}
	}
	if c.Email.Provider != "smtp" {
		fail("EMAIL_PROVIDER must be smtp in production")
	}
	if !c.Email.SMTP.RequireTLS {
		fail("SMTP_REQUIRE_TLS must be true in production")
	}
	if c.Log.Format != "json" {
		fail("LOG_FORMAT must be json in production")
	}
	if u, err := url.Parse(c.Database.URL); err == nil {
		mode := u.Query().Get("sslmode")
		switch mode {
		case "require", "verify-ca", "verify-full":
		default:
			fail("DATABASE_URL must set sslmode=require or stronger in production")
		}
	}
	return errs
}

func validateOrigin(origin string) error {
	u, err := url.Parse(origin)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.Path != "" || u.RawQuery != "" {
		return fmt.Errorf("%q is not an origin of the form scheme://host[:port]", origin)
	}
	return nil
}

// env collects parse errors so that every problem is reported at once.
type env struct {
	lookup Lookup
	errs   []error
}

func (e *env) raw(key string) (string, bool) {
	v, ok := e.lookup(key)
	if !ok {
		return "", false
	}
	v = strings.TrimSpace(v)
	if v == "" {
		return "", false
	}
	return v, true
}

func (e *env) fail(key string, err error) {
	e.errs = append(e.errs, fmt.Errorf("%s: %w", key, err))
}

func (e *env) str(key, def string) string {
	if v, ok := e.raw(key); ok {
		return v
	}
	return def
}

func (e *env) int(key string, def int) int {
	v, ok := e.raw(key)
	if !ok {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		e.fail(key, fmt.Errorf("%q is not an integer", v))
		return def
	}
	return n
}

func (e *env) int64(key string, def int64) int64 {
	v, ok := e.raw(key)
	if !ok {
		return def
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		e.fail(key, fmt.Errorf("%q is not an integer", v))
		return def
	}
	return n
}

func (e *env) bool(key string, def bool) bool {
	v, ok := e.raw(key)
	if !ok {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		e.fail(key, fmt.Errorf("%q is not a boolean", v))
		return def
	}
	return b
}

func (e *env) duration(key string, def time.Duration) time.Duration {
	v, ok := e.raw(key)
	if !ok {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		e.fail(key, fmt.Errorf("%q is not a duration such as 30s or 5m", v))
		return def
	}
	return d
}

// hexKey parses a hex-encoded key of exactly size bytes. Missing keys are
// reported by validate so the message can say what the key is for.
func (e *env) hexKey(key string, size int) []byte {
	v, ok := e.raw(key)
	if !ok {
		return nil
	}
	b, err := hex.DecodeString(v)
	if err != nil {
		e.fail(key, errors.New("must be hex encoded"))
		return nil
	}
	if len(b) != size {
		e.fail(key, fmt.Errorf("must decode to %d bytes, got %d", size, len(b)))
		return nil
	}
	return b
}

func (e *env) list(key, def string) []string {
	v := e.str(key, def)
	var out []string
	for _, item := range strings.Split(v, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

func (e *env) cidrs(key, def string) []*net.IPNet {
	var out []*net.IPNet
	for _, item := range e.list(key, def) {
		if !strings.Contains(item, "/") {
			if strings.Contains(item, ":") {
				item += "/128"
			} else {
				item += "/32"
			}
		}
		_, ipNet, err := net.ParseCIDR(item)
		if err != nil {
			e.fail(key, fmt.Errorf("%q is not an IP address or CIDR range", item))
			continue
		}
		out = append(out, ipNet)
	}
	return out
}
