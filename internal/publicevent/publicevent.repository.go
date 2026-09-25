package publicevent

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/sK4rdell/signal-engine/internal/platform/apperror"
	"github.com/sK4rdell/signal-engine/internal/platform/database"
)

// Repository persists source observations and public events. Public events
// are not account-scoped: they describe public facts, not customer data.
type Repository struct{}

// NewRepository returns the repository.
func NewRepository() *Repository { return &Repository{} }

// --- source observations ---------------------------------------------------

const observationColumns = `id, source, source_record_id, content_hash, source_url, payload, raw, first_observed_at, last_observed_at`

func scanObservation(row pgx.Row, inserted *bool) (SourceObservation, error) {
	var o SourceObservation
	var payload []byte
	dest := []any{&o.ID, &o.Source, &o.SourceRecordID, &o.ContentHash, &o.SourceURL, &payload, &o.Raw, &o.FirstObservedAt, &o.LastObservedAt}
	if inserted != nil {
		dest = append(dest, inserted)
	}
	if err := row.Scan(dest...); err != nil {
		return SourceObservation{}, err
	}
	o.Payload = payload
	return o, nil
}

const recordObservationSQL = `
	INSERT INTO source_observations (id, source, source_record_id, content_hash, source_url, payload, raw)
	VALUES ($1, $2, $3, $4, $5, $6, $7)
	ON CONFLICT ON CONSTRAINT source_observations_identity_key
	DO UPDATE SET last_observed_at = now()
	RETURNING ` + observationColumns + `, (xmax = 0) AS inserted
`

// RecordObservation stores an observation. The same record observed again
// with the same content returns the existing row with last_observed_at
// refreshed and inserted=false; changed content inserts a new row.
func (r *Repository) RecordObservation(ctx context.Context, db database.DBTX, in NewObservation) (SourceObservation, bool, error) {
	if in.Source == "" || in.SourceRecordID == "" {
		return SourceObservation{}, false, errors.New("publicevent: observation needs source and source record id")
	}
	payload, err := json.Marshal(in.Payload)
	if err != nil {
		return SourceObservation{}, false, fmt.Errorf("publicevent: encode observation payload: %w", err)
	}
	sum := sha256.Sum256(payload)
	id, err := uuid.NewV7()
	if err != nil {
		return SourceObservation{}, false, fmt.Errorf("publicevent: generate id: %w", err)
	}
	var inserted bool
	o, err := scanObservation(db.QueryRow(ctx, recordObservationSQL, id, in.Source, in.SourceRecordID, hex.EncodeToString(sum[:]), in.SourceURL, payload, in.Raw), &inserted)
	if err != nil {
		return SourceObservation{}, false, fmt.Errorf("publicevent: record observation %s/%s: %w", in.Source, in.SourceRecordID, err)
	}
	return o, inserted, nil
}

const listObservationsSQL = `
	SELECT ` + observationColumns + `
	FROM source_observations
	WHERE source = $1
	  AND source_record_id = $2
	ORDER BY first_observed_at, id
`

// ListObservations returns every observed state of one source record,
// oldest first.
func (r *Repository) ListObservations(ctx context.Context, db database.DBTX, source, sourceRecordID string) ([]SourceObservation, error) {
	rows, err := db.Query(ctx, listObservationsSQL, source, sourceRecordID)
	if err != nil {
		return nil, fmt.Errorf("publicevent: list observations: %w", err)
	}
	defer rows.Close()
	var out []SourceObservation
	for rows.Next() {
		o, err := scanObservation(rows, nil)
		if err != nil {
			return nil, fmt.Errorf("publicevent: scan observation: %w", err)
		}
		out = append(out, o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("publicevent: read observations: %w", err)
	}
	return out, nil
}

// --- public events ---------------------------------------------------------

const eventColumns = `id, source, source_event_id, event_type, occurred_on, title, organisation_number, organisation_name, workplace_cfar, workplace_name, property_municipality_code, property_designation, property_address, source_url, observation_id, first_observed_at, created_at, updated_at`

// eventColumnsOf prefixes every event column with a table alias for joins.
func eventColumnsOf(alias string) string {
	return alias + "." + strings.ReplaceAll(eventColumns, ", ", ", "+alias+".")
}

// scanEvent reads the event columns, then the optional record payload and
// the optional inserted flag, in that order.
func scanEvent(row pgx.Row, withRecord bool, inserted *bool) (PublicEvent, error) {
	var e PublicEvent
	var orgNr, cfar, municipality, designation, address *string
	var record []byte
	dest := []any{&e.ID, &e.Source, &e.SourceEventID, &e.EventType, &e.OccurredOn, &e.Title, &orgNr, &e.OrganisationName, &cfar, &e.WorkplaceName, &municipality, &designation, &address, &e.SourceURL, &e.ObservationID, &e.FirstObservedAt, &e.CreatedAt, &e.UpdatedAt}
	if withRecord {
		dest = append(dest, &record)
	}
	if inserted != nil {
		dest = append(dest, inserted)
	}
	if err := row.Scan(dest...); err != nil {
		return PublicEvent{}, err
	}
	if orgNr != nil {
		e.OrganisationNumber = *orgNr
	}
	if cfar != nil {
		e.WorkplaceCFAR = *cfar
	}
	if municipality != nil {
		e.PropertyMunicipalityCode = *municipality
	}
	if designation != nil {
		e.PropertyDesignation = *designation
	}
	if address != nil {
		e.PropertyAddress = *address
	}
	e.OccurredOn = e.OccurredOn.UTC()
	if withRecord {
		e.Record = record
	}
	return e, nil
}

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

const upsertEventSQL = `
	INSERT INTO public_events (id, source, source_event_id, event_type, occurred_on, title,
	                           organisation_number, organisation_name, workplace_cfar, workplace_name,
	                           property_municipality_code, property_designation, property_address,
	                           source_url, observation_id)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	ON CONFLICT ON CONSTRAINT public_events_source_event_key
	DO UPDATE SET event_type = EXCLUDED.event_type,
	              occurred_on = EXCLUDED.occurred_on,
	              title = EXCLUDED.title,
	              organisation_number = EXCLUDED.organisation_number,
	              organisation_name = EXCLUDED.organisation_name,
	              workplace_cfar = EXCLUDED.workplace_cfar,
	              workplace_name = EXCLUDED.workplace_name,
	              property_municipality_code = EXCLUDED.property_municipality_code,
	              property_designation = EXCLUDED.property_designation,
	              property_address = EXCLUDED.property_address,
	              source_url = EXCLUDED.source_url,
	              observation_id = EXCLUDED.observation_id,
	              updated_at = now()
	WHERE public_events.observation_id IS DISTINCT FROM EXCLUDED.observation_id
	RETURNING ` + eventColumns + `, (xmax = 0) AS inserted
`

const getEventBySourceSQL = `
	SELECT ` + eventColumns + `
	FROM public_events
	WHERE source = $1
	  AND source_event_id = $2
`

// UpsertEvent creates the event for a source record or refreshes it from a
// newer observation. An event whose observation is unchanged is left
// untouched (updated_at included) and reported as OutcomeUnchanged.
func (r *Repository) UpsertEvent(ctx context.Context, db database.DBTX, in NewEvent) (PublicEvent, UpsertOutcome, error) {
	if in.Source == "" || in.SourceEventID == "" || in.EventType == "" || in.ObservationID == uuid.Nil || in.OccurredOn.IsZero() {
		return PublicEvent{}, 0, errors.New("publicevent: event needs source, source event id, event type, observation and date")
	}
	if (in.PropertyMunicipalityCode == "") != (in.PropertyDesignation == "") || (in.PropertyAddress != "" && in.PropertyDesignation == "") {
		return PublicEvent{}, 0, errors.New("publicevent: a property needs both municipality code and designation; an address needs a property")
	}
	id, err := uuid.NewV7()
	if err != nil {
		return PublicEvent{}, 0, fmt.Errorf("publicevent: generate id: %w", err)
	}
	var inserted bool
	e, err := scanEvent(db.QueryRow(ctx, upsertEventSQL,
		id, in.Source, in.SourceEventID, string(in.EventType), in.OccurredOn, in.Title,
		nullIfEmpty(in.OrganisationNumber), in.OrganisationName, nullIfEmpty(in.WorkplaceCFAR), in.WorkplaceName,
		nullIfEmpty(in.PropertyMunicipalityCode), nullIfEmpty(in.PropertyDesignation), nullIfEmpty(in.PropertyAddress),
		in.SourceURL, in.ObservationID,
	), false, &inserted)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		// The WHERE clause skipped the update: same observation as before.
		e, err = scanEvent(db.QueryRow(ctx, getEventBySourceSQL, in.Source, in.SourceEventID), false, nil)
		if err != nil {
			return PublicEvent{}, 0, fmt.Errorf("publicevent: load unchanged event %s/%s: %w", in.Source, in.SourceEventID, err)
		}
		return e, OutcomeUnchanged, nil
	case err != nil:
		return PublicEvent{}, 0, fmt.Errorf("publicevent: upsert event %s/%s: %w", in.Source, in.SourceEventID, err)
	case inserted:
		return e, OutcomeInserted, nil
	default:
		return e, OutcomeUpdated, nil
	}
}

var getEventSQL = `
	SELECT ` + eventColumnsOf("e") + `, o.payload
	FROM public_events e
	JOIN source_observations o ON o.id = e.observation_id
	WHERE e.id = $1
`

// Get returns one event with its current observation payload, or ErrNotFound.
func (r *Repository) Get(ctx context.Context, db database.DBTX, id uuid.UUID) (PublicEvent, error) {
	e, err := scanEvent(db.QueryRow(ctx, getEventSQL, id), true, nil)
	if errors.Is(err, pgx.ErrNoRows) {
		return PublicEvent{}, ErrNotFound
	}
	if err != nil {
		return PublicEvent{}, fmt.Errorf("publicevent: get: %w", err)
	}
	return e, nil
}

// Filter narrows a listing. Zero values mean "no constraint". From and To
// are inclusive civil dates.
type Filter struct {
	Source             string
	EventType          EventType
	From               *time.Time
	To                 *time.Time
	OrganisationNumber string
}

// Page is one page of a listing.
type Page struct {
	Items      []PublicEvent
	NextCursor string
}

var listEventsSQL = `
	SELECT ` + eventColumnsOf("e") + `, o.payload
	FROM public_events e
	JOIN source_observations o ON o.id = e.observation_id
	WHERE ($1::text = '' OR e.source = $1)
	  AND ($2::text = '' OR e.event_type = $2)
	  AND ($3::date IS NULL OR e.occurred_on >= $3)
	  AND ($4::date IS NULL OR e.occurred_on <= $4)
	  AND ($5::text = '' OR e.organisation_number = $5)
	  AND ($6::date IS NULL OR (e.occurred_on, e.id) < ($6::date, $7::uuid))
	ORDER BY e.occurred_on DESC, e.id DESC
	LIMIT $8
`

// List returns events newest first (by occurred_on, then id), resuming
// after cursor. It fetches one row more than limit to know whether a next
// page exists.
func (r *Repository) List(ctx context.Context, db database.DBTX, f Filter, limit int, cursor string) (Page, error) {
	if limit < 1 {
		limit = DefaultLimit
	}
	if limit > MaxLimit {
		limit = MaxLimit
	}
	var afterOn *time.Time
	var afterID *uuid.UUID
	if cursor != "" {
		on, id, err := decodeCursor(cursor)
		if err != nil {
			return Page{}, err
		}
		afterOn, afterID = &on, &id
	}

	rows, err := db.Query(ctx, listEventsSQL, f.Source, string(f.EventType), f.From, f.To, f.OrganisationNumber, afterOn, afterID, limit+1)
	if err != nil {
		return Page{}, fmt.Errorf("publicevent: list: %w", err)
	}
	defer rows.Close()

	items := make([]PublicEvent, 0, limit)
	for rows.Next() {
		e, err := scanEvent(rows, true, nil)
		if err != nil {
			return Page{}, fmt.Errorf("publicevent: scan: %w", err)
		}
		items = append(items, e)
	}
	if err := rows.Err(); err != nil {
		return Page{}, fmt.Errorf("publicevent: read rows: %w", err)
	}

	page := Page{Items: items}
	if len(items) > limit {
		page.Items = items[:limit]
		last := page.Items[limit-1]
		page.NextCursor = encodeCursor(last.OccurredOn, last.ID)
	}
	return page, nil
}

// Cursors are opaque to clients: base64url of "<YYYY-MM-DD>|<id>".

const cursorDateLayout = "2006-01-02"

func encodeCursor(on time.Time, id uuid.UUID) string {
	return base64.RawURLEncoding.EncodeToString([]byte(on.UTC().Format(cursorDateLayout) + "|" + id.String()))
}

var errInvalidCursor = apperror.InvalidRequestFields("The request contains invalid parameters", map[string]string{"cursor": "is invalid"})

func decodeCursor(cursor string) (time.Time, uuid.UUID, error) {
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, uuid.Nil, errInvalidCursor
	}
	onPart, idPart, ok := strings.Cut(string(raw), "|")
	if !ok {
		return time.Time{}, uuid.Nil, errInvalidCursor
	}
	on, err := time.Parse(cursorDateLayout, onPart)
	if err != nil {
		return time.Time{}, uuid.Nil, errInvalidCursor
	}
	id, err := uuid.Parse(idPart)
	if err != nil {
		return time.Time{}, uuid.Nil, errInvalidCursor
	}
	return on, id, nil
}
