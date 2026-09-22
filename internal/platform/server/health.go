package server

import (
	"context"
	"net/http"
	"time"

	"betemplate/internal/platform/logging"
)

// Pinger reports whether a dependency is reachable. *pgxpool.Pool satisfies it.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Healthz reports that the process is alive. It never touches dependencies.
func Healthz() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeStatus(w, http.StatusOK, `{"status":"ok"}`)
	})
}

// Readyz reports whether the process can serve traffic: PostgreSQL must
// answer a ping within a short deadline.
func Readyz(db Pinger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := db.Ping(ctx); err != nil {
			logging.FromContext(r.Context()).Warn("readiness check failed", "error", err)
			writeStatus(w, http.StatusServiceUnavailable, `{"status":"unavailable","checks":{"database":"failed"}}`)
			return
		}
		writeStatus(w, http.StatusOK, `{"status":"ok","checks":{"database":"ok"}}`)
	})
}

func writeStatus(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body + "\n"))
}
