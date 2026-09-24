// Package publicevent is the source-independent domain of Signal Engine: a
// public event is something a public source reported about an identifiable
// organisation or workplace.
//
// Two facts are kept apart on purpose. A SourceObservation is what a source
// showed us, verbatim and append-only, in the source's own vocabulary. A
// PublicEvent is our canonical reading of the latest observation of one
// source record. Interpretation (what the event might mean commercially)
// is not part of this package.
package publicevent

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/sK4rdell/signal-engine/internal/platform/apperror"
)

// Sources known to the domain. The value is stored on observations and
// events and accepted by the source filter of the listing.
const (
	SourceArbetsmiljoverket = "arbetsmiljoverket"
	SourceKlimatklivet      = "klimatklivet"
)

// EventType says what happened, independently of which source reported it.
type EventType string

// Event types known to the domain.
const (
	// EventTypeWorkEnvironmentInspectionNotice: a work environment authority
	// inspected a workplace, found deficiencies and issued a written notice
	// to the employer.
	EventTypeWorkEnvironmentInspectionNotice EventType = "WORK_ENVIRONMENT_INSPECTION_NOTICE"
	// EventTypeClimateInvestmentGrantApproved: a public authority approved a
	// grant for a specific climate investment by an identifiable
	// organisation. The date is the decision date.
	EventTypeClimateInvestmentGrantApproved EventType = "CLIMATE_INVESTMENT_GRANT_APPROVED"
)

// SourceObservation is one observed state of one source record.
type SourceObservation struct {
	ID             uuid.UUID
	Source         string
	SourceRecordID string
	// ContentHash identifies the observed state: SHA-256 of Payload.
	ContentHash string
	// SourceURL is the page the record was observed on.
	SourceURL string
	// Payload is the parsed record in the source's vocabulary.
	Payload json.RawMessage
	// Raw is the source fragment Payload was parsed from.
	Raw             string
	FirstObservedAt time.Time
	LastObservedAt  time.Time
}

// NewObservation is the input for recording an observation. Payload is
// marshalled to canonical JSON and hashed by the repository.
type NewObservation struct {
	Source         string
	SourceRecordID string
	SourceURL      string
	Payload        any
	Raw            string
}

// PublicEvent is the canonical event derived from a source record.
type PublicEvent struct {
	ID            uuid.UUID
	Source        string
	SourceEventID string
	EventType     EventType
	// OccurredOn is a civil date (UTC midnight): sources publish dates, not
	// instants.
	OccurredOn time.Time
	Title      string
	// OrganisationNumber is the normalised ten-digit Swedish organisation
	// number, or "" when the source did not identify the organisation.
	OrganisationNumber string
	OrganisationName   string
	// WorkplaceCFAR is the SCB workplace identifier, or "" when unknown.
	WorkplaceCFAR string
	WorkplaceName string
	SourceURL     string
	// ObservationID is the observation the event was last derived from.
	ObservationID   uuid.UUID
	FirstObservedAt time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time

	// Record is the payload of the current observation. It is populated by
	// Get and List (which join the observation), not by UpsertEvent.
	Record json.RawMessage
}

// NewEvent is the input for creating or refreshing an event from an
// observation. Empty OrganisationNumber and WorkplaceCFAR are stored as
// NULL.
type NewEvent struct {
	Source             string
	SourceEventID      string
	EventType          EventType
	OccurredOn         time.Time
	Title              string
	OrganisationNumber string
	OrganisationName   string
	WorkplaceCFAR      string
	WorkplaceName      string
	SourceURL          string
	ObservationID      uuid.UUID
}

// UpsertOutcome reports what UpsertEvent did.
type UpsertOutcome int

// Upsert outcomes.
const (
	// OutcomeInserted: the source record was seen for the first time.
	OutcomeInserted UpsertOutcome = iota + 1
	// OutcomeUpdated: the record was known and its observation changed.
	OutcomeUpdated
	// OutcomeUnchanged: the record was known and re-observed unchanged.
	OutcomeUnchanged
)

// String names the outcome for logs and stats.
func (o UpsertOutcome) String() string {
	switch o {
	case OutcomeInserted:
		return "inserted"
	case OutcomeUpdated:
		return "updated"
	case OutcomeUnchanged:
		return "unchanged"
	}
	return "unknown"
}

// CodeNotFound is answered for unknown event IDs.
const CodeNotFound = "public_event_not_found"

// ErrNotFound is the not-found error of the feature.
var ErrNotFound = apperror.NotFound(CodeNotFound, "Public event not found")

// Page-size bounds for listing.
const (
	DefaultLimit = 50
	MaxLimit     = 100
)
