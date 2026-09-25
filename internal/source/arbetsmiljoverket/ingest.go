package arbetsmiljoverket

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
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

// Ingester fetches the documents of a feed for a date window and persists
// them as source observations and public events.
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
	Pages           int
	RecordsObserved int
	// RecordsAccepted are rows of the feed's document type that passed its
	// acceptance rule and were persisted (or failed to persist).
	RecordsAccepted int
	// RecordsSkipped are rows of the right document type that the feed's
	// acceptance rule rejected; they are logged and produce nothing.
	RecordsSkipped int
	// RecordsLaterInCase are accepted rows that are not the earliest
	// document of their type in their source case (feeds with OnePerCase);
	// they are logged and produce nothing.
	RecordsLaterInCase int
	// OtherDocumentTypes are rows the source returned despite the filter.
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
		"records_accepted", s.RecordsAccepted,
		"records_skipped", s.RecordsSkipped,
		"records_later_in_case", s.RecordsLaterInCase,
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

// Run ingests the default feed (inspection notices) for w; see RunFeed.
func (in *Ingester) Run(ctx context.Context, w Window) (Stats, error) {
	return in.RunFeed(ctx, DefaultFeed, w)
}

// RunFeed ingests every document of feed dated within w. It pages through
// the filtered search until the source reports no more rows. A transport,
// status or page-level parse error aborts the run with the stats so far; a
// malformed row or a failed persist is counted, logged and skipped, and so
// is a row the feed's acceptance rule rejects or, for a OnePerCase feed, a
// row that is not the earliest of its type in its case. Re-running over the
// same or an overlapping window is idempotent.
func (in *Ingester) RunFeed(ctx context.Context, feed Feed, w Window) (Stats, error) {
	var stats Stats
	if err := feed.validate(); err != nil {
		return stats, err
	}
	if err := w.Validate(); err != nil {
		return stats, err
	}
	maxPages := in.MaxPages
	if maxPages < 1 {
		maxPages = DefaultMaxPages
	}
	logger := in.logger.With("source", Source, "feed", feed.Name, "from", w.From.Format(dateLayout), "to", w.To.Format(dateLayout))

	seen := 0
	for page := 1; page <= maxPages; page++ {
		sp, err := in.client.Search(ctx, SearchQuery{
			From:         w.From,
			To:           w.To,
			SubjectArea:  feed.SubjectArea,
			DocumentType: feed.DocumentType,
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
			if doc.DocumentType != feed.DocumentTypeName {
				stats.OtherDocumentTypes++
				logger.Warn("source returned a document of another type despite the filter", "document_number", doc.DocumentNumber, "document_type", doc.DocumentType)
				continue
			}
			if ok, reason := feed.Accept(doc); !ok {
				stats.RecordsSkipped++
				logger.Info("source row does not match the feed's acceptance rule; skipped",
					"document_number", doc.DocumentNumber, "document_type", doc.DocumentType, "origin", doc.Origin,
					"case_title", doc.CaseTitle, "organisation_number", doc.OrganisationNumber, "reason", reason)
				continue
			}
			if feed.OnePerCase {
				earliest, err := in.earliestInCase(ctx, feed, doc)
				if err != nil {
					return stats, fmt.Errorf("page %d, document %s: %w", page, doc.DocumentNumber, err)
				}
				if earliest.DocumentNumber != doc.DocumentNumber {
					stats.RecordsLaterInCase++
					logger.Info("source row is not the earliest document of its type in its case; skipped",
						"document_number", doc.DocumentNumber, "document_date", doc.DocumentDate, "case_number", doc.CaseNumber,
						"earliest_document_number", earliest.DocumentNumber, "earliest_document_date", earliest.DocumentDate, "case_title", doc.CaseTitle)
					continue
				}
			}
			stats.RecordsAccepted++
			if err := in.persist(ctx, feed, doc, sp.URL, &stats); err != nil {
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
		return stats, fmt.Errorf("%w: %d of %d", ErrPersistFailures, stats.PersistFailures, stats.RecordsAccepted)
	}
	return stats, nil
}

// maxCasePages bounds the case-scoped search: a case holds a handful of
// documents of one type, never more than a few pages.
const maxCasePages = 10

// sourceEpoch is the first date the diary covers; the case-scoped search
// must see every earlier document of the case, whatever the run's window.
var sourceEpoch = time.Date(2015, 1, 1, 0, 0, 0, 0, time.UTC)

// earliestInCase asks the source for every document of the feed's type in
// doc's case up to doc's date and returns the earliest one by document
// date, then document-number suffix. It always includes doc itself, so the
// result is doc when nothing earlier exists. The answer comes from source
// chronology, not from what was ingested before, which keeps overlapping
// and out-of-order windows correct.
func (in *Ingester) earliestInCase(ctx context.Context, feed Feed, doc Document) (Document, error) {
	docDate, _ := time.Parse(dateLayout, doc.DocumentDate) // validated by the parser
	earliest := doc
	seen := 0
	for page := 1; page <= maxCasePages; page++ {
		sp, err := in.client.Search(ctx, SearchQuery{
			From:         sourceEpoch,
			To:           docDate,
			SubjectArea:  feed.SubjectArea,
			DocumentType: feed.DocumentType,
			Text:         doc.CaseNumber,
			Page:         page,
		})
		if err != nil {
			return Document{}, fmt.Errorf("case %s: %w", doc.CaseNumber, err)
		}
		if len(sp.Documents) == 0 && len(sp.RowErrors) == 0 {
			break
		}
		for _, other := range sp.Documents {
			// The text filter is a substring match; keep the case's own rows
			// of the feed's document type.
			if other.CaseNumber != doc.CaseNumber || other.DocumentType != feed.DocumentTypeName {
				continue
			}
			if documentBefore(other, earliest) {
				earliest = other
			}
		}
		seen += len(sp.Documents) + len(sp.RowErrors)
		if sp.Total > 0 && seen >= sp.Total {
			break
		}
	}
	return earliest, nil
}

// documentBefore orders two documents of one case by document date, then
// by document-number suffix. Both fields were validated by the parser.
func documentBefore(a, b Document) bool {
	if a.DocumentDate != b.DocumentDate {
		return a.DocumentDate < b.DocumentDate // ISO dates compare as strings
	}
	return documentSuffix(a.DocumentNumber) < documentSuffix(b.DocumentNumber)
}

// documentSuffix is the running number after the dash in a document number
// ("2026/044084-5" -> 5); 0 when malformed.
func documentSuffix(number string) int {
	if i := strings.LastIndexByte(number, '-'); i >= 0 {
		n, err := strconv.Atoi(number[i+1:])
		if err == nil {
			return n
		}
	}
	return 0
}

// persist records the observation and derives the event in one
// transaction, so an event never points at an observation that was not
// committed.
func (in *Ingester) persist(ctx context.Context, feed Feed, doc Document, pageURL string, stats *Stats) error {
	ev, notes := ToEvent(doc, feed.EventType)
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

// ToEvent maps a document to the canonical event of the given type. The
// title is the source's case title, verbatim. The observation ID is left
// for the caller. Values that fail validation are dropped (never guessed)
// and reported in the notes; they remain in the observation payload.
func ToEvent(doc Document, eventType publicevent.EventType) (publicevent.NewEvent, MappingNotes) {
	var notes MappingNotes
	occurredOn, _ := time.Parse(dateLayout, doc.DocumentDate) // validated by the parser

	ev := publicevent.NewEvent{
		Source:           Source,
		SourceEventID:    doc.DocumentNumber,
		EventType:        eventType,
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
