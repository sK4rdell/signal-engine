package api

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/sK4rdell/signal-engine/internal/platform/apperror"
	"github.com/sK4rdell/signal-engine/internal/platform/config"
	"github.com/sK4rdell/signal-engine/internal/platform/metrics"
)

// RouterConfig configures the shared middleware stack.
type RouterConfig struct {
	Logger  *slog.Logger
	HTTP    config.HTTPConfig
	Metrics *metrics.Metrics
}

// NewRouter builds the chi router with the standard middleware stack, in
// this order (outermost first):
//
//  1. RequestID        assigns the ID and opens the logging scope
//  2. RealIP           resolves the client IP (trusted proxies only)
//  3. RequestLogger    logs every request, records metrics
//  4. Recover          converts panics into 500 responses
//  5. SecurityHeaders  static response headers
//  6. CORS             preflight and allowed-origin headers
//  7. TrustedOrigin    CSRF guard for unsafe methods
//  8. BodyLimit        default request body limit for Handle
//
// Authentication and rate limiting are applied per route group by features.
// Unknown routes and methods answer with the standard JSON error format.
func NewRouter(cfg RouterConfig) chi.Router {
	r := chi.NewRouter()
	r.Use(
		RequestID(cfg.Logger),
		RealIP(cfg.HTTP.TrustedProxies),
		RequestLogger(cfg.Metrics),
		Recover(),
		SecurityHeaders(cfg.HTTP.HSTS),
		CORS(cfg.HTTP.AllowedOrigins),
		TrustedOrigin(cfg.HTTP.AllowedOrigins),
		BodyLimit(cfg.HTTP.MaxBodyBytes),
	)
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		WriteError(r.Context(), w, apperror.NotFound(apperror.CodeNotFound, "Route not found"))
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		WriteError(r.Context(), w, apperror.New(http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed"))
	})
	return r
}
