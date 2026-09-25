# Source: Stockholm Bygg- och plantjänsten (failed inspections of motorised building equipment)

Second Signal Engine source. Everything below was verified against the
live public service on 2026-09-25 with plain HTTP requests, no login, no
browser automation and no document ordered. The research behind it, with
the twelve-month counts and the municipal portability scan, is
[docs/evaluations/stockholm-motorised-equipment-inspections.md](../evaluations/stockholm-motorised-equipment-inspections.md).

## Verdict

Stockholm's building committee (stadsbyggnadskontoret) publishes every
supervision case for lifts, powered doors and gates, escalators and other
motorised building equipment in its public case service. One stateless
GET lists the cases started in a date window as embedded JSON; one GET
per case returns the property, the district, the address and the list of
registered documents. The document description states the outcome in the
source's own words ("Intyg återkommande besiktning, ej godkänt,
L2992070"), which is what the adapter turns into a public event. The
documents themselves are not fetched.

## What is ingested

| Selection | Rule | Event type |
| --- | --- | --- |
| cases of type `Funktionskontroll, Tillstånd` (`CaseTypeRecNo=200001`) in class `7.2` | every incoming document (`Skrivelse In`) whose description starts with `Intyg återkommande besiktning, ej godkänt` (also `Intyg första besiktning`, `Intyg revisionsbesiktning`, `Intyg ombesiktning`, `… ej godkänd`) | `MOTORISED_BUILDING_EQUIPMENT_INSPECTION_FAILED` |

```bash
make ingest source=stockholm from=2026-09-01 to=2026-09-24
go run ./cmd/ingest stockholm --from 2026-09-01 --to 2026-09-24
```

The window is the **case start date** (`Ärendestart`), not the document
date; the two coincide for the certificate that opens a case, which is
almost every failed certificate. Both ends are inclusive.

Why an incoming certificate in a 7.2 case means "failed": PBF 5 kap. 11 §
second paragraph obliges the accredited inspection body to send its
protocol to the building committee when the device has deficiencies of
the kind in PBL 10 kap. 20 § andra stycket 1, and the committee registers
that protocol as "Intyg …, ej godkänt". Approved certificates ("godkänt")
and partly approved ones ("delvis godkänt") reach the same case later, as
do the committee's use-prohibition reminders ("Påminnelse om
användningsförbud"), complaints and notes; none of those is an event.
The case title alone ("Anmaning att låta åtgärda hiss samt utföra förnyad
besiktning") is never sufficient: every 7.2 case carries that template
title whether or not a failed certificate has been registered.

Approved certificates are **not** events of any type. A case that closes
(`Ärendeavslut` set) leaves the refresh set (below) but closure is not
read as "passed": the source does not say why a case closed.

## Public entry point

```text
https://etjanster.stockholm.se/Byggochplantjansten/arendeochhandlingar
```

Search request (GET, unprefixed parameters; a `SearchParameters.` prefix
returns "Ange minst ett sökkriterium"):

```text
/Byggochplantjansten/arendeochhandlingar?CaseTypeRecNo=200001&JournalPlanCode=7.2&CaseStartDateFrom=2026-09-01&CaseStartDateTo=2026-09-24
```

The response is server-rendered HTML with the whole result set in one
script:

```text
var CaseSearchResultsViewModel = {"BuildCases":{"CaseSearchDetails":[…]},"RealEstateCases":{…},"OtherCases":{"CaseSearchDetails":[{"RecNo":"1327421","CaseTypeCode":"Funktionskontroll, Tillstånd","Description":"Anmaning att låta åtgärda hissar samt utföra förnyad besiktning","StartDate":"2026-09-23T00:00:00","RealEstateName":"Kronkvarnen 39","RealEstateAddress":"Artillerigatan 48","Name":"2026-15984","IsEarchive":false}, …]}};
```

Pagination is client-side; the embedded model is complete (verified for
a thirteen-month window of 2,193 cases, 32 MB cap in the client). 7.2
cases appear under `OtherCases`; the parser reads all three lists.

Case page (GET):

```text
/Byggochplantjansten/arende/arende/<RecNo>?dataSource=Active
```

Fields read from the definition list (`<dt class="dt-heading">`):
Diarienummer, Ärendegrupp, Diarieplansbeteckning, Fastighetsbeteckning,
Adress, Stadsdel, Ärendestart, Ärendeavslut, Handläggare, Ärendemening.
"Information saknas för "Adress"" is read as no address. The document
list is the JSON array passed to
`createInstance('documentList', …, [ {title, category, date, fileName}, … ], …)`
and its length must equal the `(N st)` count in the list heading;
otherwise the page is rejected as incomplete. `dataSource=Archived`
exists for e-archive cases (`IsEarchive`); none of the 7.2 cases seen so
far is archived.

Requests carry `User-Agent: signal-engine-ingest/0.1` and are paced one
per `STOCKHOLM_REQUEST_INTERVAL` (default 1 s). Response bodies are
bounded (32 MB search, 4 MB case page); a bigger body fails to parse.

## Source record, observation, event

* **Source record** = the case. `source_record_id` is the RecNo; the
  observation payload is the whole case (diary number, class, property,
  address, district, start and closure dates, officer, title, document
  count, every document with description, category, timestamp and file
  name); `raw` is the case page. A case whose document list changes is a
  new observation of the same record; the history is append-only.
* **Event** = one failed certificate. `source_event_id` is
  `<RecNo>:<hex>` where hex is the first 16 bytes of SHA-256 over the
  RecNo, the document timestamp (`2026-09-23T13:29:08`), the category
  and the whitespace-normalised description. The source has no document
  id, and the ordinal of a document in the list is not used: the list is
  newest first, so a later certificate would shift ordinals. Reordering
  the list or appending an approved certificate therefore never changes
  an existing event's identity, and a second, later failure in the same
  case is a second event. A certificate that names several devices
  ("ej godkänt, 55117, 53489, 55116") is one event; the identifiers are
  in the title and payload.
* Two rows with identical RecNo, timestamp, category and description
  are indistinguishable to the source's readers and become one event;
  the ingester counts them (`duplicate_certificates`) and logs the case.
  None was seen in 4,474 documents of the research population.
* Event fields: `occurred_on` = the document date, `title` = the
  description verbatim, organisation and workplace null, `property` =
  municipality `0180`, the designation (`Fastighetsbeteckning`) and the
  address when the case has one, `source.url` = the case page.

## Runs and open-case refresh

Every run does two things, in one process, without a scheduler:

1. **New case discovery**: search the window, fetch each case page once,
   record an observation and derive events.
2. **Known open case refresh**: for every case whose latest observation
   has no closure date and was last confirmed more than
   `STOCKHOLM_REFRESH_INTERVAL` ago (default 7 days), fetch the page
   again; oldest first, at most 500 per run (`MaxRefreshPerRun`). A case
   discovered in step 1 is not fetched twice. Refreshing an unchanged
   case only moves `last_observed_at`; a case that gained a failed
   certificate emits a new event; one that gained an approved certificate
   or closed records a new observation and no event.

Every case page that cannot be fetched or parsed aborts the run with an
error (the counters printed are what was persisted before the failure,
never a claim of completeness); a persistence failure is counted and the
run continues. Re-running a window is idempotent.

## Environment

```text
STOCKHOLM_BASE_URL          https://etjanster.stockholm.se
STOCKHOLM_REQUEST_INTERVAL  1s
STOCKHOLM_REFRESH_INTERVAL  168h
```

## Limitations

* The source identifies the property, not its owner; resolving the
  owner (Lantmäteriet, or the housing association / company behind a
  designation) is a later step and is not guessed.
* A street address is missing on about 2 % of cases; the designation
  and district are always present.
* Nearly every lift is under a service contract, so a failed certificate
  is an open remediation need, not necessarily an open supplier selection.
* Cases in class `587` (before 2024) are not searched; a historical
  backfill would need a second class code.
* Stockholm only; the other municipalities scanned do not expose these
  cases the same way (see the evaluation, section 13).
