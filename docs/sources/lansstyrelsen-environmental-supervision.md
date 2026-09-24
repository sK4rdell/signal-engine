# Source: Länsstyrelsernas environmental supervision (public diary)

Research note for the fifth Signal Engine experiment: environmental
supervision by the 21 county administrative boards (Länsstyrelsen) as a
buying-need signal. Everything marked VERIFIED SOURCE FACT was observed on
2026-09-24 with plain unauthenticated HTTP against
`https://diarium.lansstyrelsen.se/` (no CAPTCHA, login or anti-bot
mechanism exists on the site; nothing was bypassed). Everything marked
COMMERCIAL INFERENCE is our reading of the data. The empirical evaluation
and the implementation decision are in
`lansstyrelsen-environmental-supervision-evaluation.md`; the scored sample
is `lansstyrelsen-environmental-supervision-sample.csv`.

## Verdict: technically easy, commercially thin — DO_NOT_IMPLEMENT for now

VERIFIED SOURCE FACT: the diary is fully scrapeable with plain HTTP, has
stable case identifiers, handling-level rows and roughly six months of
history. COMMERCIAL INFERENCE: after classifying 1,396 handlings in 474
supervision-type cases (drawn from 2,250 environmental-unit cases across
nine counties, 1 Jul–24 Sep 2026), only 8 handlings (1.3 % of candidate
handlings, 0.4 % of all sampled cases) meet the HIGH bar of "named buyer +
concrete problem + mandatory action + external supplier + timely". The
task's stop gate (< 10 % HIGH) is hit by a wide margin, so no adapter was
built. See the evaluation note for the re-entry conditions.

## 1. What the diary is (VERIFIED SOURCE FACT)

* One ASP.NET WebForms application serves the public diaries of all 21
  Länsstyrelser. The diary is a mandatory search filter (`ddDiaryID`):
  2 Stockholm, 3 Uppsala, 4 Södermanland, 5 Östergötland, 6 Jönköping,
  7 Kronoberg, 8 Gotland, 9 Blekinge, 10 Skåne, 11 Halland,
  12 Västra Götaland, 13 Värmland, 14 Örebro, 15 Västmanland, 16 Dalarna,
  17 Gävleborg, 18 Västernorrland, 19 Jämtland, 20 Västerbotten,
  21 Norrbotten, 22 Kalmar.
* Each case has a numeric `caseID` (stable, used in
  `Case/CaseInfo.aspx?caseID=<int>`), a diary number `<serial>-<year>`
  (unique only within a county), status (Handläggning / Beslutat /
  Avslutat), received date, title, sender/recipient, postal town,
  municipality and decision date.
* Each case page lists its handlings in `SearchPlaceHolder_deedGridView`
  with four columns: handling number `<serial>-<year>-<n>`, handling
  title, in/out date, sender/recipient. There is no separate handling id
  and no link to any document; **no document text is readable**.
* Private persons are masked as "Personuppgift" in sender/recipient
  fields (both at case and handling level). Some handlings are
  "Sekretessmarkerad". Companies, municipalities and authorities are shown
  in clear text, but the case-level sender is often "Personuppgift" even
  when the operator is a company (the officer's name was the contact).

## 2. Search mechanics (VERIFIED SOURCE FACT)

* Landing page `Default.aspx` is a WebForms form; every search is a POST
  with `__VIEWSTATE`/`__EVENTVALIDATION` plus the filter fields
  (`ddDiaryID`, `diaryNO`, `title`, `ddlStatus`, `ddlOrgUnit`,
  `ddMunicipality`, received-date range, decision-date range).
* Title search is a case-insensitive substring match.
* The organisation-unit dropdown is populated per diary by a postback
  (`btnUpdateOrgUnit`). Posting a unit id without first loading that list
  returns HTTP 500; after loading it, filtering by unit works. Units used
  for the sample: Stockholm 631 (miljöskydd) and 1174 (mark- och
  vattenskydd); Skåne 769 (miljötillsyn) and 1526 (förorenade områden);
  Västra Götaland 1579 (industritillsyn) and 1578 (förorenade områden);
  Östergötland 1628; Uppsala 698 and 1142; Dalarna 1158; Västernorrland
  1317; Jönköping 1248 and 1387; Halland 511.
* Results land on `Case/CaseSearchResult.aspx?query=<opaque token>`, 50
  rows per page, paged by posting `__EVENTTARGET=ctl00$SearchPlaceHolder$caseGridView`,
  `__EVENTARGUMENT=Page$N` to the result URL. The result URL is stateless
  (re-GET works without cookies).
* Searching by decision date alone returned no rows in our tests;
  received-date windows work.
* A case page is a plain GET, about 0.15 s. Two of 476 requested case
  pages returned HTTP 500 (caseID 5553271 and 5553291) and were skipped.
* No `robots.txt`, no terms of use and no rate limiting were observed; we
  used a 0.8 s pause between requests and an identifying User-Agent.

## 3. Retention (VERIFIED SOURCE FACT)

Closed cases disappear from the public diary roughly six months after
their decision date: on 2026-09-24 the earliest visible decision date was
2026-03-25, and cases with earlier decisions were not returned by any
filter. Cases in Handläggning stay visible. A production ingester would
therefore have to observe every case within six months of its decision
and keep its own history; there is no archive.

## 4. Who sends what (VERIFIED SOURCE FACT from the sample)

Handlings fall into a small vocabulary. Counts over the 1,396 handlings of
the 474 supervision-type cases:

| Handling family (our label) | Typical titles | Count |
| --- | --- | --- |
| Operator/other communication | Komplettering, Yttrande, Meddelande om …, Svar | 346 |
| Administrative | Start av ärende, Tjänsteanteckning, Delgivning, Bekräftelse | 283 |
| Incident report | Anmälan om driftstörning …, spill, läckage, överskridande | 123 |
| Control programme | Förslag till kontrollprogram, provtagningsplan | 88 |
| Visit scheduled | Planerat tillsynsbesök, Dagordning, Kallelse | 71 |
| Information request | Begäran om komplettering / uppgifter / yttrande | 67 |
| Complaint | Klagomål på buller/lukt, Begäran om tillsyn | 66 |
| State-funded PFAS sampling | Verifierande provtagning, Avrop, PFAS-undersökning på er fastighet | 60 |
| Inspection report | Tillsynsrapport efter tillsynsbesök | 44 |
| Case closed | Beslut att avsluta utan åtgärd, Avskrivning | 42 |
| Transboundary waste shipment control | Tillsyn av gränsöverskridande transport av avfall | 42 |
| Remediation notified (28 §) | Anmälan om avhjälpandeåtgärder / efterbehandling / PCB-sanering | 33 |
| Fee decision | Avgift för prövning och tillsyn | 30 |
| Report submitted | Rapport om …, Redovisning av …, Årlig information | 25 |
| Seveso document | Handlingsprogram för Sevesoverksamheten | 14 |
| Investigation required | Underrättelse om behov av inventering av PFAS, Utredningsprogram, behov av kompletterande markundersökning | 13 |
| Enforcement order | Föreläggande om …, Beslut om förbud | 11 |
| Permit-condition follow-up | villkor, prövotid, försiktighetsmått | 8 |
| Grant application | Ansökan om bidrag till undersökning/efterbehandling | 7 |
| Consultation with party | Underrättelse med möjlighet till yttrande | 6 |
| Sanction fee | Beslut om miljösanktionsavgift | 5 |
| Contamination discovered | Underrättelse om upptäckt av påträffad förorening | 3 |
| Order proposed | Förslag till föreläggande | 2 |
| Unclassified | | 7 |

COMMERCIAL INFERENCE: the formal enforcement instruments that the task
hoped to find (föreläggande, förbud, order to investigate) are rare — 26
handlings in twelve weeks across nine counties — and most supervision
activity is planned visits whose findings live in documents we cannot
read.

## 5. Identity (VERIFIED SOURCE FACT)

No organisation numbers anywhere. The operator can be read from the
case-level sender (when not masked), from handling senders, or from the
case title ("Planerat tillsynsbesök på Ovako Bar AB, Boxholms kommun").
Over the 621 candidate handlings, 77 % had a company, municipality or
other named organisation resolvable this way; 17 % had only "Personuppgift"
and a generic title; 6 % named a facility but not its operator
("Slottshagens avloppsreningsverk"). Resolving names to organisation
numbers would need a separate registry lookup; nothing was guessed.

## 6. Access and etiquette

Public, unauthenticated, no terms. A polite crawler is feasible: 21 diary
searches per day per window plus one GET per new or changed case. Because
handlings are only visible on the case page, incremental updates require
re-fetching open cases (72 % of the sampled supervision cases were still in
Handläggning after twelve weeks).

## 7. If this is ever re-entered (design sketch, not built)

* Package `internal/source/lansstyrelsen`; source name `lansstyrelsen`.
* Observation identity = case page per county + `caseID` (payload = case
  header + handling rows; content hash changes when a handling is added).
* Event identity = `<county>:<case number>:<handling number>`; one event
  per handling in a trigger family, event types kept neutral
  (`ENVIRONMENTAL_ENFORCEMENT_ORDER`, `ENVIRONMENTAL_INVESTIGATION_REQUIRED`,
  `ENVIRONMENTAL_SUPERVISION_ACTION`), provenance = county, diary number,
  handling number, case URL, first observed at.
* Retention forces a "keep what you saw" model: events must never be
  deleted when the case drops out of the diary.
