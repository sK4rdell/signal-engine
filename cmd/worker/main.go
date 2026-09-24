// Command worker executes background jobs from the PostgreSQL queue.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/sK4rdell/signal-engine/internal/app"
	"github.com/sK4rdell/signal-engine/internal/platform/config"
	"github.com/sK4rdell/signal-engine/internal/platform/database"
	"github.com/sK4rdell/signal-engine/internal/platform/email"
	"github.com/sK4rdell/signal-engine/internal/platform/logging"
	"github.com/sK4rdell/signal-engine/internal/platform/metrics"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "worker:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	logger := logging.New(os.Stderr, cfg.Log.Level, cfg.Log.Format)
	logger.Info("starting worker", "env", cfg.Environment)

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

	if err := application.Worker.Run(ctx); err != nil {
		return err
	}
	logger.Info("worker stopped")
	return nil
}
