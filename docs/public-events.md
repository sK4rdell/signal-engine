# Public events

The domain of Signal Engine: public events reported by public sources about
an identifiable subject: an organisation and its workplace, or a property.
This document describes the persistence semantics and the dependency
direction between a source adapter and the domain.

```text
source adapter (internal/source/<source>)
        ↓ Document (source vocabulary)
source observation (source_observations)   append-only, what the source showed us
        ↓ derived
public event (public_events)               canonical, source-independent current view
        ↓
GET /v1/public-events                      provenance visible on every response
```

## Source truth versus interpretation

A **source observation** is source truth: "Arbetsmiljöverket listed
document X of case Y for organisation Z on date D with type T". It is
stored in the source's own vocabulary (`payload`), together with the raw
fragment it was parsed from (`raw`), the page it was seen on
(`source_url`) and a content hash.

A **public event** is our canonical reading of the latest observation of a
source record: event type, date, subject (organisation and workplace, or
property), title and a link back to the source. It contains no judgement about commercial value. An
interpretation (an *assessment*) would be a separate record pointing at the
event; none exists yet.

Event types are neutral statements of what happened, independent of the
source that reported it:

| Event type | Meaning | Produced by |
| --- | --- | --- |
| `WORK_ENVIRONMENT_INSPECTION_NOTICE` | a work environment authority inspected a workplace, found deficiencies and issued a written notice | Arbetsmiljöverket feed `inspection-notices` |
| `WORK_EQUIPMENT_INSPECTION_FAILED` | an accredited inspection body found a piece of work equipment did not meet the required safety standard and its certificate was registered by the authority; one event per source case (the certificate that opened it), later re-inspection certificates are not events | Arbetsmiljöverket feed `recurring-inspection-failures` |
| `MOTORISED_BUILDING_EQUIPMENT_INSPECTION_FAILED` | an accredited inspection body found a lift, powered door or gate, escalator or similar motorised building device did not pass its mandatory inspection and the municipal building committee registered the certificate; one event per failed certificate, a later approved certificate is not an event | Stockholm Bygg- och plantjänsten |

The title of an event is the source's own wording (for Arbetsmiljöverket
the case title, e.g. "Återkommande besiktning - Fordonslyft
flerpelarlyft"; for Stockholm the document description, e.g. "Intyg
återkommande besiktning, ej godkänt, L2992070"). Classifications derived
from that wording, such as the kind of device, are interpretation and are
not fields of the event.

## Subjects: organisation, workplace, property

An event is about one of two kinds of subject, and the source decides
which:

* an **organisation** (ten-digit organisation number, name) and its
  **workplace** (eight-digit CFAR, name), as Arbetsmiljöverket reports;
* a **property**: the four-digit municipality code, the cadastral
  designation (`Fastighetsbeteckning`, e.g. "Kronkvarnen 39") and, when
  the source has one, a street address, as Stockholm reports.

The property is three plain columns on `public_events`
(`property_municipality_code`, `property_designation`,
`property_address`), constrained to be either all absent or a valid code
with a non-empty designation; the address is optional. It is deliberately
not a parsed cadastral hierarchy, not a coordinate, and not an owner: the
owner of a property is not in these sources and is never guessed. Events
from before the column existed keep `property = null`.

## Observation semantics

`source_observations` is append-only per observed state:

* identity is `(source, source_record_id, content_hash)` where the hash is
  SHA-256 of the canonical JSON payload;
* re-observing an unchanged record refreshes `last_observed_at` on the
  existing row and inserts nothing;
* a changed record inserts a new row, so the history of a source record is
  never overwritten and parsing decisions can be reproduced from `raw`.

## Event semantics

`public_events` holds one row per `(source, source_event_id)`:

* inserted when a source record is first observed;
* updated from a newer observation when the content changed
  (`observation_id` then points at the new observation and `updated_at`
  moves);
* left untouched when re-observed unchanged;
* `first_observed_at` is set once and never moves.

Re-running an ingestion over the same window is therefore idempotent.

Organisation numbers are normalised to ten digits and Luhn-checked before
they reach an event; a value that fails validation is dropped from the
event (never guessed) and remains visible in the observation payload. The
same applies to the eight-digit CFAR workplace identifier.

What a *source event* is depends on the source: for Arbetsmiljöverket it
is a diary document with its own document number; for Stockholm, whose
documents have no public id, it is a failed certificate identified by a
hash of its stable metadata within the case (see
`docs/sources/stockholm.md`). Either way the id is derived from source
facts, never from the position of a row on a page.

## API

```text
GET /v1/public-events?source=&event_type=&from=&to=&organisation_number=&limit=&cursor=
GET /v1/public-events/{eventID}
```

Authentication is required; events are not account-scoped. Dates are civil
dates (`YYYY-MM-DD`), both ends inclusive. Listing is newest first by
`occurred_on`, then `id`, with an opaque cursor.

Response shape:

```json
{
  "id": "0199…",
  "event_type": "WORK_ENVIRONMENT_INSPECTION_NOTICE",
  "occurred_on": "2026-09-23",
  "title": "Inspektion inom Fortlöpande tillsyn - Unga i arbetslivet",
  "organisation": {"organisation_number": "5594800418", "name": "MITTEN MACK AB"},
  "workplace": {"cfar": "71466304", "name": "MITTEN MACK AB"},
  "property": null,
  "source": {
    "name": "arbetsmiljoverket",
    "source_event_id": "2026/060943-2",
    "url": "https://www.av.se/om-oss/diarium-och-allmanna-handlingar/bestall-handlingar/Case/?id=2026/060943",
    "observation_id": "0199…",
    "first_observed_at": "2026-09-24T17:02:11Z",
    "record": { "...parsed source payload..." }
  },
  "created_at": "…",
  "updated_at": "…"
}
```

`organisation` is `null` when the source did not identify one; so are
`workplace` and `property`. A property event looks like this (organisation
and workplace null, `address` null when the source has none):

```json
{
  "event_type": "MOTORISED_BUILDING_EQUIPMENT_INSPECTION_FAILED",
  "occurred_on": "2026-09-23",
  "title": "Intyg återkommande besiktning, ej godkänt, 53119",
  "organisation": null,
  "workplace": null,
  "property": {"municipality_code": "0180", "designation": "Kronkvarnen 39", "address": "Artillerigatan 48"},
  "source": {
    "name": "stockholm",
    "source_event_id": "1327421:5a0d…",
    "url": "https://etjanster.stockholm.se/Byggochplantjansten/arende/arende/1327421?dataSource=Active",
    "record": { "...the case with its document list..." }
  }
}
```

`record` is the observation payload the event was derived from, so a
reader can always see the source's own words next to our canonical
reading. `source` accepts `arbetsmiljoverket` and `stockholm`.

## Ingestion

```bash
make ingest from=2026-09-20 to=2026-09-23                                     # inspection notices (default feed)
make ingest feed=recurring-inspection-failures from=2026-09-20 to=2026-09-23  # failed recurring inspections
make ingest source=stockholm from=2026-09-01 to=2026-09-24                    # failed motorised-equipment inspections
# go run ./cmd/ingest arbetsmiljoverket [--feed inspection-notices|recurring-inspection-failures] --from … --to …
# go run ./cmd/ingest stockholm --from … --to …
```

There is no scheduler; the command is run by hand. A Stockholm run also
re-reads known open cases that have not been confirmed for a week, so
certificates registered after a case started are picked up without a
second command. Source specifics are in `docs/sources/`.
