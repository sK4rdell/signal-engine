# Source: Klimatklivet approved applications (Naturvårdsverket)

Research note for the second Signal Engine source. Everything marked
VERIFIED was checked against the live official source or the regulation
text on 2026-09-24. Everything marked HYPOTHESIS is our reading, not the
source's.

## Verdict

**Technically viable** (VERIFIED): one public Excel file lists every
approved application since 2015 with a stable case number, decision date,
applicant name, category, grant amount and, for non-charging rows,
municipality and county. **Commercially late by construction** (VERIFIED):
the file is republished about twice a year, so a decision becomes visible
between one week and six months after it was made. **Organisation
identity is by name only** (VERIFIED): no organisation number is
published.

## Public entry point

```text
https://www.naturvardsverket.se/amnesomraden/klimatomstallningen/klimatklivet/sa-fungerar-klimatklivet/resultat--hur-har-det-gatt-for-klimatklivet/
```

"Resultat – hur har det gått för Klimatklivet?" links one workbook:

```text
Beviljade ansökningar till Klimatklivet till och med 30 juni 2026 (xlsx)
/49f446/globalassets/amnen/klimatomstallning/klimatklivet/dokument/beviljade-ansokningar-till-klimatklivet-260701.xlsx
```

The file URL changes with every publication (path prefix and date in the
name), so the ingester discovers the current link on the results page
(first `.xlsx` link whose text contains "Beviljade ansökningar") and
records the link text's cut-off date ("till och med 30 juni 2026").
`robots.txt` disallows nothing. No API and no CSV/JSON alternative was
found on naturvardsverket.se or dataportal.se.

The download is served by a CDN with `ETag` and `Last-Modified`
(2026-07-07 for the current file), 6.9 MB, ~0.5 s. The ingester sends
`If-None-Match` with the last recorded ETag and skips everything on 304.

## Update frequency (VERIFIED via the Wayback Machine)

| Snapshot of results page | File linked (cut-off) |
| --- | --- |
| 2025-03-31 | till och med 31 december 2024 |
| 2025-08-04, 2025-12-14 | till och med 30 juni 2025 |
| 2026-03-08, 2026-06-09 | till och med 31 december 2025 |
| 2026-09-24 (live) | till och med 30 juni 2026 (published 2026-07-07) |

So: semi-annual, published about one week after the half-year ends.
Decision-to-publication latency is 1 week to 6 months, 3 months on
average. Decisions are taken continuously (2,263 distinct decision dates;
the 2026 application rounds are listed on
`/klimatklivet/klimatklivets-ansokningsomgangar/`).

## File structure (VERIFIED)

Three sheets, *Per åtgärdskategori*, *Per län*, *Per kommun*, each holding
the same 33,311 rows sorted differently (identical case-number sets). The
parser reads *Per åtgärdskategori*, falling back to the first sheet, and
resolves columns by header prefix, not position.

| Header | Content | Presence |
| --- | --- | --- |
| Ärendenummer | `NV-25-043146`, `KKL-02186-2017`, `NV-05928-15` (21 rows have a trailing space) | 100 %, unique |
| Organisationsnamn | applicant name as written; sole traders appear under a personal name | 100 % (25,530 distinct) |
| Rubrik | short project title written by the applicant ("Energikonvertering industri", "Biogasproduktion i Tibro.") | 100 % (4,861 distinct) |
| Åtgärdskategori | 11 values: Laddstation 30,339; Energikonvertering 1,907; Fordon 250; Transport 198; Produktion biogas 169; Energieffektivisering 123; Avfall 93; Infrastruktur 74; Informationsinsatser 58; Gasutsläpp 54; Övrigt 46 | 100 % |
| Län / Kommun | county and municipality names | 6,622 / 6,568 rows; **empty for all charging-station rows** |
| Senast tillgängligt beviljat stödbelopp (kr) | latest known granted amount, integer SEK | 33,310 (one row empty) |
| Summering Antal laddpunkter | number of charging points | charging rows |
| Laddinfra Publik/Icke-publik | Publik 2,194 / Icke-publik 28,143 | charging rows |
| Bifallsdatum | decision date (Excel date) | 100 %, 2015-11-04 .. 2026-06-30 |
| Senast tillgängligt slutdatum eller slutligt beslutsdatum | latest end date or final decision date | 6,434 rows (non-charging) |
| Status | Pågående åtgärd 2,438 / Slutförd åtgärd 30,873 | 100 % |
| Förordning inom Klimatklivet | 2015:517 (local climate investments) 6,622 / 2019:525 (charging points, "Ladda bilen") 26,689 | 100 % |

Not in the file: organisation number, total investment cost, project
description beyond the title, expected emission reduction, application
date, start date, decision reference/URL, rejected applications. Amounts
are "latest available", i.e. amended decisions overwrite the row in place;
whether revoked decisions are removed cannot be seen from one file
version.

Volume: 2,972 non-charging rows since 2015; 345 non-charging decisions
in 2025-01-01..2026-06-30, 146 of them in H1 2026; 2,045 charging rows in
H1 2026.

## Stable identity (VERIFIED)

`Ärendenummer` is unique across the file and stable across publications
(the same numbers appear in archived versions). The ingester uses the
trimmed case number as `source_event_id`. The dataset file itself is
recorded as one observation per published version under the record id
`dataset:beviljade-ansokningar` with URL, label, cut-off date, ETag,
Last-Modified, SHA-256, size, sheet and row count as payload, which is
the provenance of every row observed from it.

## Timing and procurement mechanics

Sources: förordning (2015:517) om stöd till lokala klimatinvesteringar
(riksdagen.se), Naturvårdsverket's pages *Kan mitt projekt få stöd?*,
*Vilka kostnader kan jag få stöd för?*, *Förbered din ansökan …*,
*Rapportera hur det går*, *Laddning för allmänheten – anbudsprocess*, and
the GBER definition (regulation (EU) 651/2014, article 2(23) and 6).

VERIFIED FACTS:

1. **No start before the decision.** 7 § 5 of the ordinance: support may
   not be given to a measure "som har påbörjats innan beslut i frågan om
   stöd har fattats". Naturvårdsverket: "Åtgärden får påbörjas först efter
   att beslut har fattats."
2. **Ordering equipment counts as starting.** A measure is started when
   "det fysiska arbetet med åtgärden har startat" or "material, utrustning
   eller annat som är nödvändigt för åtgärden har beställts". GBER 2(23):
   start of works is the earlier of construction start and "det första
   bindande åtagandet att beställa utrustning eller ett annat åtagande som
   gör investeringen oåterkallelig".
3. **Allowed before applying:** feasibility studies and pre-engineering
   (förstudie/förprojektering), site search, permit applications, project
   management, a non-binding grid connection pre-notification.
4. **Quotations are required in the application.** "Bifoga därför
   offerter som styrker den förväntade investeringskostnaden" (both the
   energy-conversion and the general application pages). The application
   must also contain a cost list, an investment calculation, a timetable
   and start/end dates (12 §). Nothing says the quotation is binding or
   that the quoting supplier must be used.
5. **Eligible costs start at the decision date**: "ha uppstått efter
   datumet för ditt beslut om stöd". Procurement costs (upphandling of
   material, contractors, equipment) are eligible only "mellan fattat
   beslut om stöd och angivet slutdatum". Design/engineering
   (projektering) is eligible only after the decision; pre-engineering is
   not.
6. **Changes must be reported**, including a materially different
   investment cost or changed implementation; approval is not guaranteed
   and support can be withheld or reclaimed (22–24 §§). Nothing forbids
   changing supplier or equipment as such.
7. **Payment**: at most 75 % before completion, in instalments close to
   when costs arise (19 §); progress reports every six months.
8. **Public bodies** follow the procurement acts; a contract award would
   be a binding commitment, so it cannot precede the decision either. For
   the public-charging track the applicant must start within six months
   of the decision.
9. Between application and decision the applicant may continue
   non-binding preparation; starting "på egen risk" would forfeit support,
   so it is not a rational option for a grant-dependent project
   (Klimatklivet only funds projects that "inte kan göras utan stöd").

COMMERCIAL HYPOTHESIS:

* At the decision date the structurally expected state is **quoted but
  not ordered**: one or more suppliers have quoted the main equipment,
  nothing is contracted. Whether the quoting supplier wins the order is
  what the manual validation must measure; for standardised equipment the
  quote often is the de-facto selection, for multi-component projects the
  installation, electrical, automation and civil parts are typically
  procured after the decision.
* Because the file is published up to six months after the decision, and
  eligible spending starts at the decision, a meaningful share of the
  main-equipment orders will already have been placed when we first see
  the row. The signal is therefore late for main equipment and possibly
  still open for later components and for projects with long lead times
  (biogas plants, industrial conversions with 2–4 year end dates).
* The category and title tell what is being built at family level, not
  which equipment; investment size is only visible through the grant
  amount (20–70 % of the investment for companies, 50 % max for others).

## Limitations and caveats

* No organisation number: organisation identity is the applicant name.
  Sole traders are published under personal names (personal data).
* Charging-station rows (91 % of the file) carry no location.
* Semi-annual publication: not a near-real-time signal.
* The file replaces amounts in place; history exists only through our
  observations.
* A markup change on the results page or a header rename in the workbook
  breaks discovery or parsing; both fail loudly.

## Smoke run 2026-09-24 (real data)

`ingest klimatklivet --from 2025-01-01` against the live site, then the
same command again:

| | |
| --- | --- |
| File | beviljade-ansokningar-till-klimatklivet-260701.xlsx, 6,916,337 bytes, cut-off 2026-06-30 |
| Rows parsed / malformed | 33,311 / 0 |
| Rows with decision ≥ 2025-01-01 | 7,074 (6,729 charging, 345 other) |
| Observations inserted | 7,075 (7,074 rows + 1 dataset version) |
| Events inserted | 7,074 |
| Wall time | 6.9 s including download |
| Second run | HTTP 304, nothing downloaded or processed, dataset observation re-stamped |

The commercial reading of these rows is in
[docs/evaluations/klimatklivet.md](../evaluations/klimatklivet.md).

## Manual smoke test

```bash
curl -s 'https://www.naturvardsverket.se/amnesomraden/klimatomstallningen/klimatklivet/sa-fungerar-klimatklivet/resultat--hur-har-det-gatt-for-klimatklivet/' \
  | grep -o 'href="[^"]*beviljade-ansokningar[^"]*\.xlsx"'
```

Expected: one link to the current workbook.
