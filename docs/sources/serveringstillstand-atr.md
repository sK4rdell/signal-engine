# Source: Swedish alcohol serving permits (Alkohol- och tobaksregistret, ATR)

Research note for the fourth Signal Engine experiment: new permanent
serving permits and operator changes at serving establishments.
Everything marked VERIFIED SOURCE FACT was read on 2026-09-24 from
atr.folkhalsomyndigheten.se (public landing page and its public
front-end bundle, no login), folkhalsomyndigheten.se, Folkhälsodata,
alkohollagen (2010:1622) and alkoholförordningen (2010:1636) on
riksdagen.se, FoHMFS 2014:5, Göteborgs Stad's open-data catalogue, and the
serving-permit pages of Göteborgs Stad and Stockholms stad. Everything
marked COMMERCIAL INFERENCE is our reading.

## Verdict: STOP before code, strongest re-entry so far

VERIFIED SOURCE FACT: national permit data lives in Folkhälsomyndigheten's
ATR behind an account that must be applied for and is granted after a
secrecy review. ATR explicitly offers accounts to companies "som behöver
upplysning om vilka företag som har serveringstillstånd", delivers register
extracts in Excel, and has a dedicated external web-service role for
register extracts. The only open dataset is Göteborg's daily CSV of
currently valid permits, which has no organisation numbers, no dates and
no archived history.

Consequence for the task's stop conditions: condition 1 (no scalable
access to ATR data *today*) holds until an account is granted, so no
adapter was built. Unlike the IVO case, the source is designed for
external company access, so the re-entry is an application, not a
public-records exception.

## 1. System owner and register purpose (VERIFIED SOURCE FACT)

* Folkhälsomyndigheten keeps a central automated register "för tillsyn,
  uppföljning och utvärdering av lagens tillämpning samt framställning av
  statistik" (alkohollagen 13 kap. 1 §). It may hold name, personal or
  organisation number, address, phone, the activity the permit covers and
  its conditions (13 kap. 2 §). Skatteverket has direct access (13 kap. 4 §).
* Municipalities decide serving permits (8 kap. 1 §) and must send a copy
  of every decision to Folkhälsomyndigheten, länsstyrelsen and the police
  (9 kap. 7 §). Folkhälsomyndigheten: "Vi registrerar samtliga tillstånd i
  Alkohol- och tobaksregistret."
* ATR also carries tobacco permits, wholesaler/warehouse-keeper data from
  Skatteverket, the annual restaurangrapport, and produces a quarterly
  file of permanent-permit holders for Skatteverket.

## 2. Access path (VERIFIED SOURCE FACT)

`https://atr.folkhalsomyndigheten.se/` is a JavaScript application with a
token-authenticated API at `/api` and hCaptcha on the login and account
forms. Nothing is readable without an account; nothing was bypassed. The
public front-end bundle documents the following.

Account application (`/applyforaccount`), first question "Innan du kan gå
vidare behöver vi veta varför du behöver ett användarkonto":

1. "Jag representerar en kommun eller länsstyrelse. Jag behöver ha tillgång
   till alkohol- och tobaksregistret för att kunna utföra mina
   arbetsuppgifter eller konfigurera en systemintegration."
2. **"Jag representerar ett företag som behöver upplysning om vilka företag
   som har serveringstillstånd."** This opens "Ansökan om användarkonto för
   registerutdrag": organisation number, company, contact person, address,
   phone, e-mail, purpose (syfte), a checkbox "Ja, mitt företag är ett
   kreditupplysningsföretag", options for "samtliga aktuella
   serveringstillstånd" and "samtliga serveringstillstånd avslutade före
   visst datum", and GDPR consent. The information text: "Folkhälsomyndigheten
   ska enligt 13 kap. 1 § alkohollagen (2010:1622) föra ett centralt
   register över tillstånd som beviljats enligt alkohollagen. Om du vill ta
   del av uppgifter i registret fyller du i formuläret. Efter
   sekretessprövning och om du beviljas ett användarkonto kommer du att få
   inloggningsuppgifter via e-post. Uppgifterna du hämtar i registret
   levereras i Excel-format."
3. "Inget av ovan stämmer in på mig, men jag är intresserad av att ta del
   av statistik relaterat till alkohol- och tobaksregistret."

Roles defined in the application: "Registerutdrag serveringstillstånd"
(category external, gives the page `/permits/records`: "Här hämtas
registerutdrag över stadigvarande serveringstillstånd."), **"Webbtjänst
registerutdrag serveringstillstånd"** (category external: a machine
interface for the same extracts), "Webbtjänst stadigvarande
serveringstillstånd" (municipal: municipalities can push permits by web
service), "Webbtjänst restaurangrapport", plus municipal, county and
Folkhälsomyndigheten read/admin roles. Authority roles have "Exportera
resultat: Ladda ner alla uppgifter i sökresultatet i en Excelfil".

Not found: published terms of use, licence, price list, rate limits, or
any statement about automated retrieval beyond the existence of the
web-service role. These must be asked for in the application.

## 3. Other official sources (VERIFIED SOURCE FACT)

| Source | Finding |
| --- | --- |
| Göteborgs Stad open data, "Restauranger med serveringstillstånd" | CSV, CC0, published since 2023-06-12, refresh frequency DAILY (file last modified 2026-09-24 04:11). 1,028 rows of currently valid permits. Columns: `recTillsynsobjektId`, `Namn`, `Besöksadress`, `strVerksamhetsutoevareNamn` (operator name), `Restaurangnummer`, `strVerksamhetNamn`, `Fakturaadress`, `TypAvTillstaand` (1,022 Stadigvarande), `ServeringTill` (927 allmänheten, 55 both, 40 slutna sällskap), `Alkoholdrycker`, `Serveringstyper`, `Serveringstider`, `Villkor` (empty), SWEREF and WGS84 coordinates. **No organisation number, no decision or validity dates, no terminated permits.** 1,021 rows carry a distinct 8-digit restaurangnummer, all starting 3480. Operator names: 894 look like aktiebolag, 70 handelsbolag/kommanditbolag, 12 associations/public, 52 personal names or other (sole traders). The Wayback Machine holds no archived version of the file. |
| dataportal.se | no other municipality publishes a permit dataset; Stockholm's open-data catalogue has none. |
| Folkhälsodata (Folkhälsomyndigheten) | annual counts of permanent permits per municipality 2007–2025 (PXWeb API, open). Aggregates only. |
| Municipal web pages | Göteborg publishes a public map of permitted restaurants; Stockholm publishes process pages. Individual decisions are public records at each municipality. |

## 4. Fields (VERIFIED SOURCE FACT, from ATR's permit forms and search)

Permit types include Stadigvarande serveringstillstånd, Tillfälligt
serveringstillstånd, gårdsförsäljningstillstånd, tobacco permits and
Skatteverket-approved wholesalers. For a permanent serving permit the
model exposes:

| Group | Fields seen in the application |
| --- | --- |
| Holder | organisation number (default sort key of the permit list), name; the extract form is keyed on organisation number |
| Object (serveringsställe) | `objektNr` labelled **Restaurangnummer** (8 digits: the first four must match the municipality's län+kommun code, with variants for large cities, e.g. Stockholm 3180/4180/5180/6180 and Göteborg 3480…), `objektNamn`, `objektAdress`, `objektPostOrt`, municipality, county |
| Permit version | `versionsNr`, `versionsStart`, `forsaljningstidFrom` / `forsaljningstidTom` (valid from/to), serving times inside/outside/closed company, seats inside/outside, max persons, outdoor serving, tasting, own-spiced snaps, drink categories (sprit, vin, starköl, andra jästa, alkoholdrycksliknande preparat), `verksamhetsOmrade`, serving-space type, `villkor` |
| Status | "Annu inte giltigt" (future, filter "Inkludera framtida"), current, "Avslutat" (filter "Inkludera avslutade"; "Inkludera äldre avslutade" for permits ended before 2003) |

So the register is **versioned and historical**: permits not yet valid,
current, and terminated are all retrievable, and each permit has version
history. Which of these columns an external registerutdrag contains is
not verified.

Mapping to the task's wish list: holder organisation number and name yes;
legal form derivable; holder postal address in the register per 13 kap. 2 §;
phone per 13 kap. 2 §; e-mail and website not indicated. Establishment:
restaurangnummer, name, street address, postal town, municipality, county
yes; coordinates only in Göteborg's data. Permit: type (permanent /
temporary), public / closed company, valid from, valid to, status, drink
categories, serving times, indoor/outdoor, conditions yes; decision date
and decision municipality plausibly (the municipality is the decider) but
not seen as fields; previous operator not seen as a field.

## 5. Identity semantics

VERIFIED SOURCE FACT:

* The permit belongs to one holder for one premises: "Serveringstillstånd
  ska omfatta ett visst avgränsat utrymme som disponeras av
  tillståndshavaren" (8 kap. 14 §). Göteborgs Stad: "Ett serveringstillstånd
  gäller bara för det bolag, enskilda firma eller förening som fått det och
  för den lokal som står i beslutet. Om verksamheten upphör eller flyttar
  gäller inte tillståndet längre."
* Restaurangnummer is assigned to the serveringsställe of a permanent
  serving permit, is used on the annual restaurangrapport together with
  the holder's organisation number (FoHMFS 2014:5 bilaga), and encodes the
  municipality, not the holder.
* Ownership changes inside the same legal entity do not create a new
  permit; they are notified (9 kap. 11 §: "betydande förändringar av
  ägarförhållandena"; Stockholm: "Även om aktier säljs behöver du anmäla
  det", fee 7 870 SEK, handling about 6 months).

COMMERCIAL INFERENCE: because the number identifies the object and not the
holder, a new operator at the same premises may well receive a permit
under the same restaurangnummer, which would make "same restaurangnummer,
new organisation number" a clean operator-change detector. This is
exactly what an extract must confirm before any event is derived from it.
Göteborg's data cannot confirm it: it shows one operator per number today
and has no history.

## 6. Legal and process timing (VERIFIED SOURCE FACT)

| Step | Rule / observed practice |
| --- | --- |
| Application | written application to the municipality where the premises are (8 kap. 1, 10 §§); police statement required for permanent permits (8 kap. 11 §); suitability and knowledge test (8 kap. 12 §) |
| Decision deadline | within four months of a complete application (alkoholförordningen 5 §). Göteborg: "cirka fyra månader från att du skickat in din ansökan". Stockholm: "Handläggningstid: 3 månader", fee 12 590 SEK |
| Opening | food service may start once the food business is registered (Göteborg: register it only when you know when you start; unstarted registrations are removed after 3 months). Serving alcohol requires the permit (8 kap. 1 §). Holders must notify the municipality when they intend to start (9 kap. 11 §) |
| Takeover | a new legal operator needs a new permit. Stockholm's application requires proof that the purchase of the business has been paid ("kontoutdrag som visar att pengarna betalats ut … att säljaren fått pengarna"), i.e. the business is bought before the permit is decided |
| Changes | permanent changes to serving times, drinks, premises: application, Stockholm "från 2 veckor upp till 3 månader"; other changes: notification, "2–4 veckor" |
| Termination | bankruptcy ends the permit immediately (9 kap. 12 §); closure must be notified; permits may be revoked (9 kap. 18 §) |
| Register entry | municipalities register directly in ATR (KommunAdmin roles, "Registrera") or through the municipal web service; the law also requires decision copies to Folkhälsomyndigheten. No cadence is published. |
| From 2026-06-01 | the food requirement for serving permits is abolished (lag 2026:511), so more bar-type venues qualify |

## 7. Freshness

VERIFIED SOURCE FACT: ATR holds future permits ("Annu inte giltigt") and
municipalities can register directly, so the register can precede the
valid-from date. Göteborg's open file is refreshed daily. Nothing states
how long a municipality takes to register a decision.

COMMERCIAL INFERENCE: for municipalities using the web service or direct
registration, a permit is visible within days of the decision; for others
the delay is unknown. Because permits are granted 3–4 months after
application and shortly before or after opening, the observable event
lands at the *opening*, not at the *decision to open*.

## 8. Volume (VERIFIED SOURCE FACT, Folkhälsodata, permanent permits at year end)

| Region | 2019 | 2021 | 2023 | 2024 | 2025 |
| --- | --- | --- | --- | --- | --- |
| Sweden | 15 709 | 16 377 | 16 924 | 16 953 | 17 053 |
| Stockholm | 2 273 | 2 370 | 2 490 | 2 500 | 2 524 |
| Göteborg | 918 | 965 | 1 030 | 1 045 | 1 065 |
| Malmö | 482 | 498 | 559 | 581 | 588 |

Net growth is about 100 permits per year nationally. Gross new permits,
operator changes and terminations per month are **not published**; only
ATR can show them. COMMERCIAL INFERENCE: hospitality churn is high, so
gross new permanent permits are plausibly in the low thousands per year
nationally, but this is unmeasured.

## 9. Legal, licensing and privacy (VERIFIED SOURCE FACT)

* Access for companies is an explicit design of ATR, granted after
  secrecy review; extracts are Excel; a web-service role exists. Terms,
  fees and permitted uses are not published and must be asked.
* Sole traders are identified by personal identity number in the register
  (13 kap. 2 §) and appear under personal names in Göteborg's data
  (about 5 % of rows). Any extract will contain personal data for them.
* Göteborg's dataset is CC0.

## 10. Technical feasibility (COMMERCIAL INFERENCE)

If a registerutdrag account is granted: a snapshot adapter (Excel via the
web service or manual download, dataset-level provenance as for
Klimatklivet, rows keyed on permit id or on restaurangnummer plus
organisation number, versioned history if the extract carries it). With
"Inkludera framtida" and "Inkludera avslutade" available in the register,
explicit `SERVING_ESTABLISHMENT_REGISTERED`, `…_OPERATOR_CHANGED` and
`SERVING_PERMIT_TERMINATED` events could be derived from state
comparison. Organisation numbers would flow through the existing
normalisation; the restaurangnummer would sit in `source_event_id` and the
payload until a second workplace identifier scheme is justified.

Göteborg alone: a daily snapshot diff on restaurangnummer would detect
new venues and operator-name changes for one city (about 1 % of national
permits per year net, gross unknown), with operator identity by name only.
Useful as a calibration stream, not as the product.

## 11. Likely commercial usefulness (COMMERCIAL INFERENCE)

* Event kinds the source can support: new permanent permit at a new
  restaurangnummer (new location or first permit at a location), new
  holder at an existing restaurangnummer (operator change, if the number
  is reused), termination. Temporary permits and closed-company permits
  are separable by type.
* Timing: the permit is decided 3–4 months after an application that
  already presupposes a lease, a financed purchase of the business and a
  food registration. One-time opening purchases (fit-out, kitchen,
  furniture, POS hardware) are therefore mostly LIKELY_SELECTED at
  permit time. Recurring services (cleaning, pest control, waste,
  accounting and payroll, IT support, telecom, staffing, insurance, linen,
  beverage supply) are plausibly MIXED to LIKELY_OPEN, especially for
  operator changes where contracts are renegotiated. This is the same
  "recurring-service customer" reading that came out of the IVO note, and
  it is untested.
* Information edge: openings are visible to local suppliers, on Google
  Maps, in hospitality media, and Göteborg even publishes a public map.
  The edge, if any, is a national, organisation-number-keyed feed of
  operator changes, which no public source offers today.

## 12. Re-entry path

1. Apply for an ATR account under "Jag representerar ett företag som
   behöver upplysning om vilka företag som har serveringstillstånd", state
   the purpose honestly (B2B market intelligence for hospitality
   suppliers), request both current and terminated permits, and ask for
   the web-service role, terms, fees and refresh cadence. This is an
   outbound application the team must send; nothing was submitted.
2. On receipt of a first extract, check: permit id and restaurangnummer
   present; organisation number present; valid-from and status present;
   whether a new holder at an existing venue keeps the restaurangnummer;
   whether version history is included; the share of sole traders.
3. Only then build `internal/source/atr` as a snapshot adapter with the
   tests the task specifies, and start the Göteborg daily diff as a
   secondary check on freshness.
4. In parallel, at zero cost, begin archiving Göteborg's CSV daily so a
   diff history exists when the adapter is built.

Draft application purpose text (Swedish):

> Vi utvecklar en tjänst som hjälper leverantörer till restaurang- och
> hotellbranschen (t.ex. städ, skadedjur, avfall, kassasystem,
> redovisning, IT och telefoni) att identifiera nyöppnade serveringsställen
> och byten av tillståndshavare. Vi önskar utdrag över samtliga aktuella
> stadigvarande serveringstillstånd samt tillstånd som avslutats de senaste
> tolv månaderna, med organisationsnummer, tillståndshavarens namn,
> restaurangnummer, serveringsställets namn och adress, kommun,
> tillståndstyp, giltig från och till samt status. Vi vill även veta om
> uttag kan göras återkommande via webbtjänsten för registerutdrag, vilka
> villkor som gäller för användningen och vad det kostar.
