// Package app is the composition root shared by cmd/api, cmd/worker and the
// integration-test harness. It wires configuration, infrastructure,
// repositories, services, routes and job handlers explicitly, so tests run
// the same stack as production.
package app

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"betemplate/internal/account"
	"betemplate/internal/auth"
	"betemplate/internal/example"
	"betemplate/internal/platform/api"
	"betemplate/internal/platform/config"
	"betemplate/internal/platform/email"
	"betemplate/internal/platform/jobs"
	"betemplate/internal/platform/metrics"
	"betemplate/internal/platform/secretbox"
	"betemplate/internal/platform/server"
)

// Deps are the infrastructure values the application is built from.
type Deps struct {
	Config  config.Config
	Logger  *slog.Logger
	Pool    *pgxpool.Pool
	Metrics *metrics.Metrics
	Email   email.Sender
}

// App holds the wired application.
type App struct {
	Router chi.Router
	Worker *jobs.Worker

	Auth     *auth.Service
	Accounts *account.Repository
}

// New wires the application. The router serves the API; the worker has every
// job handler registered and is run by cmd/worker (or inside cmd/api when
// configured).
func New(deps Deps) (*App, error) {
	box, err := secretbox.New(deps.Config.Auth.EncryptionKey)
	if err != nil {
		return nil, err
	}
	if deps.Metrics == nil {
		deps.Metrics = &metrics.Metrics{}
	}
	deps.Metrics.RegisterGauge("db_pool_total_conns", func() int64 { return int64(deps.Pool.Stat().TotalConns()) })
	deps.Metrics.RegisterGauge("db_pool_acquired_conns", func() int64 { return int64(deps.Pool.Stat().AcquiredConns()) })
	deps.Metrics.RegisterGauge("db_pool_idle_conns", func() int64 { return int64(deps.Pool.Stat().IdleConns()) })

	// Repositories and services.
	authRepo := auth.NewRepository()
	accountRepo := account.NewRepository()
	authService := auth.NewService(deps.Pool, authRepo, accountRepo, deps.Config.Auth, box)

	// Background jobs.
	worker := jobs.NewWorker(deps.Pool, deps.Config.Jobs, deps.Logger, deps.Metrics)
	auth.RegisterJobs(worker, deps.Email, deps.Config.Auth, box)

	// HTTP.
	router := api.NewRouter(api.RouterConfig{
		Logger:  deps.Logger,
		HTTP:    deps.Config.HTTP,
		Metrics: deps.Metrics,
	})
	router.Method(http.MethodGet, "/healthz", server.Healthz())
	router.Method(http.MethodGet, "/readyz", server.Readyz(deps.Pool))
	router.Method(http.MethodGet, "/metricsz", deps.Metrics.Handler())

	authenticated := auth.RequireSession(authService)
	member := account.RequireMembership(accountRepo, deps.Pool)
	auth.RegisterRoutes(router, authService, deps.Config.Auth)
	account.RegisterRoutes(router, accountRepo, deps.Pool, authenticated)

	// Example feature: delete this line together with internal/example and
	// its migration when starting a real product.
	example.RegisterRoutes(router, example.NewRepository(), deps.Pool, authenticated, member)

	return &App{
		Router:   router,
		Worker:   worker,
		Auth:     authService,
		Accounts: accountRepo,
	}, nil
}
