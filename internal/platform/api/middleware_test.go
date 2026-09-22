package api_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"betemplate/internal/platform/api"
	"betemplate/internal/platform/apperror"
	"betemplate/internal/platform/config"
	"betemplate/internal/platform/logging"
	"betemplate/internal/platform/metrics"
	"betemplate/internal/platform/ratelimit"
)

func testRouterConfig(logBuf *bytes.Buffer) api.RouterConfig {
	w := io.Discard
	if logBuf != nil {
		w = logBuf
	}
	_, private, _ := net.ParseCIDR("10.0.0.0/8")
	return api.RouterConfig{
		Logger: logging.New(w, "debug", "json"),
		HTTP: config.HTTPConfig{
			MaxBodyBytes:   1 << 20,
			AllowedOrigins: []string{"http://app.test"},
			TrustedProxies: []*net.IPNet{private},
		},
		Metrics: &metrics.Metrics{},
	}
}

func decodeError(t *testing.T, rec *httptest.ResponseRecorder) api.ErrorResponse {
	t.Helper()
	var body api.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error body %q: %v", rec.Body.String(), err)
	}
	return body
}

func TestRequestID_GeneratedAndEchoed(t *testing.T) {
	r := api.NewRouter(testRouterConfig(nil))
	var seen string
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		seen = api.RequestIDFromContext(r.Context())
	})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if got := rec.Header().Get("X-Request-ID"); got == "" || got != seen {
		t.Fatalf("X-Request-ID = %q, context = %q", got, seen)
	}

	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", "client-id.1")
	r.ServeHTTP(rec, req)
	if rec.Header().Get("X-Request-ID") != "client-id.1" {
		t.Errorf("valid client request ID should be kept, got %q", rec.Header().Get("X-Request-ID"))
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", strings.Repeat("x", 65)+" <script>")
	r.ServeHTTP(rec, req)
	if got := rec.Header().Get("X-Request-ID"); len(got) != 24 {
		t.Errorf("invalid client request ID should be replaced, got %q", got)
	}
}

func TestRequestLogger_WritesStructuredLineWithScopeAttributes(t *testing.T) {
	var buf bytes.Buffer
	r := api.NewRouter(testRouterConfig(&buf))
	r.Post("/things", func(w http.ResponseWriter, r *http.Request) {
		logging.Add(r.Context(), "user_id", "u1")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("ok"))
	})
	req := httptest.NewRequest(http.MethodPost, "/things", nil)
	req.RemoteAddr = "203.0.113.9:4444"
	r.ServeHTTP(httptest.NewRecorder(), req)

	var line map[string]any
	for _, raw := range bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n")) {
		var candidate map[string]any
		if err := json.Unmarshal(raw, &candidate); err == nil && candidate["msg"] == "request" {
			line = candidate
		}
	}
	if line == nil {
		t.Fatalf("no request log line in %s", buf.String())
	}
	for key, want := range map[string]any{
		"method": "POST", "path": "/things", "status": float64(201), "user_id": "u1", "client_ip": "203.0.113.9", "bytes": float64(2),
	} {
		if line[key] != want {
			t.Errorf("%s = %v, want %v", key, line[key], want)
		}
	}
	if line["request_id"] == nil || line["duration_ms"] == nil {
		t.Errorf("missing request_id or duration_ms: %v", line)
	}
}

func TestRecover_ReturnsGeneric500(t *testing.T) {
	var buf bytes.Buffer
	cfg := testRouterConfig(&buf)
	r := api.NewRouter(cfg)
	r.Get("/boom", func(w http.ResponseWriter, r *http.Request) {
		panic("secret internal detail")
	})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/boom", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rec.Code)
	}
	body := decodeError(t, rec)
	if body.Error.Code != apperror.CodeInternal {
		t.Errorf("code = %q", body.Error.Code)
	}
	if strings.Contains(rec.Body.String(), "secret internal detail") {
		t.Error("panic detail leaked to the client")
	}
	if !strings.Contains(buf.String(), "secret internal detail") || !strings.Contains(buf.String(), "panic recovered") {
		t.Error("panic should be logged with its detail")
	}
	if cfg.Metrics.Snapshot()["http_request_errors_total"] != 1 {
		t.Error("500 should be counted as an error")
	}
}

// errorLines returns the error-level log records in buf.
func errorLines(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, raw := range bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n")) {
		if len(raw) == 0 {
			continue
		}
		var line map[string]any
		if err := json.Unmarshal(raw, &line); err != nil {
			t.Fatalf("decode log line %s: %v", raw, err)
		}
		if line["level"] == "ERROR" {
			out = append(out, line)
		}
	}
	return out
}

func TestPanic_IsLoggedExactlyOnce(t *testing.T) {
	var buf bytes.Buffer
	r := api.NewRouter(testRouterConfig(&buf))
	r.Get("/boom", func(w http.ResponseWriter, r *http.Request) { panic("kaboom detail") })
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/boom", nil))

	errs := errorLines(t, &buf)
	if len(errs) != 1 {
		t.Fatalf("error-level log lines = %d, want 1:\n%s", len(errs), buf.String())
	}
	if errs[0]["msg"] != "panic recovered" || errs[0]["stack"] == nil || !strings.Contains(fmt.Sprint(errs[0]["panic"]), "kaboom detail") {
		t.Errorf("diagnostic line = %v", errs[0])
	}
	if !strings.Contains(buf.String(), `"msg":"request"`) || !strings.Contains(buf.String(), `"status":500`) {
		t.Error("access log line with status 500 missing")
	}
}

func TestUnexpectedError_IsLoggedExactlyOnce(t *testing.T) {
	var buf bytes.Buffer
	r := api.NewRouter(testRouterConfig(&buf))
	r.Get("/fail", func(w http.ResponseWriter, r *http.Request) {
		api.WriteError(r.Context(), w, errors.New("pq: connection refused"))
	})
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/fail", nil))

	errs := errorLines(t, &buf)
	if len(errs) != 1 {
		t.Fatalf("error-level log lines = %d, want 1:\n%s", len(errs), buf.String())
	}
	if errs[0]["msg"] != "request failed" || !strings.Contains(fmt.Sprint(errs[0]["error"]), "connection refused") {
		t.Errorf("diagnostic line = %v", errs[0])
	}
	if !strings.Contains(buf.String(), `"msg":"request"`) || !strings.Contains(buf.String(), `"status":500`) {
		t.Error("access log line with status 500 missing")
	}
}

func TestRealIP_TrustsForwardedHeaderOnlyFromTrustedProxies(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		xff        string
		want       string
	}{
		{"direct client", "203.0.113.9:1234", "", "203.0.113.9"},
		{"direct client ignores header", "203.0.113.9:1234", "198.51.100.1", "203.0.113.9"},
		{"trusted proxy uses header", "10.1.2.3:1234", "198.51.100.1", "198.51.100.1"},
		{"trusted proxy skips proxy chain", "10.1.2.3:1234", "198.51.100.1, 10.9.9.9", "198.51.100.1"},
		{"trusted proxy takes rightmost untrusted", "10.1.2.3:1234", "1.1.1.1, 198.51.100.1", "198.51.100.1"},
		{"trusted proxy without header", "10.1.2.3:1234", "", "10.1.2.3"},
		{"trusted proxy with garbage header", "10.1.2.3:1234", "not-an-ip", "10.1.2.3"},
		{"ipv6 client", "[2001:db8::1]:1234", "", "2001:db8::1"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := api.NewRouter(testRouterConfig(nil))
			var got string
			r.Get("/", func(w http.ResponseWriter, r *http.Request) { got = api.ClientIP(r.Context()) })
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tc.remoteAddr
			if tc.xff != "" {
				req.Header.Set("X-Forwarded-For", tc.xff)
			}
			r.ServeHTTP(httptest.NewRecorder(), req)
			if got != tc.want {
				t.Errorf("ClientIP = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSecurityHeaders(t *testing.T) {
	cfg := testRouterConfig(nil)
	cfg.HTTP.HSTS = true
	r := api.NewRouter(cfg)
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	for header, want := range map[string]string{
		"X-Content-Type-Options":    "nosniff",
		"X-Frame-Options":           "DENY",
		"Cache-Control":             "no-store",
		"Referrer-Policy":           "no-referrer",
		"Strict-Transport-Security": "max-age=31536000; includeSubDomains",
	} {
		if got := rec.Header().Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}

	cfg.HTTP.HSTS = false
	r = api.NewRouter(cfg)
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {})
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Header().Get("Strict-Transport-Security") != "" {
		t.Error("HSTS must be off by default")
	}
}

func TestCORS(t *testing.T) {
	r := api.NewRouter(testRouterConfig(nil))
	r.Post("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusCreated) })

	t.Run("preflight from allowed origin", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/", nil)
		req.Header.Set("Origin", "http://app.test")
		req.Header.Set("Access-Control-Request-Method", "POST")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d", rec.Code)
		}
		if rec.Header().Get("Access-Control-Allow-Origin") != "http://app.test" {
			t.Errorf("Allow-Origin = %q", rec.Header().Get("Access-Control-Allow-Origin"))
		}
		if rec.Header().Get("Access-Control-Allow-Credentials") != "true" {
			t.Error("credentials should be allowed")
		}
		if !strings.Contains(rec.Header().Get("Access-Control-Allow-Methods"), "POST") {
			t.Error("methods missing")
		}
	})

	t.Run("preflight from other origin is refused", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/", nil)
		req.Header.Set("Origin", "http://evil.test")
		req.Header.Set("Access-Control-Request-Method", "POST")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden || rec.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Fatalf("status = %d, allow-origin = %q", rec.Code, rec.Header().Get("Access-Control-Allow-Origin"))
		}
	})

	t.Run("actual request echoes exact origin, never *", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		req.Header.Set("Origin", "http://app.test")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("status = %d", rec.Code)
		}
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://app.test" {
			t.Errorf("Allow-Origin = %q", got)
		}
		if !strings.Contains(rec.Header().Get("Vary"), "Origin") {
			t.Error("Vary: Origin missing")
		}
	})

	t.Run("no origin header means no CORS headers", func(t *testing.T) {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", nil))
		if rec.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Error("unexpected CORS header")
		}
	})
}

func TestTrustedOrigin_RejectsUnsafeRequestsFromUntrustedOrigins(t *testing.T) {
	r := api.NewRouter(testRouterConfig(nil))
	r.Post("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusCreated) })
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {})

	tests := []struct {
		name    string
		method  string
		headers map[string]string
		want    int
	}{
		{"no origin (curl)", http.MethodPost, nil, http.StatusCreated},
		{"allowed origin", http.MethodPost, map[string]string{"Origin": "http://app.test"}, http.StatusCreated},
		{"allowed origin case-insensitive", http.MethodPost, map[string]string{"Origin": "HTTP://APP.TEST"}, http.StatusCreated},
		{"untrusted origin", http.MethodPost, map[string]string{"Origin": "http://evil.test"}, http.StatusForbidden},
		{"null origin", http.MethodPost, map[string]string{"Origin": "null"}, http.StatusForbidden},
		{"untrusted referer without origin", http.MethodPost, map[string]string{"Referer": "http://evil.test/page"}, http.StatusForbidden},
		{"allowed referer without origin", http.MethodPost, map[string]string{"Referer": "http://app.test/page"}, http.StatusCreated},
		{"safe method ignores origin", http.MethodGet, map[string]string{"Origin": "http://evil.test"}, http.StatusOK},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "/", nil)
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d (body %s)", rec.Code, tc.want, rec.Body.String())
			}
			if tc.want == http.StatusForbidden && decodeError(t, rec).Error.Code != api.CodeOriginNotAllowed {
				t.Errorf("code = %q", decodeError(t, rec).Error.Code)
			}
		})
	}
}

func TestRateLimit_Returns429WithRetryAfter(t *testing.T) {
	r := api.NewRouter(testRouterConfig(nil))
	r.With(api.RateLimit(ratelimit.New(2, time.Minute))).Get("/limited", func(w http.ResponseWriter, r *http.Request) {})

	do := func(ip string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/limited", nil)
		req.RemoteAddr = ip + ":1"
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec
	}
	for i := 0; i < 2; i++ {
		if rec := do("203.0.113.1"); rec.Code != http.StatusOK {
			t.Fatalf("request %d: status %d", i+1, rec.Code)
		}
	}
	rec := do("203.0.113.1")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("Retry-After missing")
	}
	if decodeError(t, rec).Error.Code != apperror.CodeRateLimited {
		t.Error("code should be rate_limited")
	}
	if rec := do("203.0.113.2"); rec.Code != http.StatusOK {
		t.Error("other clients are independent")
	}
}

func TestRouter_UnknownRouteAndMethodUseJSONErrors(t *testing.T) {
	r := api.NewRouter(testRouterConfig(nil))
	r.Get("/exists", func(w http.ResponseWriter, r *http.Request) {})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/missing", nil))
	if rec.Code != http.StatusNotFound || decodeError(t, rec).Error.Code != apperror.CodeNotFound {
		t.Errorf("missing route: status %d body %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/exists", nil))
	if rec.Code != http.StatusMethodNotAllowed || decodeError(t, rec).Error.Code != "method_not_allowed" {
		t.Errorf("wrong method: status %d body %s", rec.Code, rec.Body.String())
	}
}

func TestWriteError_NeverLeaksInternalCause(t *testing.T) {
	var buf bytes.Buffer
	r := api.NewRouter(testRouterConfig(&buf))
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		api.WriteError(r.Context(), w, errors.New("pq: duplicate key value violates unique constraint"))
	})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "duplicate key") {
		t.Error("internal error leaked")
	}
	if !strings.Contains(buf.String(), "duplicate key") {
		t.Error("internal error should be logged")
	}
}
