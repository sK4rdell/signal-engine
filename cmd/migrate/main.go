// Command migrate applies, rolls back and reports goose SQL migrations.
//
//	migrate up       apply all pending migrations
//	migrate down     roll back the most recent migration
//	migrate status   list migrations and whether they are applied
//	migrate version  print the current schema version
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/sK4rdell/signal-engine/internal/platform/config"
	"github.com/sK4rdell/signal-engine/internal/platform/database"
	"github.com/sK4rdell/signal-engine/internal/platform/logging"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "migrate:", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) != 2 {
		return fmt.Errorf("usage: migrate up|down|status|version")
	}
	command := os.Args[1]

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	logger := logging.New(os.Stderr, cfg.Log.Level, cfg.Log.Format)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := database.NewPool(ctx, cfg.Database)
	if err != nil {
		return err
	}
	defer pool.Close()

	m, err := database.NewMigrator(pool, logger)
	if err != nil {
		return err
	}

	switch command {
	case "up":
		versions, err := m.Up(ctx)
		if err != nil {
			return err
		}
		if len(versions) == 0 {
			fmt.Println("no pending migrations")
		}
		for _, v := range versions {
			fmt.Printf("applied %d\n", v)
		}
	case "down":
		v, err := m.Down(ctx)
		if err != nil {
			return err
		}
		fmt.Printf("rolled back %d\n", v)
	case "status":
		statuses, err := m.Status(ctx)
		if err != nil {
			return err
		}
		for _, s := range statuses {
			state := "pending"
			if s.Applied {
				state = "applied"
			}
			fmt.Printf("%-8s %d %s\n", state, s.Version, s.Path)
		}
	case "version":
		v, err := m.Version(ctx)
		if err != nil {
			return err
		}
		fmt.Println(v)
	default:
		return fmt.Errorf("unknown command %q (want up, down, status or version)", command)
	}
	return nil
}
