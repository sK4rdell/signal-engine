package stockholm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sK4rdell/signal-engine/internal/platform/database"
	"github.com/sK4rdell/signal-engine/internal/publicevent"
)

// Defaults of the ingester's bounds.
const (
	// DefaultRefreshInterval is how long an unchanged open case is left
	// alone before its page is read again.
	DefaultRefreshInterval = 7 * 24 * time.Hour
	// DefaultMaxRefreshPerRun bounds the open-case refresh of one run; the
	// oldest-checked cases go first, so the rest follow on later runs.
	DefaultMaxRefreshPerRun = 500
)

// Ingester discovers supervision cases for a window of case start dates,
// refreshes known open cases, and persists each case as a source
// observation with one public event per explicitly failed certificate.
type Ingester struct {
	client *Client
	repo   *publicevent.Repository
	pool   *pgxpool.Pool
	logger *slog.Logger
	// RefreshInterval and MaxRefreshPerRun are the open-case refresh policy.
	RefreshInterval  time.Duration
	MaxRefreshPerRun int
	// now is replaceable by tests.
	now func() time.Time
}

// NewIngester wires an ingester. A non-positive refreshInterval means the
// default.
func NewIngester(client *Client, repo *publicevent.Repository, pool *pgxpool.Pool, logger *slog.Logger, refreshInterval time.Duration) *Ingester {
	if refreshInterval <= 0 {
		refreshInterval = DefaultRefreshInterval
	}
	return &Ingester{client: client, repo: repo, pool: pool, logger: logger, RefreshInterval: refreshInterval, MaxRefreshPerRun: DefaultMaxRefreshPerRun, now: time.Now}
}

// Stats summarises one run.
type Stats struct {
	// SearchCasesDiscovered is the number of distinct cases the search
	// returned for the window.
	SearchCasesDiscovered int
	// NewCasesFetched counts case pages read because the search listed
	// them; OpenCasesRefreshed counts pages read because the case was known
	// locally, still open and due for a refresh.
	NewCasesFetched    int
	OpenCasesRefreshed int
	CasePagesFetched   int
	// RecordsObserved is the number of cases persisted (one observation
	// each); ObservationsInserted how many of those were new states.
	RecordsObserved      int
	ObservationsInserted int
	// FailedCertificates counts documents that matched the failure rule;
	// DocumentsExcluded the rest (approved, partly approved, reminders,
	// correspondence). DuplicateCertificates counts failed certificates
	// whose fingerprint equalled another one in the same case.
	FailedCertificates    int
	DocumentsExcluded     int
	DuplicateCertificates int
	EventsInserted        int
	EventsUpdated         int
	EventsUnchanged       int
	PersistFailures       int
}

// LogAttrs returns the stats as structured logging attributes.
func (s Stats) LogAttrs() []any {
	return []any{
		"search_cases_discovered", s.SearchCasesDiscovered,
		"new_cases_fetched", s.NewCasesFetched,
		"open_cases_refreshed", s.OpenCasesRefreshed,
		"case_pages_fetched", s.CasePagesFetched,
		"records_observed", s.RecordsObserved,
		"observations_inserted", s.ObservationsInserted,
		"failed_certificates", s.FailedCertificates,
		"documents_excluded", s.DocumentsExcluded,
		"duplicate_certificates", s.DuplicateCertificates,
		"events_inserted", s.EventsInserted,
		"events_updated", s.EventsUpdated,
		"events_unchanged", s.EventsUnchanged,
		"persist_failures", s.PersistFailures,
	}
}

// ErrPersistFailures is returned (wrapped) when at least one case could not
// be persisted; the rest of the run completed and the stats are valid.
var ErrPersistFailures = errors.New("stockholm: some cases could not be persisted")

// Run discovers the cases started within w, reads their pages, then reads
// the pages of known open cases that are due for a refresh, and persists
// every page read. A transport, status or parse error aborts the run with
// the stats so far: an incomplete run must not look complete. A failed
// persist is counted, logged and skipped. Re-running over the same or an
// overlapping window is idempotent.
func (in *Ingester) Run(ctx context.Context, w Window) (Stats, error) {
	var stats Stats
	if err := w.Validate(); err != nil {
		return stats, err
	}
	logger := in.logger.With("source", Source, "from", w.From.Format(dateLayout), "to", w.To.Format(dateLayout))

	summaries, searchURL, err := in.client.SearchCases(ctx, w)
	if err != nil {
		return stats, fmt.Errorf("search: %w", err)
	}
	fetched := map[string]bool{}
	for _, s := range summaries {
		if fetched[s.RecNo] {
			continue
		}
		fetched[s.RecNo] = true
		stats.SearchCasesDiscovered++
		if err := in.fetchAndPersist(ctx, logger, s.RecNo, s.Archived, &stats); err != nil {
			return stats, err
		}
		stats.NewCasesFetched++
	}
	logger.Info("new-case discovery finished", "search_url", searchURL, "cases", stats.SearchCasesDiscovered)

	due, err := in.openCasesDue(ctx, in.now().Add(-in.RefreshInterval))
	if err != nil {
		return stats, err
	}
	for _, c := range due {
		if fetched[c.RecNo] {
			continue
		}
		fetched[c.RecNo] = true
		if err := in.fetchAndPersist(ctx, logger, c.RecNo, c.Archived, &stats); err != nil {
			return stats, err
		}
		stats.OpenCasesRefreshed++
	}

	if stats.PersistFailures > 0 {
		return stats, fmt.Errorf("%w: %d of %d", ErrPersistFailures, stats.PersistFailures, stats.CasePagesFetched)
	}
	return stats, nil
}

// fetchAndPersist reads one case page and persists it. Fetch and parse
// errors are returned (they abort the run); persist errors are counted.
func (in *Ingester) fetchAndPersist(ctx context.Context, logger *slog.Logger, recNo string, archived bool, stats *Stats) error {
	c, err := in.client.FetchCase(ctx, recNo, archived)
	if err != nil {
		return fmt.Errorf("case %s: %w", recNo, err)
	}
	stats.CasePagesFetched++
	if err := in.persist(ctx, c, stats); err != nil {
		stats.PersistFailures++
		logger.Error("case could not be persisted", "recno", recNo, "diary_number", c.DiaryNumber, "error", err)
		return nil
	}
	stats.RecordsObserved++
	return nil
}

// persist records the case observation and derives the events of its
// failed certificates in one transaction, so an event never points at an
// observation that was not committed.
func (in *Ingester) persist(ctx context.Context, c Case, stats *Stats) error {
	return database.InTx(ctx, in.pool, func(tx pgx.Tx) error {
		obs, inserted, err := in.repo.RecordObservation(ctx, tx, publicevent.NewObservation{
			Source:         Source,
			SourceRecordID: c.RecNo,
			SourceURL:      c.CaseURL,
			Payload:        c,
			Raw:            c.RawHTML,
		})
		if err != nil {
			return err
		}
		if inserted {
			stats.ObservationsInserted++
		}
		seen := map[string]bool{}
		for _, d := range c.Documents {
			if !IsFailedCertificate(d) {
				stats.DocumentsExcluded++
				continue
			}
			stats.FailedCertificates++
			id := EventID(c.RecNo, d)
			if seen[id] {
				stats.DuplicateCertificates++
				in.logger.Warn("case lists two indistinguishable failed certificates; treated as one event", "recno", c.RecNo, "diary_number", c.DiaryNumber, "source_event_id", id, "description", d.Description)
				continue
			}
			seen[id] = true
			ev, err := ToEvent(c, d)
			if err != nil {
				return err
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
		}
		return nil
	})
}

// failedCertificatePattern is the rule validated in
// docs/evaluations/stockholm-motorised-equipment-inspections.md: an
// inspection certificate whose description states the inspection was not
// approved. "godkänt" and "delvis godkänt" do not match.
var failedCertificatePattern = regexp.MustCompile(`(?i)^intyg (?:(?:återkommande |första |revisions)?besiktning|ombesiktning), ej godkän[dt]`)

// IsFailedCertificate reports whether a document is an explicitly failed
// inspection certificate: an incoming document whose description starts
// with the failed-certificate template. The case title is never enough.
func IsFailedCertificate(d Document) bool {
	if !strings.EqualFold(collapse(d.Category), CategoryIncoming) {
		return false
	}
	return failedCertificatePattern.MatchString(collapse(d.Description))
}

// EventID is the source event identifier of one document: the case RecNo
// and a fingerprint of the document's stable metadata (its timestamp,
// category and exact description). It does not depend on the document's
// position in the list, so reordering or later additions cannot change it.
// Two rows with identical metadata get the same identifier and are treated
// as one observable event.
func EventID(recNo string, d Document) string {
	sum := sha256.Sum256([]byte(recNo + "\n" + strings.TrimSpace(d.Timestamp) + "\n" + collapse(d.Category) + "\n" + collapse(d.Description)))
	return recNo + ":" + hex.EncodeToString(sum[:16])
}

// ToEvent maps a failed certificate of a case to the canonical event. The
// title is the source's document description, verbatim; the property is
// the case's, with the address only when the source gave one. Organisation
// and workplace stay empty: the source names neither. The observation ID
// is left for the caller.
func ToEvent(c Case, d Document) (publicevent.NewEvent, error) {
	ts, err := time.Parse(timestampLayout, d.Timestamp)
	if err != nil {
		return publicevent.NewEvent{}, fmt.Errorf("stockholm: document timestamp %q: %w", d.Timestamp, err)
	}
	if c.PropertyDesignation == "" {
		return publicevent.NewEvent{}, fmt.Errorf("stockholm: case %s has no property designation", c.RecNo)
	}
	return publicevent.NewEvent{
		Source:                   Source,
		SourceEventID:            EventID(c.RecNo, d),
		EventType:                publicevent.EventTypeMotorisedBuildingEquipmentInspectionFailed,
		OccurredOn:               time.Date(ts.Year(), ts.Month(), ts.Day(), 0, 0, 0, 0, time.UTC),
		Title:                    d.Description,
		PropertyMunicipalityCode: MunicipalityCode,
		PropertyDesignation:      c.PropertyDesignation,
		PropertyAddress:          c.Address,
		SourceURL:                c.CaseURL,
	}, nil
}

// knownCase is a locally known case due for a refresh.
type knownCase struct {
	RecNo    string
	Archived bool
}

// openCasesDueSQL picks, per Stockholm case, its latest observed state and
// keeps the ones the source still showed as open (no closure date) whose
// state was last confirmed before the cut-off. Oldest-checked first, so a
// bounded run still reaches every open case over time.
const openCasesDueSQL = `
	WITH latest AS (
		SELECT DISTINCT ON (source_record_id) source_record_id, payload, last_observed_at
		FROM source_observations
		WHERE source = $1
		ORDER BY source_record_id, first_observed_at DESC, id DESC
	)
	SELECT source_record_id, COALESCE((payload->>'archived')::boolean, false)
	FROM latest
	WHERE COALESCE(payload->>'closed_on', '') = ''
	  AND last_observed_at < $2
	ORDER BY last_observed_at, source_record_id
	LIMIT $3
`

// openCasesDue lists the known open cases whose latest state was last
// confirmed before cutoff.
func (in *Ingester) openCasesDue(ctx context.Context, cutoff time.Time) ([]knownCase, error) {
	limit := in.MaxRefreshPerRun
	if limit < 1 {
		limit = DefaultMaxRefreshPerRun
	}
	rows, err := in.pool.Query(ctx, openCasesDueSQL, Source, cutoff, limit)
	if err != nil {
		return nil, fmt.Errorf("stockholm: list open cases due for refresh: %w", err)
	}
	defer rows.Close()
	var out []knownCase
	for rows.Next() {
		var c knownCase
		if err := rows.Scan(&c.RecNo, &c.Archived); err != nil {
			return nil, fmt.Errorf("stockholm: scan open case: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("stockholm: read open cases: %w", err)
	}
	return out, nil
}
