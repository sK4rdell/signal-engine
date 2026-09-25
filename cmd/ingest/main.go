// Command ingest fetches public records from a source for a bounded date
// window and stores them as source observations and public events.
//
//	ingest arbetsmiljoverket [--feed inspection-notices] --from 2026-09-20 --to 2026-09-23
//	ingest arbetsmiljoverket --feed recurring-inspection-failures --from 2026-09-20 --to 2026-09-23
//	ingest stockholm --from 2026-09-20 --to 2026-09-23
//
// Both dates are inclusive civil dates. --to defaults to yesterday and
// --from defaults to --to. For Arbetsmiljöverket the window is the
// document date and --feed defaults to inspection notices; for Stockholm
// the window is the case start date, and known open cases due for a
// refresh are re-read on every run. Runs are idempotent: repeating a
// window never duplicates events.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/sK4rdell/signal-engine/internal/app"
	"github.com/sK4rdell/signal-engine/internal/platform/config"
	"github.com/sK4rdell/signal-engine/internal/platform/database"
	"github.com/sK4rdell/signal-engine/internal/platform/email"
	"github.com/sK4rdell/signal-engine/internal/platform/logging"
	"github.com/sK4rdell/signal-engine/internal/platform/metrics"
	"github.com/sK4rdell/signal-engine/internal/source/arbetsmiljoverket"
	"github.com/sK4rdell/signal-engine/internal/source/stockholm"
)

const dateLayout = "2006-01-02"

// Sources the command knows.
const (
	sourceArbetsmiljoverket = "arbetsmiljoverket"
	sourceStockholm         = "stockholm"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "ingest:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	inv, err := parseArgs(args, time.Now())
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

	from, to := inv.from.Format(dateLayout), inv.to.Format(dateLayout)
	switch inv.source {
	case sourceArbetsmiljoverket:
		logger.Info("ingestion started", "source", arbetsmiljoverket.Source, "feed", inv.feed.Name, "from", from, "to", to)
		stats, runErr := application.Arbetsmiljoverket.RunFeed(ctx, inv.feed, arbetsmiljoverket.Window{From: inv.from, To: inv.to})
		if runErr != nil {
			logger.Error("ingestion failed", append([]any{"error", runErr}, stats.LogAttrs()...)...)
		} else {
			logger.Info("ingestion finished", stats.LogAttrs()...)
		}
		fmt.Printf("source:                        %s\n", arbetsmiljoverket.Source)
		fmt.Printf("feed:                          %s (%s)\n", inv.feed.Name, inv.feed.EventType)
		fmt.Printf("window:                        %s .. %s\n", from, to)
		fmt.Printf("pages processed:               %d\n", stats.Pages)
		fmt.Printf("records observed:              %d\n", stats.RecordsObserved)
		fmt.Printf("records accepted:              %d\n", stats.RecordsAccepted)
		fmt.Printf("records skipped (no match):    %d\n", stats.RecordsSkipped)
		fmt.Printf("records later in case:         %d\n", stats.RecordsLaterInCase)
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
	case sourceStockholm:
		logger.Info("ingestion started", "source", stockholm.Source, "from", from, "to", to)
		stats, runErr := application.Stockholm.Run(ctx, stockholm.Window{From: inv.from, To: inv.to})
		if runErr != nil {
			logger.Error("ingestion failed", append([]any{"error", runErr}, stats.LogAttrs()...)...)
		} else {
			logger.Info("ingestion finished", stats.LogAttrs()...)
		}
		fmt.Printf("source:                        %s\n", stockholm.Source)
		fmt.Printf("window (case start):           %s .. %s\n", from, to)
		fmt.Printf("search cases discovered:       %d\n", stats.SearchCasesDiscovered)
		fmt.Printf("new cases fetched:             %d\n", stats.NewCasesFetched)
		fmt.Printf("open cases refreshed:          %d\n", stats.OpenCasesRefreshed)
		fmt.Printf("case pages fetched:            %d\n", stats.CasePagesFetched)
		fmt.Printf("source records observed:       %d\n", stats.RecordsObserved)
		fmt.Printf("observations inserted:         %d\n", stats.ObservationsInserted)
		fmt.Printf("failed certificates found:     %d\n", stats.FailedCertificates)
		fmt.Printf("documents excluded:            %d\n", stats.DocumentsExcluded)
		fmt.Printf("duplicate certificates:        %d\n", stats.DuplicateCertificates)
		fmt.Printf("events inserted:               %d\n", stats.EventsInserted)
		fmt.Printf("events updated:                %d\n", stats.EventsUpdated)
		fmt.Printf("events already known:          %d\n", stats.EventsUnchanged)
		fmt.Printf("persist failures:              %d\n", stats.PersistFailures)
		return runErr
	}
	return errors.New(usage())
}

// invocation is a parsed command line.
type invocation struct {
	source string
	// feed is set for Arbetsmiljöverket only.
	feed     arbetsmiljoverket.Feed
	from, to time.Time
}

// usage is the one-line synopsis shown on argument errors.
func usage() string {
	names := make([]string, 0, 2)
	for _, f := range arbetsmiljoverket.Feeds() {
		names = append(names, f.Name)
	}
	return fmt.Sprintf("usage: ingest arbetsmiljoverket [--feed %s] [--from YYYY-MM-DD] [--to YYYY-MM-DD] | ingest stockholm [--from YYYY-MM-DD] [--to YYYY-MM-DD]", strings.Join(names, "|"))
}

// parseArgs resolves the command line. The Arbetsmiljöverket feed defaults
// to inspection notices so existing invocations keep their behaviour.
func parseArgs(args []string, now time.Time) (invocation, error) {
	if len(args) < 1 {
		return invocation{}, errors.New(usage())
	}
	inv := invocation{source: args[0]}
	fs := flag.NewFlagSet(inv.source, flag.ContinueOnError)
	var feedFlag *string
	switch inv.source {
	case sourceArbetsmiljoverket:
		feedFlag = fs.String("feed", arbetsmiljoverket.DefaultFeed.Name, "feed to ingest: inspection-notices or recurring-inspection-failures")
	case sourceStockholm:
	default:
		return invocation{}, errors.New(usage())
	}
	fromFlag := fs.String("from", "", "first date, inclusive (default: --to)")
	toFlag := fs.String("to", "", "last date, inclusive (default: yesterday)")
	if err := fs.Parse(args[1:]); err != nil {
		return invocation{}, err
	}
	if feedFlag != nil {
		feed, err := arbetsmiljoverket.FeedByName(*feedFlag)
		if err != nil {
			return invocation{}, fmt.Errorf("--feed: %w", err)
		}
		inv.feed = feed
	}
	from, to, err := parseWindow(*fromFlag, *toFlag, now)
	if err != nil {
		return invocation{}, err
	}
	inv.from, inv.to = from, to
	return inv, nil
}

// parseWindow resolves the flags to an inclusive window. Dates are civil
// dates in UTC; the sources are date-based.
func parseWindow(from, to string, now time.Time) (time.Time, time.Time, error) {
	var f, t time.Time
	if to == "" {
		y, m, d := now.UTC().AddDate(0, 0, -1).Date()
		t = time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	} else {
		parsed, err := time.Parse(dateLayout, to)
		if err != nil {
			return f, t, fmt.Errorf("--to: %q is not a date formatted like 2026-09-23", to)
		}
		t = parsed
	}
	if from == "" {
		f = t
	} else {
		parsed, err := time.Parse(dateLayout, from)
		if err != nil {
			return f, t, fmt.Errorf("--from: %q is not a date formatted like 2026-09-23", from)
		}
		f = parsed
	}
	if t.Before(f) {
		return f, t, errors.New("the window end is before its start")
	}
	return f, t, nil
}
