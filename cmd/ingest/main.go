// Command ingest fetches public records from a source for a bounded date
// window and stores them as source observations and public events.
//
//	ingest arbetsmiljoverket --from 2026-09-20 --to 2026-09-23
//
// Both dates are inclusive civil dates. --to defaults to yesterday (the
// diary never shows today's documents) and --from defaults to --to. Runs
// are idempotent: repeating a window never duplicates events.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sK4rdell/signal-engine/internal/app"
	"github.com/sK4rdell/signal-engine/internal/platform/config"
	"github.com/sK4rdell/signal-engine/internal/platform/database"
	"github.com/sK4rdell/signal-engine/internal/platform/email"
	"github.com/sK4rdell/signal-engine/internal/platform/logging"
	"github.com/sK4rdell/signal-engine/internal/platform/metrics"
	"github.com/sK4rdell/signal-engine/internal/source/arbetsmiljoverket"
)

const dateLayout = "2006-01-02"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "ingest:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) < 1 || args[0] != "arbetsmiljoverket" {
		return errors.New("usage: ingest arbetsmiljoverket [--from YYYY-MM-DD] [--to YYYY-MM-DD]")
	}
	fs := flag.NewFlagSet("arbetsmiljoverket", flag.ContinueOnError)
	fromFlag := fs.String("from", "", "first document date, inclusive (default: --to)")
	toFlag := fs.String("to", "", "last document date, inclusive (default: yesterday)")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	window, err := parseWindow(*fromFlag, *toFlag, time.Now())
	if err != nil {
		return err
	}

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

	logger.Info("ingestion started", "source", arbetsmiljoverket.Source, "from", window.From.Format(dateLayout), "to", window.To.Format(dateLayout))
	stats, runErr := application.Arbetsmiljoverket.Run(ctx, window)
	if runErr != nil {
		logger.Error("ingestion failed", append([]any{"error", runErr}, stats.LogAttrs()...)...)
	} else {
		logger.Info("ingestion finished", stats.LogAttrs()...)
	}

	fmt.Printf("source:                        %s\n", arbetsmiljoverket.Source)
	fmt.Printf("window:                        %s .. %s\n", window.From.Format(dateLayout), window.To.Format(dateLayout))
	fmt.Printf("pages processed:               %d\n", stats.Pages)
	fmt.Printf("records observed:              %d\n", stats.RecordsObserved)
	fmt.Printf("inspection notices identified: %d\n", stats.InspectionNotices)
	fmt.Printf("other document types skipped:  %d\n", stats.OtherDocumentTypes)
	fmt.Printf("observations inserted:         %d\n", stats.ObservationsInserted)
	fmt.Printf("events inserted:               %d\n", stats.EventsInserted)
	fmt.Printf("events updated:                %d\n", stats.EventsUpdated)
	fmt.Printf("events already known:          %d\n", stats.EventsUnchanged)
	fmt.Printf("parse failures:                %d\n", stats.ParseFailures)
	fmt.Printf("persist failures:              %d\n", stats.PersistFailures)
	fmt.Printf("invalid organisation numbers:  %d\n", stats.InvalidOrganisationNumbers)
	fmt.Printf("invalid workplace CFARs:       %d\n", stats.InvalidWorkplaceCFARs)
	return runErr
}

// parseWindow resolves the flags to an inclusive window. Dates are civil
// dates in UTC; the diary itself is date-based.
func parseWindow(from, to string, now time.Time) (arbetsmiljoverket.Window, error) {
	var w arbetsmiljoverket.Window
	if to == "" {
		y, m, d := now.UTC().AddDate(0, 0, -1).Date()
		w.To = time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	} else {
		t, err := time.Parse(dateLayout, to)
		if err != nil {
			return w, fmt.Errorf("--to: %q is not a date formatted like 2026-09-23", to)
		}
		w.To = t
	}
	if from == "" {
		w.From = w.To
	} else {
		t, err := time.Parse(dateLayout, from)
		if err != nil {
			return w, fmt.Errorf("--from: %q is not a date formatted like 2026-09-23", from)
		}
		w.From = t
	}
	if err := w.Validate(); err != nil {
		return w, err
	}
	return w, nil
}
