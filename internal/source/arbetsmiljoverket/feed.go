package arbetsmiljoverket

import (
	"errors"
	"fmt"
	"strings"

	"github.com/sK4rdell/signal-engine/internal/publicevent"
)

// Feed is one known selection of the diary: the subject area and document
// type to search for, the document type name every accepted row must carry,
// an acceptance rule for the rows the search returns, and the canonical
// event type an accepted row becomes. The two feeds below are the ones
// verified against the live source; see docs/sources/arbetsmiljoverket.md.
type Feed struct {
	// Name identifies the feed on the command line and in logs.
	Name string
	// SubjectArea and DocumentType are the search filter codes.
	SubjectArea  string
	DocumentType string
	// DocumentTypeName is the row heading the source prints for the
	// document type; rows with another heading are skipped as a guard.
	DocumentTypeName string
	// EventType is the canonical event type of accepted rows.
	EventType publicevent.EventType
	// accept decides whether a row of the right document type is an event
	// of this feed. The reason is logged when a row is skipped.
	accept func(Document) (ok bool, reason string)
}

// Accept reports whether doc is an event of the feed and, when not, why.
// It assumes the document type already matched.
func (f Feed) Accept(doc Document) (bool, string) {
	if f.accept == nil {
		return true, ""
	}
	return f.accept(doc)
}

// validate checks the feed is fully described.
func (f Feed) validate() error {
	if f.Name == "" || f.SubjectArea == "" || f.DocumentType == "" || f.DocumentTypeName == "" || f.EventType == "" {
		return fmt.Errorf("arbetsmiljoverket: feed %q is not fully configured", f.Name)
	}
	return nil
}

// Source-specific codes of the second feed. Verified against the live
// service on 2026-09-25.
const (
	// DocumentTypeRecurringInspectionCertificate is the handlingstyp code of
	// "Intyg återkommande besiktning".
	DocumentTypeRecurringInspectionCertificate = "6.1-49"
	// RecurringInspectionCertificateTypeName is the document type label
	// shown on a certificate row.
	RecurringInspectionCertificateTypeName = "Intyg återkommande besiktning"
	// originIncoming is "handlingens ursprung" of a document the authority
	// received.
	originIncoming = "Inkommande"
	// recurringInspectionTitlePrefix starts the case title of a recurring
	// inspection case; compared case-insensitively.
	recurringInspectionTitlePrefix = "återkommande besiktning"
)

// FeedInspectionNotices is the original feed: outgoing inspection notices
// (Inspektionsmeddelande) after an inspection found deficiencies.
var FeedInspectionNotices = Feed{
	Name:             "inspection-notices",
	SubjectArea:      SubjectAreaInspection,
	DocumentType:     DocumentTypeInspectionNotice,
	DocumentTypeName: InspectionNoticeTypeName,
	EventType:        publicevent.EventTypeWorkEnvironmentInspectionNotice,
}

// FeedRecurringInspectionFailures is the second feed: incoming certificates
// from accredited inspection bodies. AFS 2023:11 13 kap. 13 § obliges the
// body to notify Arbetsmiljöverket when a device "inte erbjuder betryggande
// säkerhet", so an incoming certificate in a case titled "Återkommande
// besiktning - <device>" is a device that failed its recurring inspection.
// Rows with other titles ("För kännedom - godkänd besiktning", inspection
// campaigns) or another origin are not failures and are skipped.
var FeedRecurringInspectionFailures = Feed{
	Name:             "recurring-inspection-failures",
	SubjectArea:      SubjectAreaInspection,
	DocumentType:     DocumentTypeRecurringInspectionCertificate,
	DocumentTypeName: RecurringInspectionCertificateTypeName,
	EventType:        publicevent.EventTypeWorkEquipmentInspectionFailed,
	accept:           acceptRecurringInspectionFailure,
}

// DefaultFeed is what ingestion runs when no feed is named.
var DefaultFeed = FeedInspectionNotices

// Feeds lists the known feeds in the order they are documented.
func Feeds() []Feed {
	return []Feed{FeedInspectionNotices, FeedRecurringInspectionFailures}
}

// ErrUnknownFeed is returned (wrapped) by FeedByName.
var ErrUnknownFeed = errors.New("arbetsmiljoverket: unknown feed")

// FeedByName resolves a feed name from the command line.
func FeedByName(name string) (Feed, error) {
	for _, f := range Feeds() {
		if f.Name == name {
			return f, nil
		}
	}
	names := make([]string, 0, len(Feeds()))
	for _, f := range Feeds() {
		names = append(names, f.Name)
	}
	return Feed{}, fmt.Errorf("%w %q (known feeds: %s)", ErrUnknownFeed, name, strings.Join(names, ", "))
}

// acceptRecurringInspectionFailure is the acceptance rule validated in
// docs/evaluations/arbetsmiljoverket-deterministic-signals.md: the
// certificate must have come in to the authority and the case must be a
// recurring-inspection case. The document type is checked by the caller.
func acceptRecurringInspectionFailure(doc Document) (bool, string) {
	if !strings.EqualFold(strings.TrimSpace(doc.Origin), originIncoming) {
		return false, fmt.Sprintf("origin is %q, not %s", doc.Origin, originIncoming)
	}
	title := strings.ToLower(strings.TrimSpace(doc.CaseTitle))
	if !strings.HasPrefix(title, recurringInspectionTitlePrefix) {
		return false, "case title does not start with \"Återkommande besiktning\""
	}
	return true, ""
}
