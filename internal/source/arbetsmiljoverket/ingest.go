package arbetsmiljoverket

import (
	"context"
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

// DefaultMaxPages bounds one run: 1000 pages is 10 000 rows, far beyond
// any sensible window, and stops a runaway loop if the source misbehaves.
const DefaultMaxPages = 1000

// Ingester fetches inspection notices for a date window and persists them
// as source observations and public events.
type Ingester struct {
	client   *Client
	repo     *publicevent.Repository
	pool     *pgxpool.Pool
	logger   *slog.Logger
	MaxPages int
}

// NewIngester wires an ingester.
func NewIngester(client *Client, repo *publicevent.Repository, pool *pgxpool.Pool, logger *slog.Logger) *Ingester {
	return &Ingester{client: client, repo: repo, pool: pool, logger: logger, MaxPages: DefaultMaxPages}
}

// Window is an inclusive range of civil dates.
type Window struct {
	From time.Time
	To   time.Time
}

// Validate checks the window is well formed.
func (w Window) Validate() error {
	if w.From.IsZero() || w.To.IsZero() {
		return errors.New("arbetsmiljoverket: window needs both from and to")
	}
	if w.To.Before(w.From) {
		return errors.New("arbetsmiljoverket: window end is before its start")
	}
	return nil
}

// Stats summarises one run.
type Stats struct {
	Pages                      int
	RecordsObserved            int
	InspectionNotices          int
	OtherDocumentTypes         int
	ParseFailures              int
	PersistFailures            int
	ObservationsInserted       int
	EventsInserted             int
	EventsUpdated              int
	EventsUnchanged            int
	InvalidOrganisationNumbers int
	InvalidWorkplaceCFARs      int
}

// LogAttrs returns the stats as structured logging attributes.
func (s Stats) LogAttrs() []any {
	return []any{
		"pages", s.Pages,
		"records_observed", s.RecordsObserved,
		"inspection_notices", s.InspectionNotices,
		"other_document_types", s.OtherDocumentTypes,
		"parse_failures", s.ParseFailures,
		"persist_failures", s.PersistFailures,
		"observations_inserted", s.ObservationsInserted,
		"events_inserted", s.EventsInserted,
		"events_updated", s.EventsUpdated,
		"events_unchanged", s.EventsUnchanged,
		"invalid_organisation_numbers", s.InvalidOrganisationNumbers,
		"invalid_workplace_cfars", s.InvalidWorkplaceCFARs,
	}
}

// ErrPersistFailures is returned (wrapped) when at least one record could
// not be persisted; the rest of the run completed and the stats are valid.
var ErrPersistFailures = errors.New("arbetsmiljoverket: some records could not be persisted")

// Run ingests every Inspektionsmeddelande dated within w. It pages through
// the filtered search until the source reports no more rows. A transport,
// status or page-level parse error aborts the run with the stats so far; a
// malformed row or a failed persist is counted, logged and skipped.
// Re-running over the same window is idempotent.
func (in *Ingester) Run(ctx context.Context, w Window) (Stats, error) {
	var stats Stats
	if err := w.Validate(); err != nil {
		return stats, err
	}
	maxPages := in.MaxPages
	if maxPages < 1 {
		maxPages = DefaultMaxPages
	}
	logger := in.logger.With("source", Source, "from", w.From.Format(dateLayout), "to", w.To.Format(dateLayout))

	seen := 0
	for page := 1; page <= maxPages; page++ {
		sp, err := in.client.Search(ctx, SearchQuery{
			From:         w.From,
			To:           w.To,
			SubjectArea:  SubjectAreaInspection,
			DocumentType: DocumentTypeInspectionNotice,
			Page:         page,
		})
		if err != nil {
			return stats, fmt.Errorf("page %d: %w", page, err)
		}
		stats.Pages++
		for _, re := range sp.RowErrors {
			stats.ParseFailures++
			logger.Warn("source row could not be parsed", "page", page, "row", re.Index, "error", re.Err)
		}
		if len(sp.Documents) == 0 && len(sp.RowErrors) == 0 {
			break
		}
		for _, doc := range sp.Documents {
			stats.RecordsObserved++
			if doc.DocumentType != InspectionNoticeTypeName {
				stats.OtherDocumentTypes++
				logger.Warn("source returned a document of another type despite the filter", "document_number", doc.DocumentNumber, "document_type", doc.DocumentType)
				continue
			}
			stats.InspectionNotices++
			if err := in.persist(ctx, doc, sp.URL, &stats); err != nil {
				stats.PersistFailures++
				logger.Error("record could not be persisted", "document_number", doc.DocumentNumber, "error", err)
			}
		}
		seen += len(sp.Documents) + len(sp.RowErrors)
		if sp.Total > 0 && seen >= sp.Total {
			break
		}
	}

	if stats.PersistFailures > 0 {
		return stats, fmt.Errorf("%w: %d of %d", ErrPersistFailures, stats.PersistFailures, stats.InspectionNotices)
	}
	return stats, nil
}

// persist records the observation and derives the event in one
// transaction, so an event never points at an observation that was not
// committed.
func (in *Ingester) persist(ctx context.Context, doc Document, pageURL string, stats *Stats) error {
	ev, notes := ToEvent(doc)
	if notes.InvalidOrganisationNumber {
		stats.InvalidOrganisationNumbers++
		in.logger.Warn("organisation number failed validation; stored without organisation identity", "document_number", doc.DocumentNumber)
	}
	if notes.InvalidWorkplaceCFAR {
		stats.InvalidWorkplaceCFARs++
		in.logger.Warn("workplace CFAR failed validation; stored without workplace identifier", "document_number", doc.DocumentNumber)
	}
	return database.InTx(ctx, in.pool, func(tx pgx.Tx) error {
		obs, inserted, err := in.repo.RecordObservation(ctx, tx, publicevent.NewObservation{
			Source:         Source,
			SourceRecordID: doc.DocumentNumber,
			SourceURL:      pageURL,
			Payload:        doc,
			Raw:            doc.RawHTML,
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

// MappingNotes reports source values that were dropped while mapping.
type MappingNotes struct {
	InvalidOrganisationNumber bool
	InvalidWorkplaceCFAR      bool
}

var cfarPattern = regexp.MustCompile(`^\d{8}$`)

// workplaceMissing is what the source prints as workplace name (and, on a
// few rows, as CFAR) when it has none.
const workplaceMissing = "saknas"

// ToEvent maps a document to the canonical event. The observation ID is
// left for the caller. Values that fail validation are dropped (never
// guessed) and reported in the notes; they remain in the observation
// payload.
func ToEvent(doc Document) (publicevent.NewEvent, MappingNotes) {
	var notes MappingNotes
	occurredOn, _ := time.Parse(dateLayout, doc.DocumentDate) // validated by the parser

	ev := publicevent.NewEvent{
		Source:           Source,
		SourceEventID:    doc.DocumentNumber,
		EventType:        publicevent.EventTypeWorkEnvironmentInspectionNotice,
		OccurredOn:       occurredOn,
		Title:            doc.CaseTitle,
		OrganisationName: doc.OrganisationName,
		WorkplaceName:    doc.WorkplaceName,
		SourceURL:        doc.CaseURL,
	}
	if doc.OrganisationNumber != "" {
		nr, err := publicevent.NormalizeOrganisationNumber(doc.OrganisationNumber)
		if err != nil {
			notes.InvalidOrganisationNumber = true
		} else {
			ev.OrganisationNumber = nr
		}
	}
	if strings.EqualFold(strings.TrimSpace(doc.WorkplaceName), workplaceMissing) {
		ev.WorkplaceName = ""
	}
	switch cfar := strings.TrimSpace(doc.WorkplaceCFAR); {
	case cfar == "" || strings.EqualFold(cfar, workplaceMissing):
	case cfarPattern.MatchString(cfar):
		ev.WorkplaceCFAR = cfar
	default:
		notes.InvalidWorkplaceCFAR = true
	}
	return ev, notes
}
