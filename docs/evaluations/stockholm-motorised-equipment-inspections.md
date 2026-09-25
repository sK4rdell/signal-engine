# Evaluation: failed inspections of lifts and other motorised equipment in Stockholm

Discovery spike, 2026-09-25. Question: can Stockholm's public building
and planning service give a machine-readable feed of failed recurring
inspections of lifts, powered doors and gates, escalators and other
motorised equipment, with enough metadata for a deterministic public
event, and does the pattern exist in other municipalities?

Everything marked VERIFIED was read on 2026-09-25 with plain HTTP from
the public service (no login, no browser automation, no document
ordered) or quoted from the regulation text; everything marked INFERENCE
is our reading. The per-event sheet is
`stockholm-motorised-equipment-inspections.csv`. Research scripts lived
in a scratch directory and are not part of the repository.

## 1. Recommendation: CONTINUE_WITH_LIMITATIONS (implement Stockholm next as a single-municipality source)

Stockholm alone meets the quality bar set by the Arbetsmiljöverket
recurring-inspection signal, and in some respects exceeds it: one plain
HTTP request lists every supervision case, each case page names the
device, the property, the address and the district, and the document
description says in so many words that the recurring inspection was not
approved ("Intyg återkommande besiktning, ej godkänt, L2992070"). The
rule that selects those documents matched 2,096 certificates in 13
months with no false positive in a 40-row manual sample, the source
shows the later approved certificate and the city's use-prohibition
reminder in the same case, and the volume is about 1,950 failed
certificates a year on 1,700 properties, 91 % of them lifts.

The limitations are real and are why this is not a plain CONTINUE:
the source identifies the property, not the owner, so a deterministic
owner join (property designation → registered owner) is a later step;
incumbent risk for lifts is high because nearly every lift has a
service contract, so the open remediation window does not mean an open
supplier selection; and the pattern does not travel: the large cities
scanned expose building cases only to applicants, and the municipalities
with public web diaries publish other committees' cases or different
wording. Stockholm is therefore a good next production source on its
own merits, not the first of a municipal roll-out.

## 2. Source access (VERIFIED)

Public entry point: **Bygg- och plantjänsten**, Stockholms stad,
`https://etjanster.stockholm.se/Byggochplantjansten/` ("Ärenden -
ritningar och handlingar" at `/arendeochhandlingar`). ASP.NET MVC on IIS
(`X-Powered-By: ASP.NET`, `Cache-Control: private`, session and
load-balancer cookies set but not required). No login for case metadata;
login is only needed to view drawing thumbnails. No `robots.txt` (404),
no terms found on the service. Access gate: **GOOD**.

### Case search

`GET /Byggochplantjansten/arendeochhandlingar?<parameters>`, parameters
without the `SearchParameters.` prefix the form uses in its markup (the
page's JavaScript strips it; with the prefix the server answers "Ange
minst ett sökkriterium"):

| Parameter | Meaning | Values used |
| --- | --- | --- |
| `CaseTypeRecNo` | Ärendegrupp | `200001` = *Funktionskontroll, Tillstånd* (others: `62001` Byggärende, `62000` Planärende, `100005`/`100002` lantmäteri, `300000` adresser) |
| `JournalPlanCode` | Diarieplansbeteckning within the group | **`7.2` = *Hantera tillsyn av hissar och andra motordrivna anordningar*** (cases from 2024-01-01); `587` = *Avvikelse från byggregler (hissar mm), funktionskontr av hissar mm* (cases up to 2023-12-31) |
| `CaseStartDateFrom` / `CaseStartDateTo` | Ärendestart, inclusive `YYYY-MM-DD` | |
| `DecisionDateFrom` / `DecisionDateTo`, `DecisionType` | decision filters | not used |
| `Description` | free text in the case title ("Ord i ärendemening") | e.g. `förnyad besiktning` |
| `RealEstateName` | property designation ("Aida 3") | |
| `District` | stadsdel (118 values) | |
| `DiaryNumberPlanNumber` | diarienummer ("2026-15984") | |

The response is server-rendered HTML that **embeds the complete result
set as JSON** (`var CaseSearchResultsViewModel = {…}` with
`OtherCases.CaseSearchDetails[]`, and the same rows again in the
`listComponent.createInstance(...)` call). Pagination (50 rows per page)
is done client-side by the page's JavaScript over the embedded array, so
one request returns every matching case: 665 cases for June–September
2026 came back in one 1.45 MB response, 1,447 cases for 2024 in one
response. Each case row carries `RecNo` (stable numeric case id, used in
the case URL), `Name` (diarienummer), `Description` (case title),
`StartDate`, `RealEstateName`, `RealEstateAddress`, `CaseTypeCode`,
`IsEarchive`. Search rows carry no documents and no district.

### Case page

`GET /Byggochplantjansten/arende/arende/<RecNo>?dataSource=Active`
(`Archived` for e-archived cases; none of the 7.2 cases were archived).
~25 KB, ~0.3 s. Server-rendered: diarienummer, ärendegrupp,
diarieplansbeteckning, fastighetsbeteckning, adress, **stadsdel**,
ärendestart, **ärendeavslut** (empty while open), handläggare,
ärendemening, and the document list, again embedded as JSON
(`documentListComponent = listComponent.createInstance('documentList', …,
[{title, category, date, fileName}, …])`). Document rows have a
description (`title`), a type (`category`: "Skrivelse In", "Skrivelse
Ut", "Skrivelse Sbk"), a timestamp, and **no document identifier and no
file** (`fileName: null`; files require login and are drawings only).

### Practical cadence

One search per window plus one case-page request per case. We used a
one-second pause; no rate limiting or blocking was seen over ~2,300
requests. History: the 7.2/587 case series is searchable back to 2015
(1,585 cases in 2019, 1,522 in 2022, 1,447 in 2024, 1,247 in 2025), all
with `dataSource=Active`.

## 3. Regulatory basis (VERIFIED)

Plan- och byggförordningen (2011:338) 5 kap. 11 §: "Den som har utfört
besiktningen ska 1. utfärda ett protokoll om att besiktningen har gjorts
och vad den omfattat, 2. i protokollet ange om anordningen har sådana
brister som avses i 10 kap. 20 § andra stycket 1 eller 2 och vilka
bristerna i så fall är, och 3. lämna ett exemplar av protokollet till
den som äger eller annars ansvarar för anordningen. Om anordningen har
sådana brister som avses i 10 kap. 20 § andra stycket 1 ska den som har
utfört besiktningen omedelbart underrätta den som äger eller annars
ansvarar för anordningen om detta samt skicka ett exemplar av
protokollet till byggnadsnämnden." 5 kap. 14 §: a device may not be
used unless the owner can show, with such a protocol, that it meets the
safety requirements; 9 kap. 5 §: the byggsanktionsavgift for using a
device in breach of 5 kap. 12–15 §§ is 2 prisbasbelopp. 8 kap. 6 §: the
building committee shall order the owner to have the device controlled
when needed. Stockholm's own page ("Besiktning av hiss") states that a
device "som underkänts vid besiktning får inte användas" and that the
owner must "åtgärda omgående", have it re-inspected and send the
approved certificate to stadsbyggnadskontoret.

Boverket's definition of motordrivna anordningar (PBL kunskapsbanken):
motor-driven lifts for persons or goods (including platform and stair
lifts), other motor-driven devices for transport of persons or goods
such as escalators, moving walks and cableways, and motor-driven doors,
gates, grilles and similar devices for passage of persons or vehicles
such as garage doors and roller grilles.

INFERENCE: an incoming certificate registered by stadsbyggnadskontoret
with the wording "ej godkänt" is therefore the notification the
regulation requires when the device has deficiencies of immediate
significance for safety, and the device is under a use prohibition until
a new certificate is approved.

## 4. Dataset (VERIFIED)

Population: every case in class 7.2 *Hantera tillsyn av hissar och andra
motordrivna anordningar* started 2025-09-01 to 2026-09-25 (three search
windows), and every case's page. No document was ordered.

| | |
| --- | --- |
| Cases returned by the three searches | 2,193 |
| Case pages read (one case answered with a connection reset and was skipped) | 2,192 |
| Documents listed on those pages | 4,474 |
| Case titles | "Anmaning att låta åtgärda hiss(ar) samt utföra förnyad besiktning" 2,010; "… port(ar) …" 149; "… rulltrappa/rulltrappor …" 22; other 11 |
| Cases with at least one failed certificate (DETERMINISTIC_FAILURE) | 1,984 (90.5 %) |
| Cases opened by an approved certificate only (NOT_A_FAILURE; same case title) | 173 (7.9 %) |
| Cases with only a partly approved certificate, a reminder, a complaint or notes (UNKNOWN) | 35 (1.6 %) |
| **Failed-certificate documents (candidate events)** | **2,096** |
| Approved certificates ("godkänt") and approved re-inspections ("ombesiktning, godkänt") | 934 + 97 |
| "Påminnelse om användningsförbud" (city's reminder of the use prohibition) | 1,142 |
| Partly approved certificates ("delvis godkänt") | 49 |

The case title is a fixed template used for every incoming certificate
of a property, approved or not; it is **not** the failure indicator. The
document description is.

## 5. Vocabulary and the deterministic rule (VERIFIED)

Document descriptions of the incoming certificates ("Skrivelse In")
follow a small template set (numbers masked):

```text
Intyg återkommande besiktning, ej godkänt, <id>[, <id> …]          2,096  ← failure
Intyg återkommande besiktning, ej godkänt, <n> st, <id>, <id> …            (same, multi-device form)
Intyg återkommande besiktning, godkänt, <id>                          934  approved
Intyg ombesiktning, godkänt, <id>                                      97  approved re-inspection
Intyg återkommande besiktning, delvis godkänt, <id>                    49  partly approved: ambiguous, excluded
Godkänt hissintyg <id>                                                  5  approved
Påminnelse om användningsförbud                                     1,142  city's reminder, outgoing
Påminnelse om återkommande besiktning                                  13  reminder, outgoing
Besiktningsanteckningar, Klagomål på hiss, Följebrev, Skrivelse …      ~120 other
```

A few certificates add "(Besiktningsdatum 2026-09-07)". Three
certificates read "ej godkänt," with no identifier.

**Deterministic rule** (VERIFIED against all 4,474 documents):

```text
case class code = 7.2 (Funktionskontroll, Tillstånd)
AND document category = "Skrivelse In"
AND document description matches ^Intyg (återkommande |första |revisions)?besiktning, ej godkänt
      (also ^Intyg ombesiktning, ej godkänt — not observed in the period)
→ DETERMINISTIC_FAILURE
```

"delvis godkänt" → SPECIFIC_BUT_AMBIGUOUS (49); "godkänt", reminders,
notes, complaints → NOT_A_FAILURE; cases without any certificate →
UNKNOWN. Precision: 40 of 40 randomly sampled matched descriptions are
unambiguous failure certificates; the only near-misses in the corpus are
the "delvis godkänt" rows, which the rule excludes by construction. The
rule needs no document and no interpretation of free text beyond the
template.

## 6. Metadata coverage (VERIFIED)

Over the 2,096 failed-certificate documents (per case for the
case-level fields):

| Field | Coverage | Note |
| --- | --- | --- |
| Property designation (fastighetsbeteckning) | 100 % | search row and case page |
| Street address | 98 % (43 documents in cases where the source prints 'Information saknas för "Adress"') | search row and case page; several addresses for large properties |
| District (stadsdel) | 100 % | case page only |
| Case start date and document date | 100 % | the failed certificate is the case's first document in 1,962 of 1,984 cases and dated the case start day in 1,661 |
| Asset identifier in the description | 99.5 % (2,086 of 2,096 documents; 1,975 of 1,984 cases) | one identifier in 1,710 documents, two in 236, three in 82, four or more in 58 |
| Asset type | from the case title: hiss 91 %, port 7 %, rulltrappa 1 % | never in the document description |
| Inspection result wording | 100 % | "ej godkänt" |

Identifier formats: `L` + 7 digits (1,134 cases), `S` + 6 digits (612),
plain 5 digits (492), `D` + 7 digits (357). INFERENCE: the prefix is the
inspection body's numbering (D matches DEKRA's series; L and S are two
other bodies; the plain numbers are older city lift numbers). The
identifier is stable: the approved certificate that follows a failure
carries the same identifier in 307 of the 307 Sep–Dec 2025 cases where
both exist, 115 identifiers appear in more than one failure, and 29 in
more than one case. Multiple devices per document are common (18 % of
documents), so one failed document can mean several assets; the
identifiers are listed individually, so per-asset events are derivable.

## 7. Volume (VERIFIED)

Failed-certificate documents by month (document date):

| Month | Events | Month | Events |
| --- | ---: | --- | ---: |
| 2025-09 | 185 | 2026-04 | 164 |
| 2025-10 | 103 | 2026-05 | 162 |
| 2025-11 | 150 | 2026-06 | 191 |
| 2025-12 | 101 | 2026-07 | 159 |
| 2026-01 | 139 | 2026-08 | 150 |
| 2026-02 | 210 | 2026-09 (to the 25th) | 149 |
| 2026-03 | 233 | | |

Twelve full months (Sep 2025 – Aug 2026): **1,947 failed certificates**,
about 160 a month, on about 150 distinct properties a month. Earlier
years are consistent: 1,585 cases (2019), 1,522 (2022), 1,447 (2024),
1,247 (2025) under the class codes 587/7.2, of which about nine in ten
are failures.

| Asset type (case title) | Failed documents | Share | Distinct asset identifiers |
| --- | ---: | ---: | ---: |
| Lift | 1,916 | 91.4 % | 2,344 |
| Powered door or gate | 148 | 7.1 % | 201 |
| Escalator or moving walk | 24 | 1.1 % | 23 |
| Not stated | 8 | 0.4 % | 12 |

Unique properties over the 13 months: 1,709 (258 of them with more
than one failed document). Unique asset identifiers: 2,580.

## 8. Lifecycle: failed → approved (VERIFIED)

The case page shows what happened after the failure. Chronology of a
typical case:

```text
Intyg återkommande besiktning, ej godkänt, L1142264      2025-09-02  (case opened the same day)
Påminnelse om användningsförbud                          ~ +6 months  (outgoing)
Intyg återkommande besiktning, godkänt, L1142264         2025-12-12  (+101 d)
Ärendeavslut                                             2026-05-19
```

Cohort of the 520 failed cases started September–December 2025, read
9–12 months later (VERIFIED):

| | Cases | Share |
| --- | ---: | ---: |
| Later approved certificate in the same case | 307 | 59 % |
| Approved within 30 days | 44 | 8 % |
| Approved in 31–90 days | 52 | 10 % |
| Approved in 91–180 days | 32 | 6 % |
| Approved after more than 180 days | 179 | 34 % |
| City sent "Påminnelse om användningsförbud" | 445 | 86 % |
| Case closed | 320 | 62 % |
| Still open after 9–12 months | 200 | 38 % |
| More than one failed certificate in the case | 38 | 7 % |

Days from failure to the approved certificate: median 238 (p25 80, p75
257); to the city's reminder: median 226; to closure: median 251. The
distribution is bimodal: a fifth of owners re-inspect within three
months, most of the rest only after the city's reminder about six months
later, and two in five have no approved certificate on file after nine
months or more. Tracking by asset identifier across cases gives the same
picture (952 assets with a later approved certificate, median 175 days).
Approval is taken only from an approved certificate, never from closure
(the bulk closure date 2026-05-19 on many cases is administrative).

The approved certificate is filed in the failure's case in most
instances, but 173 cases in the period were opened by an approved
certificate alone, i.e. the re-inspection of a failure filed elsewhere;
the asset identifier links them.

## 9. Timing (INFERENCE on VERIFIED chronology)

When the failed certificate appears in the diary the device is under a
use prohibition and, in the large majority of cases, no approved
certificate arrives for months. The remediation window is therefore
open at publication: **TIMELY** with respect to remediation (EARLY in the
sense that the city's own follow-up comes half a year later). What the
window says about supplier selection is a different matter: the owner
received the certificate from the inspection body on inspection day,
usually has a lift service contract, and the deficiencies are typically
handled by that incumbent. The diary cannot show whether a supplier was
already engaged; that remains a commercial-validation question, exactly
as for the Arbetsmiljöverket certificate feed.

## 10. Buyer identification (VERIFIED source fields, INFERENCE on enrichment)

The source identifies the **property**, not the legal owner:

| Field | Present | Example |
| --- | --- | --- |
| Fastighetsbeteckning (property designation, Stockholm) | 100 % of cases | `Kronkvarnen 39` |
| Street address | see coverage table (section 6) | `Artillerigatan 48` |
| Stadsdel (district) | 100 % on the case page | `Östermalm` |
| Legal entity / organisation number / contact person | **never** (handläggare is the city's officer, "Ej utsedd" while unassigned) | – |

The certificate's sender (the inspection body) and the addressee (the
owner) are not shown; the document type is only "Skrivelse In".

INFERENCE: the deterministic join to a buyer is *property designation
within Stockholms kommun* → registered owner (lagfaren ägare, or
tomträttshavare where the city owns the land) in Lantmäteriet's
fastighetsregister. The key we hold is exactly what that register uses:
municipality (Stockholm, kommunkod 0180) + trakt + block:enhet
("Kronkvarnen 39"). Owner data is licensed, not open; a later enrichment
would be one lookup per property with a deterministic answer (an
organisation number or, for private persons, a natural person to be
dropped). Housing cooperatives (bostadsrättsföreningar) and municipal
housing companies are legal entities with organisation numbers, so the
join works for the bulk of Stockholm's lift stock. No owner source was
touched in this spike.

## 11. Supplier market (INFERENCE)

| Asset category | Corrective supplier (the purchase) | Re-inspection (separate) | Likely order | Incumbent risk |
| --- | --- | --- | --- | --- |
| Lift (hiss) | lift service and repair companies: the manufacturers' service arms (KONE, Otis, Schindler, TK Elevator) and independent lift-service firms; Hissförbundet's members are stated to cover over 80 % of the lift and escalator market | the accredited inspection bodies that issue the certificates (DEKRA, Kiwa and a few others; the identifier prefixes L/S/D in the certificates appear to be body-specific) | remedy of the listed deficiencies, from a service call to component replacement, then a new inspection | **HIGH**: nearly every lift in use has a service contract, and the inspection body typically lists deficiencies the incumbent service company is expected to fix; the open question is how often owners switch after a failed inspection |
| Powered door or gate (port) | industrial and garage door service firms (national chains and local installers) | same bodies | repair of safety edges, photocells, drives; re-inspection | MEDIUM: service contracts are less universal than for lifts |
| Escalator / moving walk (rulltrappa) | escalator service (largely the same manufacturers) | same bodies | repair | HIGH |

Supplier population (INFERENCE, not counted): lift service TENS of firms
of any size nationally with a long tail of small independents (Hissförbundet
membership is in the tens); door service HUNDREDS; accredited inspection
bodies for lifts a handful. Sales model: regional (Stockholm-based
service branches and independents). Order value: from a few thousand kr
for a minor deficiency to hundreds of thousands for drive or safety
component replacement; the sanction for using a failed device is 2
prisbasbelopp, which makes prompt remediation the norm.

What the source establishes is the failed inspection and the open
remediation window; it does not establish that supplier selection is
open, and for lifts the prior is that it usually is not.

## 12. Geography and commercial density (VERIFIED counts, INFERENCE)

Districts of the 2,096 failed documents: Södermalm 299, Vasastaden 221,
Norrmalm 209, Östermalm 186, Kungsholmen 119, Ladugårdsgärdet 70, Södra
Hammarbyhamnen 46, Kista 43, Liljeholmen 34, Tensta 32, the remaining
~100 districts 837. About 150 distinct properties a month across the
municipality, 15 % of properties recurring within the period.

INFERENCE on density: a lift-service company working Stockholm sees on
the order of 150 failed lifts a month here, which is a usable lead
volume for a regional player; ownership is fragmented (inner-city
housing cooperatives and private landlords dominate the district
counts, with the municipal housing companies in the outer districts),
so the events are not concentrated on a handful of owners. Ownership
itself is not in the source (section 10).

## 13. Municipal portability (VERIFIED probes, INFERENCE on families)

Probed on 2026-09-25 with plain HTTP (one to three requests per site).
"Lift cases discoverable" means a public, login-free search returned
case or document titles about lift inspections.

| Municipality | Public service / system | URL pattern | Access | Search | Lift cases discoverable | Classification |
| --- | --- | --- | --- | --- | --- | --- |
| Stockholm | Bygg- och plantjänsten (bespoke ASP.NET) | `etjanster.stockholm.se/Byggochplantjansten/arendeochhandlingar?…` | HTML with embedded JSON, no login | class code, dates, property, district, free text, diary number | **yes**, with the certificate result in the document description | reference system |
| Göteborg | "Följ mitt byggärende" and other e-services (`etjanst-bygglov.goteborg.se/GTB_*`); no common diary, documents by request | – | e-legitimation for cases; requests by e-mail/e-service | applicant-only | no | PUBLIC_MANUAL_ONLY |
| Malmö | Självservice "Bygglov och anmälan – Se dina ärenden" (`sjalvservice.malmo.se/oversikt/overview/1123`); digital drawing archive for closed cases | – | e-legitimation for cases | applicant-only | no | PUBLIC_MANUAL_ONLY |
| Uppsala | "Webdiary" `diarium5.uppsala.se` (municipality-wide web diary) | – | host did not resolve on 2026-09-25; the city's page says the web diary is closed for technical problems | – | unknown | UNKNOWN (public diary exists in principle) |
| Linköping | "Titta på mitt byggärende" (`eservice.linkoping.se/BYGGMINA`); the city states bygg- och miljönämnden's documents are not published in the external diary search | – | login | applicant-only | no | NO_PUBLIC_ACCESS_FOUND |
| Örebro | "Mina byggärenden" (`service.orebro.se/O531`) | – | e-legitimation | applicant-only | no | NO_PUBLIC_ACCESS_FOUND |
| Västerås | "Mina byggärenden" (`minasidor.vasteras.se`); diary by contacting each förvaltning | – | e-legitimation / manual | – | no | PUBLIC_MANUAL_ONLY |
| Jönköping | e-services for applicants; bygglovsarkiv order service | – | e-legitimation / order | – | no | PUBLIC_MANUAL_ONLY |
| Helsingborg | "Min sida"; Bygglovsarkivet (drawings only, from 2003) | – | e-legitimation / archive | drawings only | no | PUBLIC_MANUAL_ONLY |
| Norrköping | "Söka bygglov och andra åtgärder" (`minasidor.norrkoping.se/bygglov`) | – | login | applicant-only | no | NO_PUBLIC_ACCESS_FOUND |
| Umeå | **Public Journal / Public 360** (Tietoevry) `umea.opengov.360online.com`; htmx `POST /Search/1` with `Query`, `ShowCases`, `ShowDocs`, `Published`, department and journal-unit filters; `CaseDetails?Id=…` | HTML fragments, no login | free text, dates, unit, document category | **partly**: "Matrisen 7 - Angående återkommande besiktning av hiss D1431547 enligt BFS 2025:12, ärende nr BN 2026-000968" and "Information om användningsförbud för hiss/hissar" appear, but in the municipal property owner's (tekniska nämnden) diary; byggnadsnämnden's own cases were not among the published journal units | DIFFERENT_BUT_MACHINE_ACCESSIBLE |
| Värmdö | Public 360 web diary (older UI) `varmdo.pj.360online.com` (`/Journal/SearchRelated?caseYear=&sequenceNumber=&documentNumber=`), cases from 2018-04-12, no documents | HTML, no login | free text, dates, department | not verified (search request format not reproduced) | DIFFERENT_BUT_MACHINE_ACCESSIBLE |
| Södertälje | `diariet.sodertalje.se/#!/search/` (single-page app over an unlisted API) | – | no login | free text | not verified | DIFFERENT_BUT_MACHINE_ACCESSIBLE (API not mapped) |
| Huddinge | "Searchport" external diary `externtdiarium.huddinge.se` (ASP.NET WebForms) | – | no login | quick search, diary selection | not verified | DIFFERENT_BUT_MACHINE_ACCESSIBLE |
| Solna | "Sök i diariet": Lex2Publish (Blazor WebAssembly, `lexapi.solna.se/Lex2PublishWasm`) | – | no login | – | not verified | DIFFERENT_BUT_MACHINE_ACCESSIBLE (WASM client, API not mapped) |
| Gävle | Formpipe **Platina Webbdiarium** `webbdiariet.gavle.se` (`POST /home/DoSearchCases`) | HTML, no login | diary, title, diary number, dates | **no**: only KS and VN diaries are published, not byggnadsnämnden; our search request answered HTTP 500 | DIFFERENT_BUT_MACHINE_ACCESSIBLE (platform), lift cases not published |

INFERENCE on families. Stockholm's service is bespoke and, as far as
this scan shows, unique. Elsewhere two things matter: whether the
municipality publishes a **web diary** at all, and whether
byggnadsnämnden's cases are in it with the inspection result in the
title. Web diaries come from a handful of platform families (Formpipe
Platina Webbdiarium, Tietoevry Public 360 "Public Journal", Formpipe
W3D3 / Lex2Publish, Searchport, a few bespoke apps), each machine
accessible with one parser per family. But the content is not
standardised: Umeå's titles say "Angående återkommande besiktning av
hiss D1431547 … BFS 2025:12" (device id present, result only in the
"Information om användningsförbud" document title), Stockholm's say
"Intyg återkommande besiktning, ej godkänt, D1431547". Several large
cities (Göteborg, Malmö, Linköping, Örebro, Norrköping) expose building
cases only to the applicant behind e-legitimation. So the realistic
picture is neither "290 bespoke adapters" nor "one platform": roughly
five platform families for the municipalities that publish a diary,
each needing its own title rules, plus a large group with no public
machine access where the signal would require records requests.

## 14. Example opportunities (VERIFIED rows)

Failed → approved (September 2025 cohort; fields: diary number, failed,
property, address, district, asset, identifier, approved, closed):

| Diary | Failed | Property | Address | District | Asset | Identifier | Approved | Closed |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 2025-13184 | 2025-09-02 | Piloten 2 | Gondolgatan 16 | Skarpnäcks Gård | lift | L1142264 | 2025-12-12 (+101 d) | 2026-05-19 |
| 2025-13183 | 2025-09-02 | Rekryten 4 | Gyllenstiernsgatan 18 | Östermalm | lift | L3110756 | 2025-12-16 (+105 d) | 2026-05-19 |
| 2025-13181 | 2025-09-02 | Primusköket 1 | Essinge Brogata 2 | Lilla Essingen | lift | L3485364 | 2026-06-12 (+283 d) | 2026-06-12 |
| 2025-13207 | 2025-09-02 | Veterinären 13 | Skeppargatan 74 | Östermalm | lift | 27079 | 2026-05-27 (+267 d), two reminders | 2026-05-27 |
| 2025-13202 | 2025-09-02 | Ledarö 3 | Lysviksgatan 48 | Farsta | lifts | 76186, 76187, 76188, 76189 | 2025-12-11 (+100 d) | open |
| 2025-13549 | 2025-09-09 | Varmfronten 1 | Skarpnäcks Allé 64–74 | Skarpnäcks Gård | lifts | L2983768, L2984632, L2984758, L2994756 | 2025-09-15 (+6 d) | 2025-12-02 |
| 2025-13547 | 2025-09-09 | Duvan 6 | Klara Södra Kyrkogata 1 | Norrmalm | lift | L2968445 | 2026-05-28 (+261 d) | 2026-05-28 |
| 2025-13649 | 2025-09-09 | Hedvig 22 | Spånga Stationsväg 71 | Solhem | lift | L7894891 | 2025-11-28 (+80 d), failed twice | 2026-05-19 |
| 2025-13635 | 2025-09-09 | Aspsätra 1 | Aspsätravägen 29 | Sätra | lift | D1605096 | 2026-01-12 (+125 d) | 2026-05-26 |
| 2025-13626 | 2025-09-09 | Helsingör 3 | Helsingörsgatan 34 | Kista | lift | L7671336 | 2025-12-29 (+111 d) | 2026-05-19 |

Recent and open (September 2026):

| Diary | Failed | Property | Address | District | Asset | Identifier(s) | Source wording |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 2026-15984 | 2026-09-23 | Kronkvarnen 39 | Artillerigatan 48 | Östermalm | lifts | 53119 | Intyg återkommande besiktning, ej godkänt, 53119 |
| 2026-15987 | 2026-09-23 | Minan 4 | Karlavägen 76 | Östermalm | lifts | 55117, 53489, 55116 | Intyg återkommande besiktning, ej godkänt, 55117, 53489, 55116 |
| 2026-15988 | 2026-09-23 | Trumslagaren 7 | Karlaplan 18 | Östermalm | lift | 51853 | Intyg återkommande besiktning, ej godkänt, 51853 |
| 2026-15990 | 2026-09-23 | Skären 4 | Biblioteksgatan 3 | Norrmalm | lift | L2992070 | Intyg återkommande besiktning, ej godkänt, L2992070 |
| 2026-15991 | 2026-09-23 | Skären 6 | Smålandsgatan 16 | Norrmalm | lifts | L2965939, L2989824 | Intyg återkommande besiktning, ej godkänt, L2965939, L2989824 |
| 2026-15994 | 2026-09-23 | Zachrisberg 5 | Årstaskogs Väg 10 | Liljeholmen | lift | D1626077 | Intyg återkommande besiktning, ej godkänt, D1626077 |
| 2026-15995 | 2026-09-23 | Ritstiftet 2 | Ångermannagatan 176 | Vällingby | lift | D1733519 | Intyg återkommande besiktning, ej godkänt, D1733519 |
| 2026-15847 | 2026-09-22 | Karteschen 9 | Furusundsgatan 18 | Ladugårdsgärdet | lift | L2533977 | Intyg återkommande besiktning, ej godkänt, L2533977 |

Case pages: `https://etjanster.stockholm.se/Byggochplantjansten/arende/arende/<RecNo>?dataSource=Active`
(RecNo in the CSV). No personal data appears in these rows; the source
masks nothing here because owners are not shown.

## 15. Proposed canonical event (INFERENCE, not implemented)

The failed certificate is a source fact of the same kind as the
Arbetsmiljöverket recurring-inspection certificate: an accredited body
found that a regulated device did not meet the safety requirement and
the public authority registered that finding. A source-independent
canonical type could be

```text
MOTORISED_BUILDING_EQUIPMENT_INSPECTION_FAILED
```

(a lift, powered door or gate, escalator or similar device in a building
failed its mandatory recurring inspection, as registered by the
municipal building committee). One type for all asset kinds is the right
granularity under `docs/public-events.md`: the asset kind (hiss/port/
rulltrappa) is source wording that belongs in the title and payload,
and the recipe or assessment layer can split it, exactly as the device
family is handled for `WORK_EQUIPMENT_INSPECTION_FAILED`. Whether this
should instead *reuse* `WORK_EQUIPMENT_INSPECTION_FAILED` is a real
question: the regulatory regime (PBL/PBF vs AFS 2023:11), the authority
(municipality vs Arbetsmiljöverket) and the identity (property vs
organisation/CFAR) differ, and the existing type's documented meaning is
work equipment under the work environment authority, so a separate
neutral type is the honest choice. Title: the source document
description verbatim ("Intyg återkommande besiktning, ej godkänt,
L2992070") or the case title; both are source wording. The event has no
organisation (null) and no CFAR; the property designation and address
would need a place in the canonical model, which today has only
organisation and workplace fields — see the implementation plan.

## 16. Minimum implementation plan (INFERENCE, not implemented)

Not implemented. If continued:

* **New adapter** `internal/source/stockholm` (the search and case pages
  share nothing with Arbetsmiljöverket's markup); reuse the observation
  and event repositories, transaction and CLI pattern. No framework.
* **Requests per run**: one search per date window (`CaseTypeRecNo=200001`,
  `JournalPlanCode=7.2`, `CaseStartDateFrom/To`), returning every case
  as embedded JSON, then one case-page request per case in the window
  (about 5 per weekday). Re-fetch open cases for a bounded period to
  see later documents.
* **Pagination**: none needed; the search embeds the full result set.
* **Acceptance rule**: a document in a 7.2 case whose description starts
  with `Intyg återkommande besiktning, ej godkänt` (also `Intyg
  ombesiktning, ej godkänt`); "godkänt", "delvis godkänt", reminders,
  complaints and notes are not events.
* **Identity**: the source has no document id. Source record = the case
  (`RecNo`), observation payload = case metadata plus the document list,
  so a later approved certificate appended to the case is a changed
  observation. Event identity = `<RecNo>` plus the ordinal of the failed
  certificate within the case (`1325123:1`), with the certificate's
  description, date and asset identifiers in the payload; asset
  identifiers are the natural key for tracking a lift across cases.
* **Later approved inspections**: not an event of this type; they stay in
  the observation payload (and a later `…INSPECTION_PASSED` type is a
  separate decision).
* **Model**: property designation and address need canonical fields or a
  documented convention (organisation null, workplace null, title carrying
  the property) before implementation; this is the one design decision.

## 17. Answers to the spike's questions

1. **Machine-readable?** Yes: plain GET, embedded JSON, stable case ids,
   no login; access GOOD.
2. **Does the metadata prove the failure?** Yes: the document description
   states "ej godkänt"; 2,096 such documents, no false positive found.
3. **How many?** About 1,950 failed certificates a year (160 a month),
   1,700 properties in 13 months.
4. **Exact property and device?** Property designation, address and
   district on every case; a device identifier on 99.5 % of failed
   documents, stable across re-inspections.
5. **Remediation completion visible?** Yes, as an approved certificate in
   the case (59 % within 9–12 months) plus the city's reminders; median
   238 days from failure to approval.
6. **Owner resolvable?** Not from the source; deterministically later via
   property designation → fastighetsregister owner (licensed data).
7. **Enough volume for service companies?** For Stockholm-based lift
   service, yes; incumbent contracts make the conversion uncertain.
8. **Gates and other equipment?** Yes, distinguishable from the case title:
   7 % powered doors and gates, 1 % escalators.
9. **Replication?** Hard: Stockholm's service is bespoke; of the twelve
   other municipalities probed, none exposes byggnadsnämnden's lift cases
   with the result in a login-free search, and five large cities have no
   public machine access at all.
10. **Next production source?** As a Stockholm-only source, yes; as a
    municipal programme, no.

## 18. Limitations

* One case page was not read (connection reset); the search rows are
  complete.
* Asset type comes from the case title ("hiss", "port", "rulltrappa"),
  not from the certificate; nine cases had neither.
* Identifier prefixes and their inspection bodies are inferred, not
  documented by the source.
* Lifecycle figures are per case; approved certificates filed in a
  separate case are only linked when the identifier matches (29
  identifiers seen in more than one case).
* Portability probes were one to three requests per municipality on one
  day; Uppsala's web diary was down, and the Värmdö, Södertälje,
  Huddinge and Solna search requests were not reproduced.
* Nothing about owners, supplier engagement or order values comes from
  the source; those parts are inference and marked as such.
