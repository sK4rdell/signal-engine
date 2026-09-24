package klimatklivet

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sK4rdell/signal-engine/internal/platform/database"
	"github.com/sK4rdell/signal-engine/internal/publicevent"
)

// Ingester downloads the dataset and persists approved applications as
// source observations and public events.
type Ingester struct {
	client *Client
	repo   *publicevent.Repository
	pool   *pgxpool.Pool
	logger *slog.Logger
}

// NewIngester wires an ingester.
func NewIngester(client *Client, repo *publicevent.Repository, pool *pgxpool.Pool, logger *slog.Logger) *Ingester {
	return &Ingester{client: client, repo: repo, pool: pool, logger: logger}
}

// Options bound one run. From and To are inclusive civil dates on the
// decision date; zero means unbounded. Force downloads and processes the
// file even when the server reports it unchanged.
type Options struct {
	From  time.Time
	To    time.Time
	Force bool
}

// Validate checks the window is well formed.
func (o Options) Validate() error {
	if !o.From.IsZero() && !o.To.IsZero() && o.To.Before(o.From) {
		return errors.New("klimatklivet: window end is before its start")
	}
	return nil
}

// Stats summarises one run.
type Stats struct {
	DatasetURL              string
	DatasetPublishedThrough string
	// DatasetNotModified: the server answered 304, nothing was processed.
	DatasetNotModified bool
	// DatasetChanged: a file version not seen before was recorded.
	DatasetChanged       bool
	RowsInFile           int
	RowsMalformed        int
	RowsInWindow         int
	RowsOutsideWindow    int
	ObservationsInserted int
	EventsInserted       int
	EventsUpdated        int
	EventsUnchanged      int
	PersistFailures      int
}

// LogAttrs returns the stats as structured logging attributes.
func (s Stats) LogAttrs() []any {
	return []any{
		"dataset_url", s.DatasetURL,
		"dataset_published_through", s.DatasetPublishedThrough,
		"dataset_not_modified", s.DatasetNotModified,
		"dataset_changed", s.DatasetChanged,
		"rows_in_file", s.RowsInFile,
		"rows_malformed", s.RowsMalformed,
		"rows_in_window", s.RowsInWindow,
		"rows_outside_window", s.RowsOutsideWindow,
		"observations_inserted", s.ObservationsInserted,
		"events_inserted", s.EventsInserted,
		"events_updated", s.EventsUpdated,
		"events_unchanged", s.EventsUnchanged,
		"persist_failures", s.PersistFailures,
	}
}

// ErrPersistFailures is returned (wrapped) when at least one row could not
// be persisted; the rest of the run completed and the stats are valid.
var ErrPersistFailures = errors.New("klimatklivet: some rows could not be persisted")

// Run locates the current dataset file, records it as an observation and
// persists every approved application decided within the window. Unless
// forced, the download is conditional on the ETag of the last recorded
// file version, so an unchanged file is neither transferred nor processed.
// Re-running is idempotent.
func (in *Ingester) Run(ctx context.Context, opts Options) (Stats, error) {
	var stats Stats
	if err := opts.Validate(); err != nil {
		return stats, err
	}
	logger := in.logger.With("source", Source)

	link, err := in.client.DiscoverDataset(ctx)
	if err != nil {
		return stats, err
	}
	stats.DatasetURL = link.URL
	stats.DatasetPublishedThrough = link.PublishedThrough

	previous, err := in.latestDataset(ctx)
	if err != nil {
		return stats, err
	}
	etag := ""
	if previous != nil && !opts.Force && previous.URL == link.URL {
		etag = previous.ETag
	}

	dl, err := in.client.Download(ctx, link, etag)
	if err != nil {
		return stats, err
	}
	if dl.NotModified {
		// Record that the same version was checked again.
		stats.DatasetNotModified = true
		if _, _, err := in.repo.RecordObservation(ctx, in.pool, publicevent.NewObservation{
			Source: Source, SourceRecordID: DatasetRecordID, SourceURL: link.URL, Payload: previous, Raw: link.Label,
		}); err != nil {
			return stats, err
		}
		return stats, nil
	}

	wb, err := ParseWorkbook(bytes.NewReader(dl.Body))
	if err != nil {
		return stats, err
	}
	stats.RowsInFile = len(wb.Applications) + len(wb.RowErrors)
	stats.RowsMalformed = len(wb.RowErrors)
	for _, re := range wb.RowErrors {
		logger.Warn("dataset row could not be parsed", "row", re.Row, "error", re.Err)
	}

	dataset := Dataset{
		URL:              dl.URL,
		Label:            link.Label,
		PublishedThrough: link.PublishedThrough,
		ETag:             dl.ETag,
		LastModified:     dl.LastModified,
		SHA256:           dl.SHA256,
		SizeBytes:        dl.Size,
		Sheet:            wb.Sheet,
		RowCount:         stats.RowsInFile,
	}
	_, inserted, err := in.repo.RecordObservation(ctx, in.pool, publicevent.NewObservation{
		Source: Source, SourceRecordID: DatasetRecordID, SourceURL: dl.URL, Payload: dataset, Raw: link.Label,
	})
	if err != nil {
		return stats, err
	}
	stats.DatasetChanged = inserted
	if inserted {
		stats.ObservationsInserted++
	}

	for _, app := range wb.Applications {
		decided, _ := time.Parse(dateLayout, app.DecisionDate) // validated by the parser
		if (!opts.From.IsZero() && decided.Before(opts.From)) || (!opts.To.IsZero() && decided.After(opts.To)) {
			stats.RowsOutsideWindow++
			continue
		}
		stats.RowsInWindow++
		if err := in.persist(ctx, app, dl.URL, &stats); err != nil {
			stats.PersistFailures++
			logger.Error("row could not be persisted", "case_number", app.CaseNumber, "error", err)
		}
	}

	if stats.PersistFailures > 0 {
		return stats, fmt.Errorf("%w: %d of %d", ErrPersistFailures, stats.PersistFailures, stats.RowsInWindow)
	}
	return stats, nil
}

// latestDataset returns the most recently observed file version, or nil.
func (in *Ingester) latestDataset(ctx context.Context) (*Dataset, error) {
	observations, err := in.repo.ListObservations(ctx, in.pool, Source, DatasetRecordID)
	if err != nil {
		return nil, err
	}
	if len(observations) == 0 {
		return nil, nil
	}
	latest := observations[0]
	for _, o := range observations[1:] {
		if o.LastObservedAt.After(latest.LastObservedAt) {
			latest = o
		}
	}
	var d Dataset
	if err := json.Unmarshal(latest.Payload, &d); err != nil {
		return nil, fmt.Errorf("klimatklivet: decode dataset observation %s: %w", latest.ID, err)
	}
	return &d, nil
}

// persist records the row observation and derives the event in one
// transaction.
func (in *Ingester) persist(ctx context.Context, app Application, fileURL string, stats *Stats) error {
	raw, err := json.Marshal(app.Raw)
	if err != nil {
		return fmt.Errorf("klimatklivet: encode raw row: %w", err)
	}
	ev := ToEvent(app, in.client.ResultsPageURL())
	return database.InTx(ctx, in.pool, func(tx pgx.Tx) error {
		obs, inserted, err := in.repo.RecordObservation(ctx, tx, publicevent.NewObservation{
			Source:         Source,
			SourceRecordID: app.CaseNumber,
			SourceURL:      fileURL,
			Payload:        app,
			Raw:            string(raw),
		})
		if err != nil {
			return err
		}
		if inserted {
			stats.ObservationsInserted++
		}
		ev.ObservationID = obs.ID
		_, outcome, err := in.repo.UpsertEvent(ctx, tx, ev)
		if err != nil {
			return err
		}
		switch outcome {
		case publicevent.OutcomeInserted:
			stats.EventsInserted++
		case publicevent.OutcomeUpdated:
			stats.EventsUpdated++
		case publicevent.OutcomeUnchanged:
			stats.EventsUnchanged++
		}
		return nil
	})
}

// ToEvent maps an approved application to the canonical event. The source
// publishes no organisation number, so organisation identity is the name
// only; it is never guessed. Grant amount, category, location and status
// stay in the observation payload.
func ToEvent(app Application, sourceURL string) publicevent.NewEvent {
	decided, _ := time.Parse(dateLayout, app.DecisionDate) // validated by the parser
	return publicevent.NewEvent{
		Source:           Source,
		SourceEventID:    app.CaseNumber,
		EventType:        publicevent.EventTypeClimateInvestmentGrantApproved,
		OccurredOn:       decided,
		Title:            app.Title,
		OrganisationName: app.OrganisationName,
		SourceURL:        sourceURL,
	}
}
