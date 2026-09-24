package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/sK4rdell/signal-engine/internal/platform/apperror"
	"github.com/sK4rdell/signal-engine/internal/platform/logging"
	"github.com/sK4rdell/signal-engine/internal/platform/metrics"
	"github.com/sK4rdell/signal-engine/internal/platform/ratelimit"
)

// Middleware is a standard net/http middleware.
type Middleware func(http.Handler) http.Handler

var requestIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

// RequestID assigns every request an ID, echoes it in X-Request-ID and opens
// the request logging scope rooted at logger. A client-supplied X-Request-ID
// is honoured when it is short and alphanumeric; anything else is replaced.
func RequestID(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get("X-Request-ID")
			if !requestIDPattern.MatchString(id) {
				id = newRequestID()
			}
			w.Header().Set("X-Request-ID", id)

			ctx := context.WithValue(r.Context(), requestIDKey, id)
			ctx = logging.NewScope(ctx, logger)
			logging.Add(ctx, "request_id", id, "method", r.Method, "path", r.URL.Path)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func newRequestID() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("api: crypto/rand unavailable: %v", err))
	}
	return hex.EncodeToString(b)
}

// RealIP resolves the client IP. The peer address is used unless it belongs
// to a trusted proxy, in which case X-Forwarded-For is walked from the right
// and the first address that is not a trusted proxy wins. Untrusted peers
// cannot spoof their address by sending the header.
func RealIP(trustedProxies []*net.IPNet) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := resolveClientIP(r, trustedProxies)
			ctx := context.WithValue(r.Context(), clientIPKey, ip)
			logging.Add(ctx, "client_ip", ip)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func resolveClientIP(r *http.Request, trusted []*net.IPNet) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	peer := net.ParseIP(strings.TrimSpace(host))
	if peer == nil {
		return ""
	}
	if !isTrusted(peer, trusted) {
		return peer.String()
	}
	forwarded := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	for i := len(forwarded) - 1; i >= 0; i-- {
		candidate := net.ParseIP(strings.TrimSpace(forwarded[i]))
		if candidate == nil {
			break
		}
		if !isTrusted(candidate, trusted) {
			return candidate.String()
		}
	}
	return peer.String()
}

func isTrusted(ip net.IP, trusted []*net.IPNet) bool {
	for _, n := range trusted {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// Recover turns a panic into a generic 500 and logs it with a stack trace.
// The connection is left in a consistent state; the process keeps serving.
func Recover() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				rec := recover()
				if rec == nil {
					return
				}
				if err, ok := rec.(error); ok && errors.Is(err, http.ErrAbortHandler) {
					panic(rec)
				}
				// The one diagnostic log for this panic; the response is
				// written without logging again.
				logging.FromContext(r.Context()).Error("panic recovered",
					"panic", fmt.Sprint(rec),
					"stack", string(debug.Stack()),
				)
				writeErrorResponse(w, apperror.Internal(fmt.Errorf("panic: %v", rec)))
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// RequestLogger writes one access-log line per request and records the
// request in metrics. It runs outside Recover so panics are logged with
// their 500 status. The line is always informational: the diagnostic for a
// failure is logged once where the error is handled (WriteError, Recover).
func RequestLogger(m *metrics.Metrics) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rw := &statusWriter{ResponseWriter: w}
			next.ServeHTTP(rw, r)
			duration := time.Since(start)
			status := rw.status
			if status == 0 {
				status = http.StatusOK
			}
			m.ObserveRequest(status, duration)

			logger := logging.FromContext(r.Context())
			attrs := []any{
				"status", status,
				"duration_ms", float64(duration.Microseconds()) / 1000,
				"bytes", rw.bytes,
			}
			logger.Info("request", attrs...)
		})
	}
}

// statusWriter records the status code and bytes written.
type statusWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *statusWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(b)
	w.bytes += n
	return n, err
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// SecurityHeaders sets the headers a JSON API should always carry.
// HSTS is only emitted when the deployment is known to be HTTPS-only.
func SecurityHeaders(hsts bool) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("X-Frame-Options", "DENY")
			h.Set("Referrer-Policy", "no-referrer")
			h.Set("Cache-Control", "no-store")
			h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
			if hsts {
				h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			}
			next.ServeHTTP(w, r)
		})
	}
}

// CORS answers preflight requests and adds the CORS headers for allowed
// origins. Credentials are allowed, so the origin is always echoed exactly
// and never "*". Requests from other origins get no CORS headers, which
// makes the browser block the response.
func CORS(allowedOrigins []string) Middleware {
	allowed := originSet(allowedOrigins)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin == "" {
				next.ServeHTTP(w, r)
				return
			}
			w.Header().Add("Vary", "Origin")
			if !allowed[strings.ToLower(origin)] {
				if isPreflight(r) {
					WriteError(r.Context(), w, apperror.Forbidden(CodeOriginNotAllowed, "Origin not allowed"))
					return
				}
				next.ServeHTTP(w, r)
				return
			}
			h := w.Header()
			h.Set("Access-Control-Allow-Origin", origin)
			h.Set("Access-Control-Allow-Credentials", "true")
			if isPreflight(r) {
				h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				h.Set("Access-Control-Allow-Headers", "Content-Type, X-Request-ID")
				h.Set("Access-Control-Max-Age", "600")
				w.WriteHeader(http.StatusNoContent)
				return
			}
			h.Set("Access-Control-Expose-Headers", "X-Request-ID")
			next.ServeHTTP(w, r)
		})
	}
}

func isPreflight(r *http.Request) bool {
	return r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != ""
}

// CodeOriginNotAllowed is returned when a browser request carries an
// untrusted Origin.
const CodeOriginNotAllowed = "origin_not_allowed"

// TrustedOrigin is the CSRF guard for cookie-authenticated requests.
//
// Threat model: a malicious site makes the victim's browser send a
// state-changing request that carries the session cookie. Browsers attach
// the Origin header to such cross-site requests (and to every fetch/XHR
// POST), so an unsafe request whose Origin is not an allowed frontend origin
// is refused. Requests without Origin fall back to Referer when present;
// requests with neither (curl, server-to-server) pass, since they cannot be
// cross-site browser requests. SameSite=Lax on the session cookie is the
// second, independent layer.
func TrustedOrigin(allowedOrigins []string) Middleware {
	allowed := originSet(allowedOrigins)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isSafeMethod(r.Method) {
				next.ServeHTTP(w, r)
				return
			}
			origin := r.Header.Get("Origin")
			if origin == "" {
				if referer := r.Header.Get("Referer"); referer != "" {
					origin = originOf(referer)
				}
			}
			if origin != "" && !allowed[strings.ToLower(origin)] {
				WriteError(r.Context(), w, apperror.Forbidden(CodeOriginNotAllowed, "Origin not allowed"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func isSafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	}
	return false
}

func originOf(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "invalid"
	}
	return u.Scheme + "://" + u.Host
}

func originSet(origins []string) map[string]bool {
	set := make(map[string]bool, len(origins))
	for _, o := range origins {
		set[strings.ToLower(strings.TrimSuffix(o, "/"))] = true
	}
	return set
}

// RateLimit refuses requests beyond the limiter's allowance per client IP
// with 429 and a Retry-After header. Apply it per route group.
func RateLimit(limiter *ratelimit.Limiter) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := ClientIP(r.Context())
			allowed, retryAfter := limiter.Allow(key)
			if !allowed {
				seconds := int(retryAfter.Seconds())
				if seconds < 1 {
					seconds = 1
				}
				w.Header().Set("Retry-After", strconv.Itoa(seconds))
				WriteError(r.Context(), w, apperror.RateLimited())
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// BodyLimit records the router-wide default body limit that Handle applies.
// Raw handlers must apply LimitBody themselves.
func BodyLimit(maxBytes int64) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), bodyLimitKey, maxBytes)))
		})
	}
}

// LimitBody bounds r.Body to maxBytes. Handlers that bypass Handle (file
// uploads, webhooks) should call it before reading the body.
func LimitBody(w http.ResponseWriter, r *http.Request, maxBytes int64) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
}
