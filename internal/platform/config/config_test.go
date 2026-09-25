package config

import (
	"strings"
	"testing"
	"time"
)

func lookupFrom(values map[string]string) Lookup {
	if _, ok := values["AUTH_ENCRYPTION_KEY"]; !ok {
		values["AUTH_ENCRYPTION_KEY"] = DevEncryptionKeyHex
	}
	return func(key string) (string, bool) {
		v, ok := values[key]
		return v, ok
	}
}

const prodKey = "0f1e2d3c4b5a69788796a5b4c3d2e1f00f1e2d3c4b5a69788796a5b4c3d2e1f0"

func TestLoadFrom_EncryptionKey(t *testing.T) {
	base := func() map[string]string {
		return map[string]string{"DATABASE_URL": "postgres://app:app@localhost:5432/app"}
	}
	tests := map[string]string{
		"missing":   "",
		"not hex":   "zz",
		"too short": "0011",
		"too long":  prodKey + "00",
	}
	for name, value := range tests {
		t.Run(name, func(t *testing.T) {
			values := base()
			values["AUTH_ENCRYPTION_KEY"] = value
			_, err := LoadFrom(lookupFrom(values))
			if err == nil || !strings.Contains(err.Error(), "AUTH_ENCRYPTION_KEY") {
				t.Fatalf("expected AUTH_ENCRYPTION_KEY error, got %v", err)
			}
		})
	}
	values := base()
	values["AUTH_ENCRYPTION_KEY"] = prodKey
	cfg, err := LoadFrom(lookupFrom(values))
	if err != nil || len(cfg.Auth.EncryptionKey) != 32 {
		t.Fatalf("valid key: %v, %d bytes", err, len(cfg.Auth.EncryptionKey))
	}
}

func TestLoadFrom_ProductionRejectsDevKeyAndEmbeddedWorker(t *testing.T) {
	values := map[string]string{
		"ENV":                  "production",
		"LOG_FORMAT":           "json",
		"DATABASE_URL":         "postgres://app:secret@db:5432/app?sslmode=require",
		"APP_BASE_URL":         "https://app.example.com",
		"HTTP_ALLOWED_ORIGINS": "https://app.example.com",
		"AUTH_ENCRYPTION_KEY":  DevEncryptionKeyHex,
		"JOBS_WORKER_IN_API":   "true",
	}
	_, err := LoadFrom(lookupFrom(values))
	if err == nil {
		t.Fatal("expected errors")
	}
	for _, want := range []string{"AUTH_ENCRYPTION_KEY must not be the development key", "JOBS_WORKER_IN_API must be false in production"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not mention %q:\n%v", want, err)
		}
	}
	values["AUTH_ENCRYPTION_KEY"] = prodKey
	values["JOBS_WORKER_IN_API"] = "false"
	if _, err := LoadFrom(lookupFrom(values)); err != nil {
		t.Fatalf("expected valid production config, got %v", err)
	}
}

func TestLoadFrom_DevelopmentDefaults(t *testing.T) {
	cfg, err := LoadFrom(lookupFrom(map[string]string{
		"DATABASE_URL": "postgres://app:app@localhost:5432/app?sslmode=disable",
	}))
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	if cfg.Environment != Development {
		t.Errorf("Environment = %q, want development", cfg.Environment)
	}
	if cfg.HTTP.Addr != ":8080" {
		t.Errorf("HTTP.Addr = %q", cfg.HTTP.Addr)
	}
	if cfg.HTTP.MaxBodyBytes != 1<<20 {
		t.Errorf("MaxBodyBytes = %d", cfg.HTTP.MaxBodyBytes)
	}
	if cfg.Auth.SessionCookieSecure {
		t.Error("cookie should not be secure by default in development")
	}
	if len(cfg.HTTP.AllowedOrigins) != 2 {
		t.Errorf("AllowedOrigins = %v", cfg.HTTP.AllowedOrigins)
	}
	if cfg.Jobs.Concurrency != 4 {
		t.Errorf("Jobs.Concurrency = %d", cfg.Jobs.Concurrency)
	}
}

func TestLoadFrom_ParsesValues(t *testing.T) {
	cfg, err := LoadFrom(lookupFrom(map[string]string{
		"DATABASE_URL":         "postgres://app:app@localhost:5432/app?sslmode=disable",
		"HTTP_ADDR":            ":9090",
		"HTTP_READ_TIMEOUT":    "2s",
		"HTTP_ALLOWED_ORIGINS": " http://a.test , http://b.test:3000 ",
		"HTTP_TRUSTED_PROXIES": "10.0.0.0/8, 127.0.0.1",
		"DB_MAX_CONNS":         "3",
		"JOBS_WORKER_IN_API":   "true",
		"AUTH_SESSION_TTL":     "1h",
	}))
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	if cfg.HTTP.Addr != ":9090" {
		t.Errorf("Addr = %q", cfg.HTTP.Addr)
	}
	if cfg.HTTP.ReadTimeout != 2*time.Second {
		t.Errorf("ReadTimeout = %v", cfg.HTTP.ReadTimeout)
	}
	if got := cfg.HTTP.AllowedOrigins; len(got) != 2 || got[0] != "http://a.test" || got[1] != "http://b.test:3000" {
		t.Errorf("AllowedOrigins = %v", got)
	}
	if len(cfg.HTTP.TrustedProxies) != 2 || cfg.HTTP.TrustedProxies[1].String() != "127.0.0.1/32" {
		t.Errorf("TrustedProxies = %v", cfg.HTTP.TrustedProxies)
	}
	if cfg.Database.MaxConns != 3 {
		t.Errorf("MaxConns = %d", cfg.Database.MaxConns)
	}
	if !cfg.Jobs.WorkerInAPI {
		t.Error("WorkerInAPI should be true")
	}
	if cfg.Auth.SessionTTL != time.Hour {
		t.Errorf("SessionTTL = %v", cfg.Auth.SessionTTL)
	}
}

func TestLoadFrom_ReportsEveryMalformedValue(t *testing.T) {
	_, err := LoadFrom(lookupFrom(map[string]string{
		"DATABASE_URL":       "postgres://app:app@localhost:5432/app",
		"HTTP_READ_TIMEOUT":  "soon",
		"DB_MAX_CONNS":       "many",
		"JOBS_WORKER_IN_API": "maybe",
	}))
	if err == nil {
		t.Fatal("expected error")
	}
	for _, want := range []string{"HTTP_READ_TIMEOUT", "DB_MAX_CONNS", "JOBS_WORKER_IN_API"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %s", err, want)
		}
	}
}

func TestLoadFrom_RequiresDatabaseURL(t *testing.T) {
	_, err := LoadFrom(lookupFrom(map[string]string{}))
	if err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("expected DATABASE_URL error, got %v", err)
	}
}

func TestLoadFrom_RejectsInvalidEnums(t *testing.T) {
	tests := map[string]map[string]string{
		"ENV":            {"ENV": "staging"},
		"LOG_LEVEL":      {"LOG_LEVEL": "loud"},
		"LOG_FORMAT":     {"LOG_FORMAT": "xml"},
		"EMAIL_PROVIDER": {"EMAIL_PROVIDER": "pigeon"},
		"APP_BASE_URL":   {"APP_BASE_URL": "localhost:3000"},
	}
	for name, values := range tests {
		t.Run(name, func(t *testing.T) {
			values["DATABASE_URL"] = "postgres://app:app@localhost:5432/app"
			_, err := LoadFrom(lookupFrom(values))
			if err == nil || !strings.Contains(err.Error(), name) {
				t.Fatalf("expected error mentioning %s, got %v", name, err)
			}
		})
	}
}

func TestLoadFrom_ProductionRefusesInsecureDefaults(t *testing.T) {
	_, err := LoadFrom(lookupFrom(map[string]string{
		"ENV":          "production",
		"DATABASE_URL": "postgres://app:app@db:5432/app?sslmode=disable",
	}))
	if err == nil {
		t.Fatal("expected production validation errors")
	}
	for _, want := range []string{
		"APP_BASE_URL must use https",
		"HTTP_ALLOWED_ORIGINS must only contain https",
		"LOG_FORMAT must be json",
		"sslmode=require",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not mention %q:\n%v", want, err)
		}
	}
}

func TestLoadFrom_ProductionAcceptsSecureConfiguration(t *testing.T) {
	cfg, err := LoadFrom(lookupFrom(map[string]string{
		"ENV":                  "production",
		"LOG_FORMAT":           "json",
		"DATABASE_URL":         "postgres://app:secret@db:5432/app?sslmode=require",
		"APP_BASE_URL":         "https://app.example.com",
		"HTTP_ALLOWED_ORIGINS": "https://app.example.com",
		"AUTH_ENCRYPTION_KEY":  prodKey,
		"SMTP_HOST":            "smtp.example.com",
		"SMTP_PORT":            "587",
		"SMTP_USER":            "user",
		"SMTP_PASSWORD":        "secret",
		"EMAIL_FROM":           "no-reply@example.com",
	}))
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	if !cfg.Auth.SessionCookieSecure {
		t.Error("cookie must be secure in production")
	}
	if !cfg.Email.SMTP.RequireTLS {
		t.Error("SMTP must require TLS in production")
	}
	if !cfg.IsProduction() {
		t.Error("IsProduction should be true")
	}
}

func TestLoadFrom_ProductionCannotDisableSecureCookie(t *testing.T) {
	_, err := LoadFrom(lookupFrom(map[string]string{
		"ENV":                        "production",
		"LOG_FORMAT":                 "json",
		"DATABASE_URL":               "postgres://app:secret@db:5432/app?sslmode=require",
		"APP_BASE_URL":               "https://app.example.com",
		"HTTP_ALLOWED_ORIGINS":       "https://app.example.com",
		"AUTH_ENCRYPTION_KEY":        prodKey,
		"AUTH_SESSION_COOKIE_SECURE": "false",
	}))
	if err == nil || !strings.Contains(err.Error(), "AUTH_SESSION_COOKIE_SECURE") {
		t.Fatalf("expected cookie error, got %v", err)
	}
}

func TestLoadFrom_SourcesDefaultsAndValidation(t *testing.T) {
	cfg, err := LoadFrom(lookupFrom(map[string]string{
		"DATABASE_URL": "postgres://app:app@localhost:5432/app?sslmode=disable",
	}))
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	if cfg.Sources.Arbetsmiljoverket.BaseURL != "https://www.av.se" || cfg.Sources.Arbetsmiljoverket.RequestInterval != time.Second {
		t.Errorf("defaults = %+v", cfg.Sources.Arbetsmiljoverket)
	}
	if cfg.Sources.Stockholm.BaseURL != "https://etjanster.stockholm.se" || cfg.Sources.Stockholm.RequestInterval != time.Second || cfg.Sources.Stockholm.RefreshInterval != 7*24*time.Hour {
		t.Errorf("stockholm defaults = %+v", cfg.Sources.Stockholm)
	}

	cfg, err = LoadFrom(lookupFrom(map[string]string{
		"DATABASE_URL":                       "postgres://app:app@localhost:5432/app?sslmode=disable",
		"ARBETSMILJOVERKET_BASE_URL":         "http://127.0.0.1:9999",
		"ARBETSMILJOVERKET_REQUEST_INTERVAL": "0",
		"STOCKHOLM_BASE_URL":                 "http://127.0.0.1:9998",
		"STOCKHOLM_REQUEST_INTERVAL":         "0",
		"STOCKHOLM_REFRESH_INTERVAL":         "48h",
	}))
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	if cfg.Sources.Arbetsmiljoverket.BaseURL != "http://127.0.0.1:9999" || cfg.Sources.Arbetsmiljoverket.RequestInterval != 0 {
		t.Errorf("overrides = %+v", cfg.Sources.Arbetsmiljoverket)
	}
	if cfg.Sources.Stockholm.BaseURL != "http://127.0.0.1:9998" || cfg.Sources.Stockholm.RequestInterval != 0 || cfg.Sources.Stockholm.RefreshInterval != 48*time.Hour {
		t.Errorf("stockholm overrides = %+v", cfg.Sources.Stockholm)
	}

	for name, values := range map[string]map[string]string{
		"base url with path": {"ARBETSMILJOVERKET_BASE_URL": "https://www.av.se/diarium/"},
		"base url not a url": {"ARBETSMILJOVERKET_BASE_URL": "av.se"},
		"negative interval":  {"ARBETSMILJOVERKET_REQUEST_INTERVAL": "-1s"},
		"malformed interval": {"ARBETSMILJOVERKET_REQUEST_INTERVAL": "soon"},
	} {
		t.Run(name, func(t *testing.T) {
			values["DATABASE_URL"] = "postgres://app:app@localhost:5432/app?sslmode=disable"
			_, err := LoadFrom(lookupFrom(values))
			if err == nil || !strings.Contains(err.Error(), "ARBETSMILJOVERKET_") {
				t.Fatalf("expected an ARBETSMILJOVERKET_ error, got %v", err)
			}
		})
	}
	for name, values := range map[string]map[string]string{
		"base url with path":        {"STOCKHOLM_BASE_URL": "https://etjanster.stockholm.se/Byggochplantjansten/"},
		"base url not a url":        {"STOCKHOLM_BASE_URL": "etjanster.stockholm.se"},
		"negative interval":         {"STOCKHOLM_REQUEST_INTERVAL": "-1s"},
		"malformed interval":        {"STOCKHOLM_REQUEST_INTERVAL": "soon"},
		"zero refresh interval":     {"STOCKHOLM_REFRESH_INTERVAL": "0"},
		"negative refresh interval": {"STOCKHOLM_REFRESH_INTERVAL": "-24h"},
	} {
		t.Run("stockholm "+name, func(t *testing.T) {
			values["DATABASE_URL"] = "postgres://app:app@localhost:5432/app?sslmode=disable"
			_, err := LoadFrom(lookupFrom(values))
			if err == nil || !strings.Contains(err.Error(), "STOCKHOLM_") {
				t.Fatalf("expected a STOCKHOLM_ error, got %v", err)
			}
		})
	}
}
