# Evaluation: Länsstyrelsen environmental supervision as a buying signal

Companion to `lansstyrelsen-environmental-supervision.md` (technical
facts). Data: 2,250 cases from the environmental units of nine counties
(received 1 Jul–24 Sep 2026), of which 474 supervision-type cases with
1,396 handlings were fetched in full. Scored rows are in
`lansstyrelsen-environmental-supervision-sample.csv` (621 candidate
handlings). Everything below marked VERIFIED is counted from that data;
everything marked INFERENCE is our judgement.

## 1. Sample (VERIFIED)

| County | Cases (env. units, 12 weeks) | Supervision-type share |
| --- | --- | --- |
| Stockholm | 438 | 22 % |
| Västra Götaland | 310 | 55 % |
| Skåne | 281 | 49 % |
| Halland | 242 | 14 % |
| Jönköping | 236 | 37 % |
| Uppsala | 218 | 9 % |
| Västernorrland | 207 | 39 % |
| Östergötland | 179 | 40 % |
| Dalarna | 139 | 43 % |

Weekly intake across the nine counties ranged from 63 (week 31, summer)
to 260 (week 36). The unit filter is the useful selector: it excludes
nature, hunting, agriculture and planning diaries, but the environmental
units still carry a lot of non-supervision work.

## 2. Case-kind composition of the 2,250 cases (VERIFIED, title-rule based)

| Kind | Share |
| --- | --- |
| Permit application / notification (ansökan, anmälan om vattenverksamhet, samråd) | 20.2 % |
| Environmental incident (anmälan om driftstörning, spill, läckage) | 16.0 % |
| Consultation / remiss / yttrande | 14.8 % |
| Direct supervision (planerat tillsynsbesök, tillsyn av …, kontrollprogram) | 12.0 % |
| Unknown | 9.8 % |
| Other administrative | 7.6 % |
| Contaminated-site supervision (förorenade områden, PFAS, avhjälpande) | 4.4 % |
| Municipal C-activity information copies | 4.2 % |
| Private complaints (Personuppgift complainant, private targets) | 2.9 % |
| Routine reporting (miljörapport, köldmedier, årsrapport) | 2.7 % |
| Permit change / condition change | 2.4 % |
| Appeals | 1.9 % |
| Complaints against operators | 1.2 % |

Supervision in the sense of the task (direct + contaminated-site +
incidents + complaints) is about a third of environmental-unit intake.

## 3. Trigger families and problem domains (VERIFIED)

The handling-level families are listed in the source note. Problem
domains were derived from case and handling titles. Over the 170 HIGH or
MEDIUM handlings the domains are: contaminated land 39, spills and leaks
26, chemicals / Seveso 23, noise 22, general supervision (no stated focus)
16, unknown 12, wastewater 7, air emissions 6, waste 4, stormwater 3,
monitoring 3, fire 3, water activity 3, PFAS 2, permit conditions 1.

## 4. Scoring rules (INFERENCE)

* `mandatory_action_likelihood`: HIGH for orders, prohibitions,
  investigation demands, contamination discoveries, 28 § notifications;
  MEDIUM for inspection reports, incidents, information requests, control
  programmes, Seveso reviews; LOW for scheduled visits, complaints, grants,
  state-funded sampling.
* `external_purchase_likelihood`: HIGH when the demanded work is
  investigation / sampling plan / remediation / permit application;
  MEDIUM for inspection findings (unknown), noise complaints, spills to
  ground, equipment failures, emission exceedances; LOW for paperwork.
* `timing_quality`: EARLY (visit scheduled, investigation demanded before
  any consultant appears), TIMELY (order just issued, incident just
  reported), LATE (28 § remediation notifications, control programmes
  already prepared, consultant already appears as sender, state-funded
  sampling where Länsstyrelsen itself is the buyer).
* `commercial_relevance`: HIGH = identifiable buyer + mandatory HIGH +
  purchase HIGH/MEDIUM + not LATE; MEDIUM = identifiable + (mandatory
  HIGH or purchase ≥ MEDIUM) + not LATE, or LATE with purchase HIGH;
  UNKNOWN = target not identifiable but some signal; LOW = rest.
* 39 rows were reviewed by hand and overridden where the rules were
  wrong (column `manual_notes` starts with "Reviewed."); the rest are
  rule-based.

## 5. Results over the 621 candidate handlings (VERIFIED)

| Tier | Handlings | Share | Cases (best handling) |
| --- | --- | --- | --- |
| HIGH | 8 | 1.3 % | 8 |
| MEDIUM | 162 | 26.1 % | 112 |
| LOW | 350 | 56.4 % | 170 |
| UNKNOWN | 101 | 16.3 % | 59 |

Relative to the whole 2,250-case sample, HIGH cases are 0.4 % and
HIGH + MEDIUM cases are 5.3 %.

Gates set by the task:

| Gate | Threshold | Observed | Result |
| --- | --- | --- | --- |
| HIGH share of candidates | > 25 % promising, < 10 % stop | 1.3 % | STOP |
| Metadata sufficient without document (HIGH + MEDIUM) | ≥ 50 % | 20 % sufficient, 64 % "document would improve", 16 % "document required" | FAIL |
| Buyer identifiable from metadata (all candidates) | ≥ 80 % | 77 % (100 % of HIGH + MEDIUM by construction) | MARGINAL |
| Timing (HIGH + MEDIUM early or timely) | most | 81 % (48 early, 90 timely, 32 late) | PASS |

The HIGH rows all have a named buyer and metadata that is sufficient or
nearly sufficient; the problem is volume, not quality.

## 6. What the HIGH rows look like (VERIFIED rows, INFERENCE on need)

| Handling | Date | Buyer | Handling title | Need |
| --- | --- | --- | --- | --- |
| 21315-2026-2 (Skåne) | 2026-07-02 | Ystad Hamn Logistik AB | Underrättelse om behov av inventering av PFAS | MIFO inventory by consultant; report delivered 2026-08-31 |
| 21326-2026-2 (Skåne) | 2026-07-02 | Trelleborgs Hamn AB | Underrättelse om behov av inventering av PFAS | Same; MIFO 1 delivered 2026-09-10 |
| 24178-2026-1 (Skåne) | 2026-08-12 | Skåne Blekinge Vattentjänst AB | Underrättelse om upptäckt av påträffad förorening | Investigation of newly found contamination |
| 5408-2026-2 (Jönköping) | 2026-07-01 | Nässjö Affärsverk | Anmälan om påträffad dieselförorening | Investigation and remediation |
| 5955-2026-4 (Jönköping) | 2026-09-14 | Sävsjö kommun | Meddelande om behov av kompletterande markundersökning | Follow-up soil investigation |
| 5554-2026-1 (Jönköping) | 2026-07-03 | Vetlanda kommun | Beslut om betydande miljöpåverkan samt föreläggande att söka tillstånd | Permit application and EIA |
| 5531-2026-2 (Halland) | 2026-08-21 | Halmstads kommun | Föreläggande om att vidta skyddsåtgärder … hantering av länsvatten | Water handling / treatment measures |
| 36859-2026-1 (Västra Götaland) | 2026-09-10 | Brenntag Nordic AB | Förbud att tvätta mer än 625 storbehållare | Capacity or permit change; reason unknown |

Representative MEDIUM and LOW rows:

* 13249-2026-4 Stena Recycling AB, "Tillsynsrapport efter tillsynsbesök"
  — findings unknown; document required (typical of 26 MEDIUM inspection
  reports).
* 28014-2026-1 Öresundskraft, "Förhöjd kvicksilverhalt i rökgaserna" —
  concrete, but a self-report handled through existing suppliers.
* 26593-2026-1 Skåne Blekinge Vattentjänst, "Haveri skrapspel" at Osby
  WWTP — a repair purchase, ordered before the diary entry.
* 23920-2026-1 Kemira Kemi AB, noise complaint; Länsstyrelsen asked for a
  statement — a measurement demand may follow, nothing more yet.
* 6142-2026-4 Boliden Mineral AB, "Föreläggande om försiktighetsmått vid
  schaktarbeten" — an order, but on a 28 § remediation already designed
  and contracted (LATE).
* 24913-2026-2 FAAC Nordic AB, "PFAS-undersökning på er fastighet" —
  Länsstyrelsen is the buyer (Sweco called off under a framework two
  weeks later); the landowner buys nothing.
* 36534-2026-3 Vattenfall Värme, "Beslut om avgifter för prövning och
  tillsyn" — pure fee administration (30 such handlings).
* 40639-2026-10 "Beslut om exportförbud" — transboundary waste control of
  small traders, no supplier need.

## 7. Supplier verticals (INFERENCE)

| Vertical | Trigger families that feed it | Buyers in sample | Firms in Sweden |
| --- | --- | --- | --- |
| Environmental consultants, contaminated land (investigation, sampling plans, risk assessment, remediation design) | investigation required, contamination discovered, orders outside 28 § cases, municipal grant applications | ports, VA companies, municipalities, industrial owners | HUNDREDS (large multi-disciplinary firms plus many small) |
| Remediation contractors | 28 § notifications (already contracted), discoveries | industry, municipalities | HUNDREDS |
| Acoustic consultants | noise complaints, noise reports | industry, ports, wind farms | TENS to HUNDREDS |
| Accredited emission measurement / flue-gas treatment | emission exceedance incidents, control programmes | energy and process plants | TENS |
| Wastewater process and equipment service | WWTP incidents, control programmes | VA companies | HUNDREDS |
| Seveso / process-safety consultants | Seveso visits, handlingsprogram reviews | chemical, fuel and food industry | TENS |
| Environmental law | orders, sanction fees, permit orders | all | TENS |

Best vertical: contaminated-land consultants. It receives 5 of the 8
HIGH rows and 39 of the 170 HIGH + MEDIUM rows, buyers are named, and the
window between notice and report was about eight weeks in the two port
PFAS cases. The weakness is volume: about four strong triggers per twelve
weeks in nine counties.

## 8. Volume (VERIFIED sample, INFERENCE extrapolation)

Per week across the nine sampled counties: 52 candidate handlings, 0.7
HIGH handlings, 13.5 MEDIUM handlings, 10 cases with a HIGH or MEDIUM
handling. The nine counties include the three largest industrial regions
(Stockholm, Västra Götaland, Skåne), so a linear scale-up to 21 counties
overstates; a plausible national range is 1–2 HIGH handlings and 15–25
HIGH-or-MEDIUM cases per week. Complaint clusters (one Nouryon odour case
produced 15 complaint handlings) inflate handling counts; case counts are
the honest unit.

## 9. Document experiment (drafted, not sent)

Fifteen handlings where the document would change the assessment. Each is
a public record that can be requested from the county registrar; no
request was sent.

| # | Handling | County | Buyer | Why the document matters |
| --- | --- | --- | --- | --- |
| 1 | 13249-2026-4 Tillsynsrapport efter tillsynsbesök | Östergötland | Stena Recycling AB | Deficiencies and demands unknown |
| 2 | 14148-2026-6 Tillsynsrapport efter tillsynsbesök | Östergötland | Lantmännen Biorefineries AB | Same |
| 3 | 6129-2026-2 Tillsynsrapport efter tillsynsbesök | Jönköping | Husqvarna AB | Same (IED plant) |
| 4 | 6706-2026-5 Tillsynsrapport efter tillsynsbesök | Dalarna | Bergkvist Siljan Blyberg AB | Same (sawmill; waste/by-product question raised) |
| 5 | 41484-2026-3 Tillsynsrapport | Stockholm | Stockholms Bulkhamn AB | Event-driven visit; what triggered it |
| 6 | 13794-2026-1 Beslut om miljösanktionsavgift | Östergötland | Ovako Bar AB | Which violation |
| 7 | 36859-2026-1 Förbud att tvätta mer än 625 storbehållare | Västra Götaland | Brenntag Nordic AB | Why the cap, what would lift it |
| 8 | 5531-2026-2 Föreläggande om att vidta skyddsåtgärder | Halland | Halmstads kommun | Which measures |
| 9 | 24178-2026-1 Underrättelse om upptäckt av påträffad förorening | Skåne | Skåne Blekinge Vattentjänst AB | What and where |
| 10 | 5955-2026-4 Behov av kompletterande markundersökning | Jönköping | Sävsjö kommun | Scope of the follow-up investigation |
| 11 | 46914-2026-1 Utredningsprogram, fosfor i dagvatten | Stockholm | Swedavia AB | Whether external work is required |
| 12 | 28014-2026-1 Förhöjd kvicksilverhalt i rökgaserna | Skåne | Öresundskraft Kraft & Värme AB | Cause and corrective action |
| 13 | 26593-2026-1 Haveri skrapspel, Osby reningsverk | Skåne | Skåne Blekinge Vattentjänst AB | Whether repair or replacement |
| 14 | 23920-2026-2 Begäran om yttrande (bullerklagomål) | Skåne | Kemira Kemi AB | Whether measurements are demanded |
| 15 | 13246-2026-5 Föreläggande om att lämna in uppgifter | Östergötland | Swed Handling AB | What the Seveso inspection found |

Draft request (Swedish, one per county, to the county's registrar
address; recipients to be confirmed before any sending):

```text
Ämne: Begäran om allmän handling – dnr <diarienummer>

Hej,

Med stöd av tryckfrihetsförordningen 2 kap. begär jag kopior av följande
handlingar i ert diarium:

- <diarienummer-handlingsnummer>, "<handlingsrubrik>", <datum>
- …

Handlingarna önskas digitalt (PDF) till denna e-postadress. Om någon
uppgift omfattas av sekretess ber jag att få handlingen med den
uppgiften maskerad. Meddela gärna i förväg om avgift tillkommer.

Vänliga hälsningar,
<namn>
```

## 10. Decision: DO_NOT_IMPLEMENT (INFERENCE)

* The HIGH gate fails by a wide margin (1.3 % against a 10 % stop
  threshold). MEDIUM rows are mostly inspection reports whose content we
  cannot read, self-reported incidents already being handled, 28 §
  notifications that arrive after the consultant and contractor are
  engaged, and complaints that may never produce a demand.
* Metadata alone is sufficient for one in five HIGH + MEDIUM rows.
  Without document access the source is a list of "something happened
  at company X", which is what a sales team already gets from news and
  from the operators' own environmental reports.
* No adapter, migration, config or CLI was added. `internal/source/`
  still contains only `arbetsmiljoverket` on this branch.

Re-entry conditions, any one of which would justify a second look:

1. Cheap document access at scale (a county e-service for handlingar, or
   an agreement to receive tillsynsrapporter and förelägganden in bulk).
   Then the 26 inspection reports per twelve weeks and the enforcement
   orders become readable triggers and the metadata gate no longer
   applies.
2. A product focus on contaminated-land consultants, where a narrow feed
   (families investigation required, contamination discovered,
   enforcement orders outside 28 § cases, municipal grant applications;
   ~1–2 per week nationally) complements a higher-volume source. The
   cheapest form is 21 daily title searches ("behov av inventering",
   "påträffad förorening", "föreläggande", "utredningsprogram") rather
   than crawling every case.
3. Evidence from the document experiment (section 9) that inspection
   reports routinely contain concrete demands with deadlines; that would
   lift the inspection-report family from MEDIUM to HIGH and change the
   volume picture (about 4 per week in nine counties).
