# Food control signal evaluation

Discovery spike, 2026-09-25. Candidate signal:
`FOOD_CONTROL_NON_COMPLIANCE_DETECTED`, the municipal food-control
authority (livsmedelskontrollen) found that a food business did not meet
food law at an inspection.

Everything marked VERIFIED was read on 2026-09-25 with plain HTTP (curl,
no browser automation, no login, no API key, nothing ordered) and is
quoted from the response; everything marked ESTIMATE or INFERENCE is our
reading. Research scripts lived in a scratch directory and are not part
of the repository. No production code was changed.

## 1. Executive summary

**NO-GO.**

* **Access is fragmented, not national.** Every control authority
  (249 municipal authorities plus Livsmedelsverket) reports inspection
  results per establishment to Livsmedelsverket once a year, but that
  database is not public: the extraction tool sits behind the
  authorities' login portal, and Livsmedelsverket's open data is food
  composition only. What exists publicly is a handful of municipal
  "Livsmedelskollen" services on six different platforms. Three of them
  are automatable with useful detail (Stockholm, JSON search API;
  Uppsala, HTML list plus detail pages; Örebro, WFS plus a JSON report
  endpoint per inspection), two give only a yes/no result (Karlstad
  WFS, Jönköping ArcGIS), one open-data API that follows the national
  specification is dead (Linköping), and the second and third largest
  cities publish nothing per establishment (Göteborg, Malmö).
* **No company identifier anywhere.** The national open-data
  specification recommends `organizationNumber`; no live publisher
  emits it. Stockholm accepts an organisation number as a search input
  but never returns one, and its filter is a substring match that would
  also expose sole traders' personal identity numbers. Names are trading
  names (a legal form appears in 8.5 % of Stockholm names, 2 % of
  Uppsala's); the address is present on 94–100 %. Deterministic matching
  would need a name-plus-address join against a workplace register,
  which is not deterministic on trading names and could not be
  quantified in this spike.
* **Finding detail is coarse except in Uppsala and Örebro.** Stockholm
  gives the legislative area letter (J Hygien, B Information, K
  Riskhantering …), which separates hygiene from labelling but cannot
  isolate pests or temperature. Uppsala and Örebro give
  Livsmedelsverket's reporting points (J06 Bekämpning av skadedjur, J08
  kylkedjan …), the only sources where a pest or temperature event can
  be derived; Örebro's web page deliberately withholds results for 30
  days after the inspection.
* **Most events are noise for a seller, and the window is short.** In
  the 45 most recent Uppsala deviation inspections, 24 were labelling,
  information, administrative or personal-hygiene remarks, or were
  already marked fixed at a follow-up one to three weeks later; 15 were
  possible leads (temperature, premises, HACCP); 6 were strong (pests or
  an unresolved physical deficiency). At the accessible volume (about
  190 deviation inspections a month in Stockholm, Uppsala and Örebro
  together, ESTIMATE) that is roughly 25 strong leads a month, none
  with an organisation number.

Recommendation: stop spending time on this source and proceed to the
next candidate signal. The two things that would reopen it are listed at
the end of section 12.

## 2. Data landscape

Who controls whom (VERIFIED, Livsmedelsverket's report L 2025 nr 13
"Sveriges livsmedelskontroll 2024", pages 36–48):

* Municipalities control "knappt 94 000 verksamheter" (restaurants,
  shops, school kitchens, producers); Livsmedelsverket controls 1 760
  (large industry, slaughterhouses); länsstyrelserna control 37 896
  primary producers (farms). 249 municipal control authorities.
* 78 181 controls in the stages after primary production in 2024
  (53 358 planned, 20 123 follow-up, 4 700 event-driven), down from
  94 688 in 2023 because of the new risk classification.
* Deviations at 38 % of planned controls; the median authority notes
  deviations in 20.9 % of its controls, with a wide spread.
* Decisions in 2024: 3 889 förelägganden on the business, 520 förbud on
  a specific food, 489 förelägganden on a specific food, 225 förbud on
  part of the business, 118 closures, 251 sanction fees, 5 police
  reports. Only 20 % of businesses with a deviation got a decision.

Where the data goes (VERIFIED, Kontrollwiki articles 548, 1004, 1008):

* Every authority uploads an XML file to Livsmedelsverket's
  "Myndighetsrapportering" by 31 January for the previous year, with
  establishments, controls, results per reporting point, deviation ids,
  follow-up assessments (Åtgärdad / Kvarstår / Avskriven) and measures.
* Reporting points are the national taxonomy: legislative areas A–Q,
  each with numbered points (bilaga 2), e.g. J02 Utformning och
  underhåll av lokaler och utrustning, J03 Hygien före, under och efter
  processen, J04 Personlig hygien, J05 Utbildning, J06 Bekämpning av
  skadedjur, J08 Upprätthållande av kylkedjan, K01 Faroanalys och
  kritiska styrpunkter, K06 Allergena kriterier, H01 Spårbarhet, B02
  Obligatorisk livsmedelsinformation.
* The reported data is used for the EU report and for indicators; the
  "Uttagswebb" that lets one query it per year and authority is reached
  through Livstecknet, the authorities' login site. Nothing per
  establishment is published nationally. Livsmedelsverket's open-data
  page lists only food composition data.

Classification of what exists:

| Kind | Examples | Usable for company-level signals? |
| --- | --- | --- |
| Nationally available | annual report L 2025 nr 13, Kolada KPI U07455 (Insikt index), Livsmedelsverket code lists | no, aggregates |
| Reported nationally, not exposed | Myndighetsrapportering database, Uttagswebb (login) | not without a records request per authority |
| Municipality-specific, public | Stockholm, Uppsala, Karlstad, Jönköping, Örebro, Höganäs, Sjöbo, Lomma, Hallstahammar, Kristinehamn "Livsmedelskollen"/maps | yes, per municipality, six platforms |
| Open-data specification | Sambruk/NSÖD/ÖDIS "Livsmedelskontroller som öppna data" v2.0 (JSON/CSV, `organizationNumber` recommended, reporting points, assessment 0–2) | in principle; one publisher found (Linköping) and its endpoint is dead |

## 3. Candidate sources

| Source | Coverage | Structured | Automated access | Company ID | Finding detail | Freshness | Verdict |
| --- | --- | ---: | ---: | ---: | ---: | --- | --- |
| Livsmedelsverket Myndighetsrapportering / Uttagswebb | national, all authorities | yes (XML per establishment) | no, login (Livstecknet); aggregates only | unknown | reporting points | yearly, 31 Jan | NO |
| Livsmedelsverket open data | national | – | – | – | – | – | NO (food composition only) |
| Stockholm "Livsmedelsinspektioner" e-service | Stockholm (about 8 000 establishments, ESTIMATE) | JSON | POST search API, 1 500-result cap, no date filter | no (input only) | area letter + date + reason + result | same day (inspections dated 2026-09-25 visible 2026-09-25) | PROMISING for access, WEAK for identity |
| Uppsala "Livsmedelskollen" | Uppsala, 1 864 establishments | HTML | GET list with result/type/date filters, detail pages | no | reporting points + verdict + follow-up | one day (2026-09-24 visible 2026-09-25) | PROMISING for detail, WEAK for identity |
| Karlstad Livsmedelskollen (GeoServer WFS) | Karlstad, 696 establishments | GeoJSON | WFS GetFeature | no | latest control: date, type, Ja/Nej, diary no. | about 3 weeks (latest 2026-09-02) | WEAK |
| Jönköping kommunatlas (ArcGIS) | Jönköping, 1 146 establishments | JSON | ArcGIS REST query | no; has fastighetsbeteckning | latest three controls as HTML text, Utan/Med avvikelse | about 1 week | WEAK |
| Linköping "Livsmedelskontroller" open data | Linköping | JSON/XML per spec | API key required and endpoint dead (404 / connection reset) | spec: recommended | spec: reporting points | spec: daily | NO |
| Örebro livsmedelskarta.orebro.se + orebro.se foodreport REST | Örebro, 1 234 establishments | GeoJSON + JSON | WFS list of establishments, facility page with inspection ids, `/rest-api/foodreport/reports/{inspectionId}` | no | reporting points with per-point result (Utan avvikelse / Avvikelse / Åtgärdad / Kvarstår / Avskriven), date, reason, announced | API answers within 2 days; the city's page shows "Handläggning pågår" for 30 days | PROMISING for detail, WEAK for identity |
| Göteborg, Malmö | – | – | none; Göteborg: "lämnar vi ut informationen" on request; Malmö: yearly summary article only | – | – | – | NO |
| Sambruk specification publishers on dataportal.se | – | – | only Linköping found by title search | – | – | – | NO |

## 4. Best source

Two sources are needed to show the ceiling: Stockholm for access and
volume, Uppsala for detail. Neither carries a company identifier.

### Stockholm

**Entry point** `https://etjanster.stockholm.se/livsmedelsinspektioner/`
(also the "Livsmedelskollen" app by Visma Consulting).

**Endpoint** (VERIFIED)

```text
POST https://etjanster.stockholm.se/Livsmedelsinspektioner/Livsmedelsinspektioner/SearchFacilitiesMap
Content-Type: application/json
{"FoodPlaceName":"sushi","FoodPlaceAddress":"","FoodPlaceOrgNr":"",
 "NoDeficiency":false,"MinorDeficiency":false,"Revisit":false,
 "FacilityTypeGroups":[],"SortByName":null,
 "NorthCoordinate":null,"EastCoordinate":null,"MaxDistanceAllowedFromPoint":null,"Ids":null}
```

Response: a JSON-encoded string holding an array of establishments:

```json
{"Name":"Asami sushi","Id":"59bd3dc8-…","Address":"Svartviksslingan 104",
 "SweRefCoordinateEasting":…, "Business":"1. Restaurang","Judgement":1,"ReviewLabel":"Med avvikelser",
 "ReadMore":"+ Läs mer om inspektioner i senaste ärendet (Diarie-/ärendenummer: )",
 "InspectionList":[
   {"InspectionDate":"2026-07-31","TypeText":"Oanmäld","ReasonText":"Ordinarie kontroll",
    "SummaryText":"Vid inspektionen konstaterades avvikelser",
    "ControlAreaList":[{"Number":"J","Title":"Hygien","Link":"http://foretag.stockholm.se/…/#J"}]},
   {"InspectionDate":"2025-10-09","TypeText":"Oanmäld","ReasonText":"Ordinarie kontroll",
    "SummaryText":"Vid den senaste inspektionen konstaterades inga avvikelser i de områden vi kontrollerade","ControlAreaList":[]}]}
```

Observed over 1 500 establishments (the cap of an empty search) and
their 7 455 inspections (VERIFIED):

| Field | Values |
| --- | --- |
| Judgement / ReviewLabel | 0 Utan avvikelser 1 221, 1 Med avvikelser 150, 4 Inspektion ännu inte gjord 129 |
| ReasonText | Ordinarie kontroll 5 517, Uppföljande kontroll 1 468, Händelsestyrd kontroll 245, Annan anledning 225 |
| TypeText | Oanmäld 5 986, Föranmäld 1 466 |
| SummaryText | inga avvikelser 5 344, konstaterades avvikelser 2 095 (28 %), Omdöme saknas 16 |
| ControlAreaList on deviation inspections | J Hygien 1 267, B Information om livsmedel 680, K Riskhantering 221, H Spårbarhet 100, A Administration 85, C Specialinformation 66, E, M, I, D, O, F, Q ≤ 22; 1.18 areas per deviation inspection |
| Dates | 2018-01-03 to 2026-09-25; 1 104 inspections in 2025, 781 in 2026 to date |
| Address | present on 1 404 of 1 500 |
| Diary number | the ReadMore text carries an empty "Diarie-/ärendenummer" on all 1 500 |

Filters: `MinorDeficiency=true` alone returned 888 establishments whose
latest inspection found deviations; `Revisit=true` returned 0 ("Extra
kontroll krävs" is not in use today). The `Ids` parameter filters by
facility id. `FoodPlaceOrgNr` is a substring filter on a stored
organisation or personal number that is never returned: for one
establishment, single characters `-`, `0`, `1`, `2`, `9` matched and
`5`, `16`, `55` did not, so the stored value is a hyphenated personal
identity number (sole trader). Reconstructing numbers this way is
possible in principle and unacceptable in practice.

**Pagination** none; 1 500 results per query, sorted by name. A full
crawl needs partitioning by name prefix, area (`NorthCoordinate`,
`EastCoordinate`, `MaxDistanceAllowedFromPoint`) or facility type. An
empty search took 17 s and 14.6 MB; the 888-establishment deviation
query took 14 s and 9.5 MB.

**Incremental ingestion** there is no date filter and no changed-since
parameter. The workable loop is: daily, fetch the `MinorDeficiency=true`
set (one 10 MB request), diff each establishment's `InspectionList`
against the last observation, emit an event for each new inspection
whose SummaryText says deviations were found. Inspections dated the
same day are already visible, so latency is a day at most.

**Identifiers** facility `Id` (GUID, stable across the two runs made),
name, address, SWEREF coordinates, business type. No organisation
number, no diary number, no property designation.

**Constraints** an unauthenticated e-service with no published terms of
use or rate limit; no robots exclusion checked because the endpoint is
not crawled by URL. Sole traders' establishment names can be personal
names.

### Uppsala

**Entry point**
`https://www.uppsala.se/foretag-och-naringsliv/tillstand-regler-och-tillsyn/livsmedelskollen/`

**Requests** (VERIFIED)

```text
GET …/livsmedelskollen/?query=&selectedResults=1&page=2     # 1 = Avvikelse, 0 = Utan avvikelse, 2 = new/not inspected; selectedTypes 0-3; date filter
GET …/livsmedelskollen/Details?id=-1922505589
```

The list shows 1 864 establishments (1 174 Utan avvikelse, 51
Avvikelse as latest verdict, 639 new or not yet inspected), sorted by
latest control, with name, address and latest verdict and date. A detail
page lists the five latest inspections with Kontrolldatum, Anledning
(Planerad kontroll / Uppföljning av tidigare avvikelse / Annan kontroll),
Typ (Oanmäld / Föranmäld), Diarienummer (MHN-2026-3301), Omdöme (Utan
avvikelse / Avvikelse / Avvikelse åtgärdad / Avvikelse kvarstår) and,
for Avvikelse, the deviations as area heading plus reporting-point name:

```text
Kontrolldatum: 2025-05-17  Anledning: Planerad kontroll  Typ: Oanmäld  Diarienummer: MHN-2025-3829  Omdöme: Avvikelse
Avvikelser:
  Grundförutsättningar, hygien
    Utbildning i hygien och arbetsmetoder
    Upprätthållande av kylkedjan och uppfyllande av temperaturkriterier
  HACCP-baserade förfaranden
    Faroanalys och kritiska styrpunkter
Kontrolldatum: 2025-06-11  Anledning: Uppföljning av tidigare avvikelse  Diarienummer: MHN-2025-3829  Omdöme: Avvikelse åtgärdad
```

Crawled sample: 107 establishments (all 51 with a latest Avvikelse plus
the 70 most recently inspected), 332 inspections from 2024-08-06 to
2026-09-24: Avvikelse 124, Utan avvikelse 96, Avvikelse åtgärdad 87,
Avvikelse kvarstår 25; 201 planned, 118 follow-ups, 13 other. Every
Avvikelse record carried its reporting points (2.3 per inspection,
max 11).

**Incremental ingestion** the list accepts a date filter and is sorted
by latest control, so a daily fetch of page 1–2 with
`selectedResults=1` plus the detail page of each new or changed
establishment (about 30 requests a day) is enough. Diary number and
date make an inspection identity; the follow-up inspection reuses the
diary number, which links the outcome to the original deviation.

**Identifiers** name, address, diary number, an opaque numeric id in
the detail URL. No organisation number, no facility category in the
detail page (the list filter has four types).

**Constraints** an EPiServer site with anti-forgery tokens on the form,
but plain GET works; no terms found. Names of sole traders appear.

### The others, briefly

* Karlstad: `https://gi.karlstad.se/geoserver/ows?service=WFS&version=2.0.0&request=GetFeature&typeNames=webbkartan:livsmedelskontroller_restaurang&outputFormat=application/json`
  and three sibling layers (696 features): `namn`, `kategori`,
  `inriktning`, `senaste_tillsyn_datum`, `kontroll` (Ordinarie /
  Extra), `avvikelser` (Ja/Nej: 123 Ja), `arendenummer` (MN-2026-1631).
  Only the latest control, no address in the properties, no points.
* Jönköping: `https://gis.jonkoping.se/arcgis/rest/services/kommunatlas/Kommunatlas_Naringsliv_och_Arbete/MapServer/10/query?where=har_avvikelse%3D%271%27&outFields=*&f=json`
  (layers 10–14, 1 146 features, 21 with `har_avvikelse=1`): name,
  address, `fastighetsbeteckning`, type, and `senaste_kontroller` as
  HTML text ("2026-02-12 Planerad kontroll <br>Kontrollresultat: Med
  avvikelse"). No points.
* Linköping: dataset "Livsmedelskontroller" on dataportal.se
  (`https://linkoping.entryscape.net/store/1/resource/66`, DAILY),
  distribution `http://livsmedelsdata.linkoping.se/swagger/index.html`
  (404); the city's page documents
  `http://opendata.linkoping.se/ws_opendata/main.asmx/LivsmedelsobjektAlla?CustomKey=…`
  which needs an API key and answered 404 / connection reset on
  2026-09-25.
* Örebro: `https://livsmedelskarta.orebro.se/` is a Hajk map
  (`appConfig.json` → `/api/v2/config/livsmedelskarta`) over a QGIS
  Server WFS:
  `https://livsmedelskarta.orebro.se/api/v2/proxy/qgisserver/maps/ExternKarttjanst/Livsmedelskontroller?SERVICE=WFS&VERSION=2.0.0&REQUEST=GetFeature&TYPENAMES=qgs:Livsmedelsverksamheter_p&OUTPUTFORMAT=application/json`
  returns all 1 234 establishments with `Objektsnamn`, `AnlaggningId`
  (GUID), `AllaInriktningar` ("Team Restaurang", "Team Butik", …) and
  the `url` of the establishment's result page on orebro.se. That page
  embeds a `const INSPECTIONS = [{id, reason, date}, …]` array (ten
  inspections back to 2020 for the store tried) and its script calls
  `https://www.orebro.se/rest-api/foodreport/reports/{inspectionId}`,
  which returns one row per reporting point:

  ```json
  [{"Nr":"J02","Anmarkning":"Avvikelse","Beskrivning":"Utformning och underhåll av lokaler och utrustning",
    "TillsynsDatum":"2026-09-23","Kontrollomrade":"Grundförutsättningar, hygien","Anmald-oanmald":"Oanmäld"}, …]
  ```

  Sampled: 45 establishments, 76 inspections (their two latest), 501
  point rows: Utan avvikelse 445, Avvikelse 30, Åtgärdad 23, Kvarstår 2,
  Avskriven 1; 37 inspections had at least one row that was not "Utan
  avvikelse". Deviation points: J02 11, J08 10, J03 9, K01 6, H01 6,
  B04 3, B02 2, J09 2, C03 2, others 1. The page script replaces the
  result with "Handläggning pågår" for inspections younger than 30 days,
  but the REST endpoint already returned "Avvikelse" for an inspection
  dated 2026-09-23 (two days old). The facility page also says "Vid
  ägarbyte tar vi bort resultatet från tidigare kontroller" and that
  full reports are obtained through an e-service that needs the
  requester's contact details.

## 5. Company matching

What the records carry (VERIFIED):

| Identifier | Stockholm | Uppsala | Karlstad | Jönköping |
| --- | --- | --- | --- | --- |
| organisationsnummer | searchable, never returned | no | no | no |
| establishment id | GUID | opaque int in URL | GUID | OBJECTID |
| name | trading name; legal form in 128 of 1 500 | trading name; legal form in 2 of 107 | 45 of 696 | trading name |
| address | 1 404 of 1 500 | 107 of 107 | not in WFS properties | yes |
| property designation | no | no | no | yes |
| municipality | implicit | implicit | implicit | implicit |
| category | business type (Restaurang, Butik, Café, Förskola tillagning, Kosttillskott …) | four list filters | kategori/inriktning | typ/kategori |
| operator / legal entity | no | no | no | no |

Consequences:

* No source supports a deterministic company match by identifier.
* A name-plus-address join to a workplace register (SCB arbetsställen
  with CFAR, which Bolagsbild already carries for the Arbetsmiljöverket
  source) is the only automatable route. Trading names ("Satsu sushi",
  "Blackbird", "Hemköp Uppsala Svava") often differ from both the legal
  name and the registered workplace name, and one address can host
  several companies. The match rate could not be measured here because
  the spike has no access to the register; INFERENCE from the names is
  that exact matches would be a minority and anything above that needs
  fuzzy matching, which the brief rules out.
* Chains and franchises (Hemköp, ICA, Espresso House, Max, Norlandia)
  would match to the franchisee or store company only through the
  address, and the buyer of a service is often the chain, not the
  store.
* Sole traders are a material share (Stockholm's substring test hit a
  personal number on the first establishment tried); their identifier
  is a personal identity number, which Bolagsbild must not collect from
  this channel.

Percentages asked for: organisation number 0 % (all sources); exact
legal name about 8 % (Stockholm), 2 % (Uppsala); usable address 94 %
(Stockholm), 100 % (Uppsala, Jönköping), 0 % in Karlstad's WFS
properties; matchable to Bolagsbild company data: not measurable
without the register, expected low on exact rules.

## 6. Signal taxonomy

Only what the sources actually carry. Confidence is about whether the
type can be derived deterministically from source fields, not about
commercial value.

| Signal type | Source fields | Interpretation | Confidence | Example (VERIFIED) |
| --- | --- | --- | --- | --- |
| `FOOD_CONTROL_NON_COMPLIANCE_DETECTED` | Stockholm SummaryText "Vid inspektionen konstaterades avvikelser"; Uppsala Omdöme "Avvikelse"; Karlstad avvikelser "Ja"; Jönköping "Med avvikelse" | at least one deviation from food law at the inspection | high in all four | Stockholm restaurant, 2026-07-31, Oanmäld, Ordinarie kontroll, area J |
| `FOOD_CONTROL_HYGIENE_DEFICIENCY` | Stockholm ControlAreaList Number "J"; Uppsala area "Grundförutsättningar, hygien" | one or more of J01–J09 (premises, process hygiene, personal hygiene, training, pests, water, cold chain, contact materials) | high, but J is a bag of nine different things | Uppsala 2026-09-16: J03, J03, J02, J02, J01 |
| `FOOD_CONTROL_PEST_DEFICIENCY` | Uppsala point "Bekämpning av skadedjur"; Örebro row `Nr` "J06" with `Anmarkning` "Avvikelse" | pest control inadequate or pests found | high in Uppsala and Örebro; not derivable in Stockholm (letter J) | Uppsala hotel restaurant 2026-09-22: J02, J06 |
| `FOOD_CONTROL_TEMPERATURE_DEFICIENCY` | Uppsala point "Upprätthållande av kylkedjan och uppfyllande av temperaturkriterier"; Örebro `Nr` "J08" | cold chain or temperature criteria not met | high in Uppsala and Örebro | Uppsala private preschool 2026-09-03: J08; Örebro sample: 10 of 76 inspections |
| `FOOD_CONTROL_PREMISES_EQUIPMENT_DEFICIENCY` | Uppsala point "Utformning och underhåll av lokaler och utrustning"; Örebro `Nr` "J02" | premises or equipment design/maintenance | high in Uppsala and Örebro; the most common point (52 of 124 in Uppsala, 11 of 76 inspections in Örebro) | Örebro care-home kitchen 2026-09-23: J02 Avvikelse |
| `FOOD_CONTROL_HACCP_DEFICIENCY` | Stockholm area "K Riskhantering"; Uppsala area "HACCP-baserade förfaranden" / point "Faroanalys och kritiska styrpunkter" (K01) | hazard analysis / own-control procedures missing or inadequate | high | Uppsala 2026-09-15: K01, J09 |
| `FOOD_CONTROL_LABELLING_DEFICIENCY` | Stockholm area "B Information om livsmedel" / "C Specialinformation"; Uppsala areas B/C, points B02, B04, B99, C02, C03 | mandatory or voluntary food information wrong or missing (includes allergen information, but the point does not say which) | high for the area; allergen not separable | Uppsala bakery 2026-09-03: B02 |
| `FOOD_CONTROL_TRACEABILITY_DEFICIENCY` | Stockholm area "H Spårbarhet"; Uppsala area "Spårbarhet" (H01, H11) | traceability records or origin marking missing | high | Uppsala 2026-09-07: J02, J08, H01, K01 |
| `FOOD_CONTROL_DEVIATION_PERSISTS` | Uppsala Omdöme "Avvikelse kvarstår" on a follow-up with the same diary number; Örebro `Anmarkning` "Kvarstår" on a point | the deviation was not fixed by the follow-up | high in Uppsala and Örebro (25 of 332 Uppsala inspections; 2 of 501 Örebro point rows) | diary MHN-2026-… follow-up 2026-09-09 kvarstår |

Not supported by any source seen: `FOOD_CONTROL_ALLERGEN_DEFICIENCY`
as a distinct type (allergen information sits inside B02; K06
"Allergena kriterier" occurred 0 times in 124 deviation inspections),
severity, decisions (föreläggande, förbud, sanktionsavgift), remediation
deadlines, closures, and free-text findings. Stockholm's data has no
points and no follow-up outcome beyond the next inspection's result.

## 7. Volume analysis

National (VERIFIED, 2024): 95 324 establishments, 78 181 controls,
deviations at 38 % of planned controls; about 25 000 controls a year
with at least one deviation (ESTIMATE from 53 358 planned × 38 % plus a
share of follow-ups), 5 300 decisions with requirements, 251 sanction
fees.

Accessible through the public services found:

| Source | Establishments | Controls per year | Controls with deviations | Basis |
| --- | --- | --- | --- | --- |
| Stockholm | about 8 000 (ESTIMATE: Stockholm has 54 of the 626 national control staff-years; 1 500-cap prevents a count) | about 5 000–6 700 (ESTIMATE: 0.74 inspections per establishment in the 1 500 sample in 2025; 8.6 % of national controls) | about 1 400 a year, 120 a month (ESTIMATE: 28 % of inspections in the sample); 888 establishments currently flagged "Med avvikelser" (VERIFIED) |
| Uppsala | 1 864 (VERIFIED) | about 1 500 (ESTIMATE) | about 450 a year, 40 a month (ESTIMATE, 30 %); 51 establishments currently with Avvikelse as latest verdict (VERIFIED) |
| Örebro | 1 234 (VERIFIED) | about 1 000 (ESTIMATE) | about 300 a year, 25 a month (ESTIMATE: 30 "Avvikelse" point rows in 76 sampled inspections, roughly a third of inspections with a new deviation) |
| Karlstad | 696 (VERIFIED) | about 400 (ESTIMATE from 2026 monthly counts of 25–42) | 123 establishments currently "Ja" (VERIFIED) |
| Jönköping | 1 146 (VERIFIED) | not derivable | 21 establishments currently "Med avvikelse" (VERIFIED) |

Together the automatable sources cover roughly 13 000 of 95 000
establishments (14 %) and about 190 deviation inspections a month
(ESTIMATE). Unique legal entities cannot be counted (no identifier);
unique establishments are the counts above. Potentially actionable
events, using the section 8 rates (13 % strong, 33 % possible): about
25 strong and 60 possible leads a month across Stockholm, Uppsala and
Örebro.

## 8. Manual lead-quality review

Reviewed: the 45 most recent Uppsala inspections with Omdöme
"Avvikelse" (2026-05-27 to 2026-09-23), the only sample with
point-level detail. Classification rule: STRONG when pests (J06) were
found or a physical deficiency (J02/J08) was still "kvarstår" at the
follow-up; POSSIBLE when temperature (J08), premises/equipment (J02),
HACCP (K01) or process hygiene (J03) was among the points and the
deviation was not already marked fixed; NOISE when the points were only
information, labelling, administrative or personal-hygiene remarks, or
the follow-up had already marked the deviation fixed.

| Class | Count | Share |
| --- | ---: | ---: |
| STRONG LEAD | 6 | 13 % |
| POSSIBLE LEAD | 15 | 33 % |
| NOISE | 24 | 53 % |

Representative records (business names only; sole-trader and
person-named establishments omitted):

* STRONG. Hotel restaurant, central Uppsala, 2026-09-22, planned
  unannounced control: J02 premises/equipment and J06 pests. A pest
  contractor or a kitchen-maintenance firm has a concrete reason to
  call this week.
* STRONG. Mobile food unit, 2026-09-07: J02, J08, H01, K01; follow-up
  2026-09-09 "Avvikelse kvarstår". Unresolved cold-chain and equipment
  problem, but a one-person business.
* POSSIBLE. Private preschool chain kitchen, 2026-09-03: J08 cold
  chain. A refrigeration service lead if the chain buys centrally.
* POSSIBLE. Supermarket (Hemköp Uppsala Svava), 2026-09-11: J04
  personal hygiene and J08 temperature. Chain store; the buyer is not
  the store.
* POSSIBLE. Restaurant, 2026-09-16: J03 ×2, J02 ×2, J01, D08. Five
  hygiene points at once; cleaning or consultancy lead, no follow-up
  yet.
* NOISE. Bakery, 2026-09-03: B02 mandatory information; fixed at
  follow-up 2026-09-11.
* NOISE. Fair-trade shop, 2026-09-04: B99, B04, B02, B02 labelling only.
* NOISE. Municipal preschool kitchen, 2026-09-04: J01 general
  requirement; a public operator on framework agreements.
* NOISE. Burger restaurant, 2026-08-24: J08 and J02, fixed at the
  follow-up 2026-09-08, two weeks later.

What the review shows: deviations are found and fixed inside a two-
to three-week window (follow-ups in the sample came 2 to 35 days after
the deviation, 87 of 118 follow-ups ended "åtgärdad"), the business
usually fixes them itself, and a third of the sample is documentation.
A salesperson would want the pest and persistent-deviation records and
would ignore the rest.

## 9. Commercial use cases

| Signal | Likely buyer | Why now | Lead strength |
| --- | --- | --- | --- |
| Pest deficiency (J06, Uppsala only) | pest-control company | the authority documented inadequate pest control; the operator must show a fix at the follow-up within weeks; most restaurants already have a pest contract, so the buy is often a contract switch or an extra treatment | medium; 18 of 124 deviation inspections, about 6 a month in Uppsala |
| Temperature / cold-chain deficiency (J08, Uppsala only) | refrigeration and kitchen-equipment service | a cold room, fridge or process failed a temperature check; often a routine (thermometer, logging) rather than equipment fault, and the fix is usually internal | low to medium; 34 of 124 |
| Premises and equipment deficiency (J02, Uppsala only) | kitchen fit-out, ventilation, surfaces, cleaning | the authority found worn or unsuitable premises or equipment; the point does not say what or how much | low; 52 of 124 but unspecific |
| HACCP / own-control deficiency (K area, Stockholm and Uppsala) | food-safety consultant, training provider, digital own-control (egenkontroll) software | the operator lacks or does not follow a hazard analysis; a consultant can sell a plan, software vendors sell the system | low to medium; 221 of 2 474 Stockholm areas, 21 of 124 Uppsala |
| Hygiene area J (Stockholm, letter only) | cleaning, training, pest, refrigeration cannot be told apart | a deviation somewhere in nine hygiene points | low as a lead; useful only as a flag |
| Labelling / information (B, C) | labelling consultant, packaging printer | wrong or missing information; fixed with a label | low; the dominant category and mostly noise |
| Traceability (H) | ERP / traceability software | records missing | low; rare (100 of 2 474 Stockholm areas) |
| Deviation persists (Uppsala follow-up "kvarstår") | any of the above matching the point | the operator failed to fix it in the first window; a decision with requirements may follow | medium; 25 of 332 inspections |

Buyer identity is the recurring problem: every row above sells to the
operator, and the operator is unknown (no organisation number, trading
name only).

## 10. Canonical event proposal

Not justified for implementation. For the record, the minimal model the
two usable sources could fill is:

```text
food_control_non_compliance_detected
  source              stockholm-livsmedelskollen | uppsala-livsmedelskollen | orebro-foodreport
  source_record_id    establishment id + inspection date (Stockholm has no inspection id; Uppsala diary number + date)
  establishment       name, address, municipality code, business type (source wording)
  company             null (never identified by the source)
  inspection_date     Kontrolldatum / InspectionDate (the canonical event time)
  observed_at         first time our crawl saw it (publication is same-day to one day)
  reason              planned | follow-up | event-driven | other
  announced           true | false
  areas               legislative area letters (both) 
  points              reporting-point codes (Uppsala, Örebro)
  outcome             deviation | deviation persists (Uppsala follow-up, Örebro "Kvarstår")
  source_url          establishment page
```

Severity, decision and deadline do not exist in the sources and would
be null everywhere.

## 11. Risks

* **Fragmented municipal publishing.** 249 authorities, six platform
  families seen (bespoke ASP.NET, EPiServer, GeoServer WFS, ArcGIS,
  Hajk/QGIS Server plus a Sitevision REST endpoint, a dead ASMX web
  service), most authorities publish nothing. Adding a municipality
  means a new adapter each time, exactly the situation that ended the
  lift-inspection family.
* **Missing organisation numbers.** Zero sources return one; the
  national specification's `organizationNumber` has no live publisher.
  Matching would rest on trading names and addresses.
* **Delayed publication** is mixed: same-day in Stockholm, one day in
  Uppsala, three weeks in Karlstad, and Örebro shows "Handläggning
  pågår" for 30 days while its REST endpoint answers at once. Reading
  a result the municipality deliberately withholds is not acceptable,
  so Örebro's effective latency is 30 days. The remediation window
  closes within two to three weeks, so a lead is stale fast either way.
* **Low severity / weak buying intent.** Half the events are
  documentation; decisions with requirements follow only 20 % of
  deviations nationally and are not published by any source seen.
* **Personal data.** Sole traders' establishment names are personal
  names; Stockholm's search filter can leak personal identity numbers
  by substring probing. Any ingestion must avoid storing or deriving
  those.
* **Source instability.** Linköping's open-data API is dead; Uppsala's
  five-inspection window and Stockholm's 1 500 cap and absent date
  filter are UI conveniences, not contracts.
* **Terms and access.** No terms of use were found on the e-services;
  the Sambruk specification asks publishers for CC0 but none of the
  usable sources publishes under it.
* **Duplicate establishments and operator changes.** A new operator at
  the same address gets a new establishment (Uppsala's list shows 639
  "new or not inspected"); Örebro removes earlier results at an owner
  change ("Vid ägarbyte tar vi bort resultatet från tidigare
  kontroller"), so history disappears; none of the sources expose the
  operator change itself.

## 12. Recommendation

### NO-GO

Stop spending time on food-control data and proceed to the next
candidate signal. Access is possible for three cities but not
nationally, no source identifies the company, the finding detail that
makes a lead specific (pests, temperature) exists in two mid-sized
cities (one of which withholds results for 30 days), and most
published deviations are documentation fixed within weeks.

What would reopen the question, so the decision is easy to revisit:

* Livsmedelsverket publishes the Myndighetsrapportering data per
  establishment with organisation numbers, or a records request shows it
  can be obtained in bulk yearly (it would still be up to 13 months
  late, useful for profiles, not for timing).
* One or more large municipalities publish under the Sambruk
  specification with `organizationNumber` and `inspectionPoints`
  filled, through a live endpoint. Linköping's dataset is the one to
  watch; today it is registered as daily and returns 404.

Sources (all read 2026-09-25):

* https://www.livsmedelsverket.se/4ab379/globalassets/publikationsdatabas/rapporter/2025/l-2025-nr-13---sveriges-livsmedelskontroll_2024_.pdf
* https://kontrollwiki.livsmedelsverket.se/artikel/548/myndighetsrapportering
* https://kontrollwiki.livsmedelsverket.se/artikel/1004/information-om-kontrollresultat-2026
* https://kontrollwiki.livsmedelsverket.se/artikel/1008/rapporteringspunkter-bilaga-2-2026
* https://www.livsmedelsverket.se/en/about-us/open-data/
* https://sambruk.github.io/livsmedel/ (teknisk specifikation v2.xlsx, exempel-json, rekommendation)
* https://admin.dataportal.se/sparql (dataset titles containing "livsmedel")
* https://etjanster.stockholm.se/livsmedelsinspektioner/ and its SearchFacilitiesMap endpoint
* https://www.uppsala.se/foretag-och-naringsliv/tillstand-regler-och-tillsyn/livsmedelskollen/
* https://gi.karlstad.se/geoserver/ows (WFS, workspace webbkartan)
* https://gis.jonkoping.se/arcgis/rest/services/kommunatlas/Kommunatlas_Naringsliv_och_Arbete/MapServer
* https://linkoping.entryscape.net/store/1/resource/66 and https://www.linkoping.se/oppna-data-fran-linkopings-kommun/livsmedelskontroll
* https://livsmedelskarta.orebro.se/ (appConfig.json, /api/v2/config/livsmedelskarta, QGIS Server WFS) and https://www.orebro.se/rest-api/foodreport/reports/{inspectionId}
* https://goteborg.se/wps/portal/start/omsorg-och-stod/sjukvard-och-tandvard/matforgiftning/sa-kontrolleras-maten
* https://malmo.se/Aktuellt/Artiklar-Malmo-stad/2026-02-12-Sa-klarade-foretagen-i-Malmo-livsmedelskontrollerna-2025.html
