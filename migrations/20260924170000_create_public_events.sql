-- +goose Up
-- Source observations are the append-only record of what a public source
-- showed us: one row per (source, record, content). Re-observing an
-- unchanged record bumps last_observed_at; a changed record adds a row, so
-- the history of a source record is never overwritten.
CREATE TABLE source_observations (
    id                uuid        PRIMARY KEY,
    source            text        NOT NULL,
    source_record_id  text        NOT NULL,
    -- SHA-256 of the canonical JSON payload: the identity of one observed state.
    content_hash      text        NOT NULL,
    -- Where the record was observed (the listing page that showed it).
    source_url        text        NOT NULL,
    -- Parsed source snapshot in the source's own vocabulary.
    payload           jsonb       NOT NULL,
    -- Raw source fragment the payload was parsed from, kept for re-parsing.
    raw               text        NOT NULL,
    first_observed_at timestamptz NOT NULL DEFAULT now(),
    last_observed_at  timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT source_observations_identity_key UNIQUE (source, source_record_id, content_hash)
);

-- Public events are the canonical, source-independent view: one row per
-- source record, always derived from its most recent observation.
CREATE TABLE public_events (
    id                  uuid        PRIMARY KEY,
    source              text        NOT NULL,
    source_event_id     text        NOT NULL,
    event_type          text        NOT NULL,
    -- Civil date the event happened on; sources only publish dates.
    occurred_on         date        NOT NULL,
    title               text        NOT NULL,
    -- Swedish organisation number, normalised to ten digits. NULL when the
    -- source did not identify the organisation; never guessed.
    organisation_number text,
    organisation_name   text        NOT NULL DEFAULT '',
    -- SCB workplace identifier (CFAR). NULL when unknown.
    workplace_cfar      text,
    workplace_name      text        NOT NULL DEFAULT '',
    source_url          text        NOT NULL,
    -- Provenance: the observation this row was last derived from.
    observation_id      uuid        NOT NULL REFERENCES source_observations (id),
    first_observed_at   timestamptz NOT NULL DEFAULT now(),
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT public_events_source_event_key UNIQUE (source, source_event_id),
    CONSTRAINT public_events_organisation_number_check CHECK (organisation_number ~ '^[0-9]{10}$'),
    CONSTRAINT public_events_workplace_cfar_check CHECK (workplace_cfar ~ '^[0-9]{8}$')
);

-- Listing order and keyset pagination: ORDER BY occurred_on DESC, id DESC.
CREATE INDEX public_events_occurred_idx ON public_events (occurred_on DESC, id DESC);
-- Lookup by organisation.
CREATE INDEX public_events_organisation_idx ON public_events (organisation_number) WHERE organisation_number IS NOT NULL;

-- +goose Down
DROP TABLE public_events;
DROP TABLE source_observations;
