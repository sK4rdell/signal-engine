package publicevent

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sK4rdell/signal-engine/internal/platform/api"
	"github.com/sK4rdell/signal-engine/internal/platform/apperror"
)

// dateLayout is the wire format of civil dates.
const dateLayout = "2006-01-02"

// Response is the public shape of an event. Provenance is explicit: the
// source block names the source, its own identifier for the record, the
// page it came from and the parsed source record the event was derived
// from.
type Response struct {
	ID         uuid.UUID `json:"id"`
	EventType  EventType `json:"event_type"`
	OccurredOn string    `json:"occurred_on"`
	Title      string    `json:"title"`
	// Organisation is null when the source did not identify one.
	Organisation *OrganisationResponse `json:"organisation"`
	// Workplace is null when the source gave neither identifier nor name.
	Workplace *WorkplaceResponse `json:"workplace"`
	Source    SourceResponse     `json:"source"`
	CreatedAt time.Time          `json:"created_at"`
	UpdatedAt time.Time          `json:"updated_at"`
}

// OrganisationResponse identifies the legal organisation.
type OrganisationResponse struct {
	OrganisationNumber *string `json:"organisation_number"`
	Name               string  `json:"name"`
}

// WorkplaceResponse identifies the physical workplace.
type WorkplaceResponse struct {
	CFAR *string `json:"cfar"`
	Name string  `json:"name"`
}

// SourceResponse is the provenance of an event.
type SourceResponse struct {
	Name            string          `json:"name"`
	SourceEventID   string          `json:"source_event_id"`
	URL             string          `json:"url"`
	ObservationID   uuid.UUID       `json:"observation_id"`
	FirstObservedAt time.Time       `json:"first_observed_at"`
	Record          json.RawMessage `json:"record"`
}

func toResponse(e PublicEvent) Response {
	res := Response{
		ID:         e.ID,
		EventType:  e.EventType,
		OccurredOn: e.OccurredOn.Format(dateLayout),
		Title:      e.Title,
		Source: SourceResponse{
			Name:            e.Source,
			SourceEventID:   e.SourceEventID,
			URL:             e.SourceURL,
			ObservationID:   e.ObservationID,
			FirstObservedAt: e.FirstObservedAt,
			Record:          e.Record,
		},
		CreatedAt: e.CreatedAt,
		UpdatedAt: e.UpdatedAt,
	}
	if e.OrganisationNumber != "" || e.OrganisationName != "" {
		res.Organisation = &OrganisationResponse{OrganisationNumber: nullIfEmpty(e.OrganisationNumber), Name: e.OrganisationName}
	}
	if e.WorkplaceCFAR != "" || e.WorkplaceName != "" {
		res.Workplace = &WorkplaceResponse{CFAR: nullIfEmpty(e.WorkplaceCFAR), Name: e.WorkplaceName}
	}
	if res.Source.Record == nil {
		res.Source.Record = json.RawMessage("null")
	}
	return res
}

// ListRequest binds the query of GET /v1/public-events.
type ListRequest struct {
	Source             string `query:"source" json:"-" validate:"omitempty,oneof=arbetsmiljoverket"`
	EventType          string `query:"event_type" json:"-" validate:"omitempty,oneof=WORK_ENVIRONMENT_INSPECTION_NOTICE"`
	From               string `query:"from" json:"-" validate:"omitempty,datetime=2006-01-02"`
	To                 string `query:"to" json:"-" validate:"omitempty,datetime=2006-01-02"`
	OrganisationNumber string `query:"organisation_number" json:"-" validate:"omitempty,max=20"`
	Limit              int    `query:"limit" json:"-" validate:"omitempty,min=1,max=100"`
	Cursor             string `query:"cursor" json:"-" validate:"omitempty,max=200"`
}

// ListResponse is the body of the listing.
type ListResponse struct {
	Items      []Response `json:"items"`
	NextCursor *string    `json:"next_cursor"`
}

// List handles GET /v1/public-events.
func List(repo *Repository, pool *pgxpool.Pool) api.HandlerFunc[ListRequest, ListResponse] {
	return func(ctx context.Context, req ListRequest) (ListResponse, error) {
		f := Filter{Source: req.Source, EventType: EventType(req.EventType)}
		if req.From != "" {
			from, _ := time.Parse(dateLayout, req.From)
			f.From = &from
		}
		if req.To != "" {
			to, _ := time.Parse(dateLayout, req.To)
			f.To = &to
		}
		if f.From != nil && f.To != nil && f.To.Before(*f.From) {
			return ListResponse{}, apperror.Validation(map[string]string{"to": "must not be before from"})
		}
		if req.OrganisationNumber != "" {
			nr, err := NormalizeOrganisationNumber(req.OrganisationNumber)
			if err != nil {
				return ListResponse{}, apperror.Validation(map[string]string{"organisation_number": "must be a valid Swedish organisation number"})
			}
			f.OrganisationNumber = nr
		}

		page, err := repo.List(ctx, pool, f, req.Limit, req.Cursor)
		if err != nil {
			return ListResponse{}, err
		}
		res := ListResponse{Items: make([]Response, 0, len(page.Items))}
		for _, e := range page.Items {
			res.Items = append(res.Items, toResponse(e))
		}
		if page.NextCursor != "" {
			res.NextCursor = &page.NextCursor
		}
		return res, nil
	}
}

// GetRequest binds GET /v1/public-events/{eventID}.
type GetRequest struct {
	EventID uuid.UUID `path:"eventID" json:"-" validate:"required"`
}

// Get handles GET /v1/public-events/{eventID}.
func Get(repo *Repository, pool *pgxpool.Pool) api.HandlerFunc[GetRequest, Response] {
	return func(ctx context.Context, req GetRequest) (Response, error) {
		e, err := repo.Get(ctx, pool, req.EventID)
		if err != nil {
			return Response{}, err
		}
		return toResponse(e), nil
	}
}

// RegisterRoutes mounts the public-event endpoints behind authentication.
// Events are public facts, not account data, so no membership is required.
func RegisterRoutes(r chi.Router, repo *Repository, pool *pgxpool.Pool, authenticated api.Middleware) {
	r.Route("/v1/public-events", func(r chi.Router) {
		r.Use(authenticated)
		r.Get("/", api.Handle(http.StatusOK, List(repo, pool)))
		r.Get("/{eventID}", api.Handle(http.StatusOK, Get(repo, pool)))
	})
}
