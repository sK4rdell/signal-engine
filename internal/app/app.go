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

	"github.com/sK4rdell/signal-engine/internal/account"
	"github.com/sK4rdell/signal-engine/internal/auth"
	"github.com/sK4rdell/signal-engine/internal/example"
	"github.com/sK4rdell/signal-engine/internal/platform/api"
	"github.com/sK4rdell/signal-engine/internal/platform/config"
	"github.com/sK4rdell/signal-engine/internal/platform/email"
	"github.com/sK4rdell/signal-engine/internal/platform/jobs"
	"github.com/sK4rdell/signal-engine/internal/platform/metrics"
	"github.com/sK4rdell/signal-engine/internal/platform/secretbox"
	"github.com/sK4rdell/signal-engine/internal/platform/server"
	"github.com/sK4rdell/signal-engine/internal/publicevent"
	"github.com/sK4rdell/signal-engine/internal/source/arbetsmiljoverket"
	"github.com/sK4rdell/signal-engine/internal/source/stockholm"
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

	// Arbetsmiljoverket ingests the diary feeds (inspection notices,
	// recurring-inspection failures); run by cmd/ingest.
	Arbetsmiljoverket *arbetsmiljoverket.Ingester
	// Stockholm ingests failed inspections of lifts and other motorised
	// building equipment from Stockholms stad; run by cmd/ingest.
	Stockholm *stockholm.Ingester
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

	// Public events and the sources that produce them.
	publicEventRepo := publicevent.NewRepository()
	publicevent.RegisterRoutes(router, publicEventRepo, deps.Pool, authenticated)
	avClient, err := arbetsmiljoverket.NewClient(deps.Config.Sources.Arbetsmiljoverket, nil)
	if err != nil {
		return nil, err
	}
	stockholmClient, err := stockholm.NewClient(deps.Config.Sources.Stockholm, nil)
	if err != nil {
		return nil, err
	}

	return &App{
		Router:            router,
		Worker:            worker,
		Auth:              authService,
		Accounts:          accountRepo,
		Arbetsmiljoverket: arbetsmiljoverket.NewIngester(avClient, publicEventRepo, deps.Pool, deps.Logger),
		Stockholm:         stockholm.NewIngester(stockholmClient, publicEventRepo, deps.Pool, deps.Logger, deps.Config.Sources.Stockholm.RefreshInterval),
	}, nil
}
