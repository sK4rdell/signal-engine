# Source: Arbetsmiljöverket web diary (webbdiarium)

Research note for the first Signal Engine source. Everything below was
verified against the live public service on 2026-09-24 with plain HTTP
requests (curl), no browser automation.

## Verdict

Ingestion of `Inspektionsmeddelande` **metadata** is viable without browser
automation: a stateless GET returns server-rendered HTML with a working
document-type filter, an inclusive date window and page-number pagination.
The **content** of the documents (the actual deficiencies) is *not*
programmatically accessible: documents must be ordered through a shopping
cart and are delivered by email after a manual secrecy review. See
"Document content" below.

## Public entry point

```text
https://www.av.se/om-oss/diarium-och-allmanna-handlingar/bestall-handlingar/
```

Linked from "Diarium och allmänna handlingar" on av.se. The older URL
`/om-oss/sok-i-arbetsmiljoverkets-diarium/` redirects (301) here; that old
path is listed in `robots.txt` as disallowed, the current path is not.

The service is an ASP.NET application (`ASP.NET_SessionId` cookie is set,
`Cache-Control: private`). Searching does **not** depend on the session:
every page below was fetched without a cookie jar. One search page is
~310 KB (the form embeds every municipality) and answers in ~0.3 s.

The diary covers documents registered from 2015-01-01 up to and including
**yesterday** (source freshness: one-day lag).

## Search request

`GET` with query parameters (names are the form field names):

| Parameter | Values | Notes |
| --- | --- | --- |
| `SearchText` | free text | Optional. Supports `*` suffix wildcard. Matches case/document numbers, org numbers, org/workplace names, titles. |
| `FromDate` / `ToDate` | `YYYY-MM-DD` | "Handlingens datum": the date the document was sent, received or finalised. **Both ends inclusive** (a single-day window `2026-09-22..2026-09-22` returned only `2026-09-22` documents). |
| `SelectedArendeProcess` | `6.1` = *Bedriva inspektion* | Subject area (ämnesområde). `7.1` = *Bedriva marknadskontroll*, etc. Empty = all. |
| `SelectedHandlingType` | `6.1-23` = *Inspektionsmeddelande* | Document type (handlingstyp). Related codes: `6.1-20` *Dokumenterad bedömning inklusive brister och krav*, `6.1-24` *Tillsynsmeddelande*, `6.1-99` *Övriga handlingar inspektion*. |
| `SelectedCounty` / `SelectedMunicipality` | codes | Optional geography filter. Not used. |
| `OnlyActive` | `true`/`false` | Only documents in ongoing cases. Not used (`false`). |
| `SelectedSortOrder` | `Dokumentdatum\|Desc`, `Dokumentdatum\|Asc`, `Diarienr\|Desc`, `Diarienr\|Asc` | Sort. Ingestion uses `Dokumentdatum|Desc`. |
| `p` | 1-based page number | Pagination. |

Example (the query the ingester issues):

```text
/om-oss/diarium-och-allmanna-handlingar/bestall-handlingar/?FromDate=2026-09-15&ToDate=2026-09-23&SelectedArendeProcess=6.1&SelectedHandlingType=6.1-23&SelectedSortOrder=Dokumentdatum%7CDesc&OnlyActive=false&p=2
```

## Pagination

* Fixed page size of **10** rows.
* Pagination links in the response carry the full filter query plus `p=N`,
  and requesting `p=N` directly with the same filters works without any
  session state (pages 1, 2 and 35 of the same query had no overlap).
* Total result count: `<span id="dd-pagination-result-total">347</span>`
  (formatted with thousands separators, e.g. `2 064 895` for the unfiltered
  diary).
* A page beyond the last one answers 200 with an empty result list, so
  "zero rows" is a reliable termination condition.

Volume: 347 `Inspektionsmeddelande` documents between 2026-09-15 and
2026-09-23 (9 days), 33 on 2026-09-22 alone: roughly 35–40 per weekday.

## Fields in a search result row

Each row is `<li class="document-list__item">` containing `<dt>`/`<dd>`
pairs. Observed labels and values (87 rows profiled from the filtered
query):

| Label | Example | Presence |
| --- | --- | --- |
| Handlingstyp (heading) | `Inspektionsmeddelande` | 100 % |
| Handlingsnummer | `2026/060943-2` (case number + running number; stable, unique) | 100 % |
| Handlingens datum | `<time datetime="2026-09-23">` | 100 % |
| Ärende (link to `Case/?id=2026/060943`) | `Inspektion inom Fortlöpande tillsyn - Unga i arbetslivet` | 100 % |
| Ärendets status | `Pågående` / `Avslutat` | 100 % |
| Företag/organisation (link to `Company/?orgnr=5594800418`) | `MITTEN MACK AB` | 98 % (2 of 87 rows had an empty block) |
| Organisationsnummer | `5594800418` (10 digits, no hyphen) | same as above |
| Handlingens ursprung | `Utgående` (all inspection notices) | 100 % |
| Ämnesområde | `Bedriva inspektion` | 100 % |
| Arbetsställe | `RUSTA HÖÖR` (workplace name; `Saknas` when unknown) | 100 % (value `Saknas` on the 2 rows without organisation) |
| Arbetsställenummer (CFAR) | `71466304` (8 digits) | 100 %, even on rows without organisation |
| Cart button `data-document-id` | `2026/060943-2` | 100 % |

Notes:

* The **organisation number is present** and machine-readable both as the
  `orgnr` query parameter of the company link and as text. Sole traders
  are registered with their personal identity number in this field (10
  digits, same format), which is personal data.
* The **CFAR (workplace identifier)** is present on every row. Workplace
  and legal organisation are distinct: e.g. organisation `RUSTA AB (PUBL)`
  with workplace `RUSTA HÖÖR`.
* The case title is the inspection campaign ("Inspektion inom …") or the
  triggering incident ("Olycka 20260907 Fysiskt våld", "Tillbud …"). It
  is the only content-bearing text available in the listing.
* Municipality/county are **not** in the search rows. They are on the
  case detail page.

## Fields on the case detail page

`GET /om-oss/diarium-och-allmanna-handlingar/bestall-handlingar/Case/?id=2026/015577`

Adds to the row data: Plats as county `STOCKHOLMS LÄN (01)` and
municipality `Norrtälje (0188)` (names with official codes), and the list
of all documents in the case with type, number, date and origin, paginated
10 per page (`&p=N`). Example case 2026/015577 held 19 documents including
`Faktaunderlag`, `Svar på kravskrivelse`, `Uppföljning av krav` and
`Beslut om att ärende avslutas (avslutsbrev)`, i.e. the case page exposes
the **escalation trail** of an inspection as metadata.

The ingester does not fetch case pages yet (one extra request per event);
the case URL is stored on every event so this can be added later.

`Company/?orgnr=…` lists every document of an organisation, same row
format.

## Identifying an Inspektionsmeddelande

Reliable and deterministic: filter on `SelectedHandlingType=6.1-23` and
verify the row heading equals `Inspektionsmeddelande`. All 87 profiled rows
matched. No fuzzy matching is needed.

What the document means (av.se, "Så går en inspektion till"): after an
inspection where deficiencies (*brister*) were found, the employer receives
a written notice describing the deficiencies and the applicable rules,
normally with a request to report back what has been or will be done. So
every `Inspektionsmeddelande` implies at least one identified deficiency at
the workplace, but the listing does not say which.

## Document content

* There is **no document URL** and no download. Each row has an "Lägg till
  i varukorg" (add to cart) button
  (`.../bestall-handling-varukorg/AddProductToBasket?productId=2026%2f060943-2`).
* Ordering: fill the cart, submit with an email or postal address. Orders
  are handled manually in arrival order, with a secrecy assessment and
  possible masking, then delivered by email (free) or post. The page
  currently warns of "longer waiting times than usual". The order itself
  becomes a public record.
* Some documents in ongoing cases are marked "Vi arbetar med ärendet och
  kan därför inte lämna ut handlingen just nu" (cannot be released while
  the case is ongoing). In the checked ongoing case that applied to
  `Registrerad kontroll`, not to the `Inspektionsmeddelande` itself.
* No other official endpoint exposes the text. Arbetsmiljöverket's
  statistics service is separate and does not contain case documents.

Conclusion: the *fact* that a workplace received an inspection notice, its
date, organisation, workplace and inspection campaign are available in
near real time. The *specific deficiency* is not, short of ordering each
document manually. Mass ordering was deliberately not automated.

## Limitations observed

* Page size is fixed at 10 and one page is ~310 KB, so 100 events cost ~10
  requests / 3 MB. The ingester waits `ARBETSMILJOVERKET_REQUEST_INTERVAL`
  between requests (default 1 s).
* HTML structure is the contract; a markup change breaks the parser. Raw
  row HTML is stored with every observation so records can be re-parsed.
* Documents dated today never appear (one-day lag).
* Rows without organisation exist (2 %); CFAR was still present.
* Sole-trader organisation numbers are personal identity numbers.

## Example identifiers (real)

```text
document 2026/060943-2  case 2026/060943  Inspektion inom Fortlöpande tillsyn - Unga i arbetslivet  org 5594800418  CFAR 71466304
document 2026/061028-2  case 2026/061028  Inspektion inom Fortlöpande tillsyn Fall från samma nivå   org 5562802115  CFAR 72896509
document 2026/058034-4  (no organisation shown, workplace "Saknas")                                     CFAR 73278392
```

## Smoke run 2026-09-24 (real data)

`ingest arbetsmiljoverket` over 2026-09-21..2026-09-23 against the live
service, then re-run over the same day:

| | |
| --- | --- |
| Pages fetched | 14 (10 rows each, ~0.3 s + 1 s interval per page) |
| Source rows observed | 137, all `Inspektionsmeddelande` |
| Parse failures | 0 |
| Canonical public events | 137 (136 distinct cases) |
| With valid organisation number | 132 (96 %); none were sole traders |
| With CFAR | 136 (99 %); one row printed `Saknas` in the CFAR field |
| Workplace name differs from organisation name | 33 (24 %) |
| Public-sector organisations (`2…` numbers) | 14 |
| Case status | 136 `Pågående`, 1 `Avslutat` |
| Re-run of 2026-09-23 | 70 already known, 0 inserted, 0 updated |

Most frequent case titles (campaigns): *Fortlöpande tillsyn - Riskmiljöer*
(20), *Olycka …* (11), *Myndighetsgemensamma kontroller* (11), *Unga i
arbetslivet* (11), *Kvarts och kemi inom markarbete och industri* (10),
*Belastningsergonomi i arbetslivet* (10), *Systematiken i byggprojektets
arbetsmiljöarbete* (7), *SAM i mindre företag* (6), *Arbete och säkerhet
intill väg* (6).

The commercial reading of these records is in
[arbetsmiljoverket-evaluation.md](arbetsmiljoverket-evaluation.md), with the
per-event sheet in `arbetsmiljoverket-evaluation-sheet.csv`.

## Manual smoke test

```bash
curl -s 'https://www.av.se/om-oss/diarium-och-allmanna-handlingar/bestall-handlingar/?FromDate=2026-09-22&ToDate=2026-09-22&SelectedArendeProcess=6.1&SelectedHandlingType=6.1-23&SelectedSortOrder=Dokumentdatum%7CDesc&OnlyActive=false&p=1' \
  | grep -c 'document-list__item'
```

Expected: `10` (or fewer on the last page).

## Enforcement and inspection-certificate document types (verified 2026-09-25)

Added for the deterministic-signal spike
(`docs/evaluations/arbetsmiljoverket-deterministic-signals.md`). Everything
here was read from the live diary with plain HTTP; the same search
mechanics as above apply (no session, 10 rows per page, `p=N`, inclusive
date window, stable `Handlingsnummer`).

### Subject areas (`SelectedArendeProcess`)

`2.2` Anskaffa varor och tjänster, `6.1` Bedriva inspektion, `7.1` Bedriva
marknadskontroll, `4.1` Föreskrifter, `6.3.2` Hantera Arbetsmiljöverkets
överklagande, **`6.4` Hantera avgiftsutdömande**, `8.1` Hantera tillstånd,
`6.3.1` Hantera överklaganden.

### Document types relevant to enforcement (`SelectedHandlingType`)

| Code | Handlingstyp | Documents 2025-09-25..2026-09-24 | What it is |
| --- | --- | ---: | --- |
| `6.4-1` | Avgiftsföreläggande | 1 385 | The sanction-fee order sent to the employer. One case per deficiency; the case title names the deficiency ("Sanktionsavgift – …"). |
| `6.4-2` | Svar på avgiftsföreläggande | 1 111 | Employer's reply (accept / contest). |
| `6.4-3` | Vidarebefordran av godkänt avgiftsföreläggande | 666 | Accepted order forwarded for collection. |
| `6.4-4` | Ansökan om påförande av sanktionsavgift | 656 | Contested orders taken to the administrative court. |
| `6.4-99` | Övriga handlingar - Avgiftsutdömande | 790 | Correspondence in fee cases. |
| `6.1-49` | **Intyg återkommande besiktning** | 1 803 | Incoming certificate from an accredited inspection body; case title "Återkommande besiktning - <device type>". |
| `6.1-24` | Tillsynsmeddelande | 6 175 | Outgoing supervision letter; in certificate cases AV's demand to the employer. |
| `6.1-55` | Beslut om slutligt omedelbart förbud | 887 | Immediate prohibition; title "Inspektion - Omedelbart förbud <date> - <hazard> - <address>". |
| `6.1-21` / `6.1-22` | Tillfälligt omedelbart förbud / upphävande | 14 / 18 | Rare. |
| `6.1-56` | Beslut om upphävande av förbud | 239 | Prohibition lifted. |
| `6.1-33` / `6.1-34` | Beslut om förbud med / utan vite | 160 / 1 | Titles are campaign or incident names, not the deficiency. |
| `6.1-35` / `6.1-36` | Beslut om föreläggande med / utan vite | 264 / 2 | Same. |
| `6.1-32` | Underrättelse om föreläggande/förbud | 362 | Advance notice of an order. |
| `6.1-17` | Registrerad kontroll | 30 269 | Internal record of a performed inspection; no content. |
| `6.1-14` | Anmälan, rapport från yrkeshygienisk mätning | 101 | Incoming exposure-measurement reports ("Yrkeshygienisk mätning - <agent>"). |
| `6.1-15` | Anmälan, rapport från medicinsk kontroll | 2 | Incoming medical-surveillance reports. |
| `6.1-20` | Dokumenterad bedömning inklusive brister och krav | 0 | Not used in the period. |

Calendar-year counts for `6.4-1`: 1 641 (2024), 1 761 (2025). For `6.1-49`:
1 706 (2025).

### Sanction-fee cases (`6.4`)

* Sanction cases are **separate cases** from the inspection that found the
  deficiency. The `6.4` case holds only the fee documents; the preceding
  inspection lives in a `6.1` case for the same organisation, usually with
  a `Faktaunderlag` dated the same day as the order.
* Rows carry organisation number (81 %), CFAR (83 %) and workplace name
  (78 %). Posting-of-workers cases (`utstationering`) are the main gap:
  foreign employers have no Swedish identifiers.
* The case page adds county and municipality; the fee documents in
  ongoing cases are often marked as not releasable.

### Inspection certificates (`6.1-49`)

* AFS 2023:11 13 kap. 13 §: "Om kontrollorganet bedömer att anordningen
  inte erbjuder betryggande säkerhet, ska de snarast meddela detta till
  Arbetsmiljöverket." Bilaga (arbetskorgar) 3.3 and 10 kap. 41 § (boilers
  that may not be operated) carry the same duty. An incoming `Intyg
  återkommande besiktning` is therefore a device that **failed** its
  recurring inspection. A handful of rows are titled "För kännedom -
  godkänd besiktning" and are the exception.
* Observed case pattern (sampled case pages): Intyg (day 0) →
  Tillsynsmeddelande to the employer (0–22 days) → Påminnelse → Svar på
  kravskrivelse → Beslut om att ärende avslutas, sometimes with a second
  Intyg when the device passes re-inspection.
* Rows carry organisation number (97 %) and CFAR (93 %); the workplace
  differs from the legal entity in about a third of cases.

### Lookups by identifier

`SearchText` matches organisation numbers **and CFAR numbers**, so the
history of a workplace can be listed with `SearchText=<CFAR>` plus a date
window without the (larger) `Company/?orgnr=` page. `Company/?orgnr=…&p=N`
returned HTTP 500 for one organisation during the spike; the search route
did not.
