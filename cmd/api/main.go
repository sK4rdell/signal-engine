// Command api serves the HTTP API. With JOBS_WORKER_IN_API=true it also runs
// the background worker in-process, which is convenient locally.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
	"sync"
	"syscall"

	"betemplate/internal/app"
	"betemplate/internal/platform/config"
	"betemplate/internal/platform/database"
	"betemplate/internal/platform/email"
	"betemplate/internal/platform/logging"
	"betemplate/internal/platform/metrics"
	"betemplate/internal/platform/server"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "api:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	logger := logging.New(os.Stderr, cfg.Log.Level, cfg.Log.Format)
	logger.Info("starting api", "env", cfg.Environment, "revision", revision())

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := database.NewPool(ctx, cfg.Database)
	if err != nil {
		return err
	}
	defer pool.Close()

	sender, err := email.NewSender(cfg.Email, logger)
	if err != nil {
		return err
	}

	application, err := app.New(app.Deps{
		Config:  cfg,
		Logger:  logger,
		Pool:    pool,
		Metrics: &metrics.Metrics{},
		Email:   sender,
	})
	if err != nil {
		return err
	}

	// Shutdown order: stop accepting HTTP, finish in-flight requests, stop
	// the worker, close the pool (deferred above).
	var wg sync.WaitGroup
	workerCtx, stopWorker := context.WithCancel(context.Background())
	defer stopWorker()
	if cfg.Jobs.WorkerInAPI {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := application.Worker.Run(workerCtx); err != nil {
				logger.Error("worker stopped with error", "error", err)
			}
		}()
	}

	srv := server.New(cfg.HTTP, application.Router)
	serveErr := server.Run(ctx, srv, cfg.HTTP.ShutdownTimeout, logger)

	stopWorker()
	wg.Wait()
	if serveErr != nil {
		return serveErr
	}
	logger.Info("api stopped")
	return nil
}

// revision reports the VCS revision embedded by the Go toolchain, if any.
func revision() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	for _, s := range info.Settings {
		if s.Key == "vcs.revision" {
			return s.Value
		}
	}
	return "unknown"
}
