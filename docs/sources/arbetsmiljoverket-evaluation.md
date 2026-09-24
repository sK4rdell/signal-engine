# Arbetsmiljöverket inspection notices: evaluation of the first 137 events

Evaluation of the real `Inspektionsmeddelande` events ingested on
2026-09-24 for document dates 2026-09-21 to 2026-09-23 (see
[arbetsmiljoverket.md](arbetsmiljoverket.md) for how the source works).
The companion sheet
[arbetsmiljoverket-evaluation-sheet.csv](arbetsmiljoverket-evaluation-sheet.csv)
lists every event with the derived fields below and empty columns for
manual scoring.

Everything under "commercial relevance" is a **hypothesis derived from the
case title alone**. The source does not expose the deficiency text (the
notice must be ordered by email), so the title, which names the inspection
campaign or the triggering incident, is the only content we have. Nothing
in this report is stored in the database; the public events remain pure
source facts.

## 1. Data basis

| | |
| --- | --- |
| Events (all `Inspektionsmeddelande`, all `Bedriva inspektion`) | 137 |
| Distinct cases | 136 |
| Document dates | 2026-09-21 (34), 2026-09-22 (33), 2026-09-23 (70) |
| Organisation number and CFAR both present | 132 (96 %) |
| CFAR only (organisation withheld by the source) | 4 |
| Neither identifier | 1 |
| Workplace name differs from organisation name | 33 (24 %) |
| Case status `Pågående` | 136 |

## 2. Method

Each case title was mapped to a **campaign family** with a rule table
(regular expression on the title; incident-triggered titles of the form
`Olycka YYYYMMDD <type>` / `Tillbud YYYYMMDD <type>` are split by incident
type). Each family carries three judgements:

* **External purchase likelihood** (the tier): how likely remediation of a
  deficiency found under that theme requires buying a product or service
  from an outside supplier. `HIGH` = the theme almost always implies an
  external category (measurement, certified inspection, equipment,
  training). `MEDIUM` = often, but the category is broad or remediation may
  be internal. `LOW` = mostly organisational or administrative, or the
  inspected party is a poor buyer (work-life crime controls, public-sector
  organisational themes). `UNKNOWN` = the title says nothing.
* **Specificity**: how much the title narrows down the actual deficiency.
* **Supplier categories**: the plausible sellers for that theme.

The tiers are priors, not findings. A *Kvarts och kemi* inspection may
have found only a missing risk assessment; a *Riskmiljöer* inspection may
have found a missing machine guard. Section 8 says how to verify.

## 3. Campaign / title distribution

| Events | Share | Campaign family (from case title) | Tier | Specificity | Plausible supplier categories |
| --- | --- | --- | --- | --- | --- |
| 20 | 15 % | Fortlöpande tillsyn - Riskmiljöer (high-risk environments) | MEDIUM | low | Broad: machine safety, chemicals, PPE, fall protection, consulting |
| 11 | 8 % | Fortlöpande tillsyn - Unga i arbetslivet (young workers) | LOW | low | Mostly internal; e-learning/introduction training |
| 11 | 8 % | Myndighetsgemensamma kontroller (joint agency controls, work-life crime) | LOW | low | Varies; poor buyer profile |
| 10 | 7 % | Belastningsergonomi i arbetslivet (ergonomics) | MEDIUM | medium | Ergonomist / occupational health service; lifting aids; adjustable workstations |
| 10 | 7 % | Kvarts och kemi inom markarbete och industri | HIGH | high | Occupational hygiene measurement; ventilation/dust extraction; respiratory PPE; occupational health (medical checks) |
| 7 | 5 % | Systematiken i byggprojektets arbetsmiljöarbete | MEDIUM | medium | Bas-P/Bas-U training and certification; construction H&S consulting |
| 6 | 4 % | Arbete och säkerhet intill väg (roadwork safety) | HIGH | high | Traffic safety equipment (TMA, barriers, signage) rental; TA-plan services; Arbete på väg training |
| 6 | 4 % | Fortlöpande tillsyn SAM i mindre företag | MEDIUM | low | SAM consulting; digital SAM tools; occupational health service subscription |
| 5 | 4 % | Truck (forklift), announced and unannounced | HIGH | high | Forklift training providers; equipment maintenance/inspection; floor marking/barriers |
| 4 | 3 % | EMPACT (EU action against labour exploitation) | LOW | low | Varies; poor buyer profile |
| 4 | 3 % | Ett hälsosamt arbetsliv – arbetsanpassning | LOW | low | Mostly internal; occupational health service |
| 4 | 3 % | Ett hållbart arbetsliv i välfärden - hemtjänst | LOW | low | Mostly internal (public sector); organisational consulting |
| 4 | 3 % | Fall kopplat till arbetsutrustning/ställningar/hissar | HIGH | high | Fall protection equipment; scaffolding contractors/inspection; lift inspection; training |
| 4 | 3 % | Olycka-triggered inspection: Fysiskt våld | MEDIUM | high | Security consulting; personal alarms; conflict-management training |
| 3 | 2 % | Ett hållbart arbetsliv – pilot förskolan | LOW | low | Mostly internal (public sector) |
| 3 | 2 % | Omedelbart förbud (immediate prohibition) | HIGH | very high | Named by hazard: chemical handling/storage, fall protection, safety consulting |
| 3 | 2 % | Övrig tillsyn (other supervision) | UNKNOWN | low | Unknown until content is read |
| 2 | 1 % | Fall från samma nivå (slips and trips) | MEDIUM | medium | Anti-slip flooring/mats; lighting; minor works |
| 2 | 1 % | Olycka-triggered inspection: Maskin eller transportanordning | HIGH | high | Machine guarding; machine safety consulting (risk assessment); maintenance |
| 2 | 1 % | Olycka-triggered inspection: Person föll | HIGH | high | Fall protection; access equipment; training |
| 2 | 1 % | Vibrationer - handhållna maskiner | HIGH | high | Vibration measurement; low-vibration tools; occupational health (medical checks) |
| 2 | 1 % | Återkommande besiktning (periodic inspection of equipment) | HIGH | very high | Accredited inspection body; equipment repair/replacement |
| 2 | 1 % | Övrig tillsyn – Asbest | HIGH | high | Asbestos analysis labs; certified removal contractors; asbestos training |
| 1 | 1 % | Hot och våld i myndighetsutövning | LOW | medium | Security consulting; personal alarms; training (public sector) |
| 1 | 1 % | Olycka-triggered inspection: Fallande/flygande föremål | MEDIUM | high | Racking inspection; load securing; PPE |
| 1 | 1 % | Olycka-triggered inspection: Handhållet verktyg/föremål | MEDIUM | high | Tools/PPE; training |
| 1 | 1 % | Olycka-triggered inspection: Kemikalie | HIGH | high | Chemical handling consulting; storage; PPE; ventilation |
| 1 | 1 % | Säker Schaktning (excavation safety) | HIGH | high | Shoring/trench boxes (rental); geotechnical assessment; training |
| 1 | 1 % | Tillbud-triggered inspection: Elektricitet | HIGH | high | Certified electrical contractor; lockout equipment; training |
| 1 | 1 % | Tillbud-triggered inspection: Fallande/flygande föremål | MEDIUM | high | Racking inspection; load securing; PPE |
| 1 | 1 % | Tillbud-triggered inspection: Maskin eller transportanordning | HIGH | high | Machine guarding; machine safety consulting (risk assessment); maintenance |
| 1 | 1 % | Tips om misstänkt missförhållande | UNKNOWN | low | Unknown until content is read |
| 1 | 1 % | Utredningar av allvarliga händelser 33a | UNKNOWN | low | Unknown until content is read |

Observations:

* About 85 % of titles are national **campaigns** ("Inspektion inom …");
  14 (10 %) are **incident-triggered** (`Olycka`/`Tillbud`) and name
  the incident type, which is the most specific text in the dataset.
* The largest family, *Fortlöpande tillsyn - Riskmiljöer* (20 events),
  is also the least specific: high-risk workplaces of any kind.
* 15 events (11 %) come from work-life-crime controls
  (*Myndighetsgemensamma kontroller*, *EMPACT*). Four of the five events
  where the source withheld the organisation are in these families.

## 4. Commercial relevance (hypothesis)

| Tier | Events | Share |
| --- | --- | --- |
| HIGH | 42 | 31 % |
| MEDIUM | 52 | 38 % |
| LOW | 38 | 28 % |
| UNKNOWN | 5 | 4 % |

* HIGH excluding public-sector organisations: 41 (30 %).
* HIGH + MEDIUM: 94 (69 %).

Against the experiment's kill conditions (share clearly implying an
external buying need: below 10 % kill, 10–25 % narrow subvertical, above
25 % customer validation, above 40 % very promising): the title-based HIGH
tier lands at 31 %, in the "customer validation worthwhile" band,
**provided the theme-level priors survive contact with the actual
documents**. If only half of the HIGH tier turns out to describe a
concrete external need, the signal drops into the "narrow subvertical"
band. That is the single most important thing to verify next.

## 5. Recurring supplier categories

| Supplier category (hypothesis) | Events | HIGH | MEDIUM | LOW/UNKNOWN |
| --- | --- | --- | --- | --- |
| SAM consulting, digital SAM tools, occupational health subscription | 21 | 0 | 6 | 15 |
| Broad (machine, chemical, fall, SAM); unknown until content is read | 20 | 0 | 20 | 0 |
| Work-life crime controls; poor buyer profile | 15 | 0 | 0 | 15 |
| Occupational hygiene measurement, dust control, occupational health | 12 | 12 | 0 | 0 |
| Ergonomics services and lifting aids | 10 | 0 | 10 | 0 |
| Organisational (public/care sector); little external purchase | 7 | 0 | 0 | 7 |
| Construction H&S coordination (Bas-P/Bas-U training, consulting) | 7 | 0 | 7 | 0 |
| Fall protection, scaffolding, access equipment | 6 | 6 | 0 | 0 |
| Roadwork traffic safety (equipment rental, TA-plans, training) | 6 | 6 | 0 | 0 |
| Security, alarms, conflict training | 5 | 0 | 4 | 1 |
| Unknown until content is read | 5 | 0 | 0 | 5 |
| Forklift training, equipment inspection, traffic separation | 5 | 5 | 0 | 0 |
| Machine safety (guarding, risk assessment, maintenance) | 3 | 3 | 0 | 0 |
| Acute remediation named in title (chemicals x2, fall risk x1) | 3 | 3 | 0 | 0 |
| Accredited periodic inspection of equipment, repair | 2 | 2 | 0 | 0 |
| Storage/racking, load securing, PPE | 2 | 0 | 2 | 0 |
| Asbestos analysis, certified removal, training | 2 | 2 | 0 | 0 |
| Slip/trip remediation (flooring, housekeeping, lighting) | 2 | 0 | 2 | 0 |
| Tools, PPE, training | 1 | 0 | 1 | 0 |
| Electrical safety (certified contractor, lockout, training) | 1 | 1 | 0 | 0 |
| Excavation shoring equipment, training | 1 | 1 | 0 | 0 |
| Chemical handling (storage, ventilation, PPE, consulting) | 1 | 1 | 0 | 0 |

Recurring, specific and external: occupational hygiene measurement and
occupational health (silica, chemicals, vibration), fall protection and
access equipment, roadwork traffic safety, forklift training, and machine
safety. These five account for 32 events (23 %). Ergonomics
(10) and construction coordination
(7) recur but are broader.

## 6. Who receives the notices

Sector guessed from the organisation name (rule-based, so "unclassified"
stays large; it is a guess, not SNI data):

| Sector (name-based guess) | Events | HIGH | MEDIUM | LOW | UNKNOWN |
| --- | --- | --- | --- | --- | --- |
| Construction / civil / installation | 30 | 16 | 9 | 4 | 1 |
| Unclassified (name gives no hint) | 30 | 4 | 10 | 14 | 2 |
| Manufacturing / industry | 17 | 7 | 7 | 3 | 0 |
| Automotive / vehicle service | 15 | 5 | 6 | 3 | 1 |
| Public sector | 14 | 1 | 9 | 3 | 1 |
| Hospitality / food service | 10 | 0 | 4 | 6 | 0 |
| Care / social services | 8 | 0 | 3 | 5 | 0 |
| Gardening / property service | 5 | 4 | 1 | 0 | 0 |
| Transport / logistics | 4 | 4 | 0 | 0 | 0 |
| Retail / wholesale | 4 | 1 | 3 | 0 | 0 |

* Public sector: 14 events (10 %), buying through procurement, mostly
  organisational themes (ergonomics in care, home care, preschools).
* 115 of 137 events concern aktiebolag; 3 concern other private legal forms.
* Repeat organisations within three days: MALMÖ KOMMUN, AVESTA KOMMUN,
  HÖGANÄS OMSORG AB, and ÖREBRO PROLACK AB (two notices in one immediate
  prohibition case).

## 7. The eight evaluation questions

1. **Clear business problem?** By title, 94 of 137 (69 %) name a theme
   where a deficiency is a concrete operational problem; 38 are
   organisational or poor-buyer controls; 5 say nothing.
   Unverified at document level.
2. **External supplier likely?** 42 (31 %) HIGH and 52 (38 %) MEDIUM, by theme.
3. **Recurring supplier categories?** Yes, see section 5. Occupational
   hygiene/health and fall protection recur across campaigns and incidents.
4. **How specific is the need?** Specific for 49 events (36 %):
   incident type, periodic-inspection object, immediate-prohibition hazard,
   or a narrow campaign. Low for 68 (50 %). Never at the level of
   "which machine, which guard".
5. **Early enough?** The notice is registered the day it is sent and is
   visible the next day. The employer normally has to report back what has
   been or will be done, so the buying window opens at receipt. Position of
   the notice in its case (running number of the document):

| Running number | Events | Share |
| --- | --- | --- |
| 2 | 42 | 31 % |
| 3 | 33 | 24 % |
| 4 | 24 | 18 % |
| 5 | 21 | 15 % |
| 6 | 7 | 5 % |
| 7 | 5 | 4 % |
| 8 | 3 | 2 % |
| 11 | 2 | 1 % |

   75 events (55 %) are the first or second registered document
   of their case, so the notice is the opening move. Incident-triggered
   notices arrive 10 to 111 days after the incident (median 28).
6. **Identify the organisation?** Yes for 132 (96 %): valid ten-digit
   organisation number, no sole traders in this sample. Withheld by the
   source on 5.
7. **Identify the workplace?** Yes for 136 (99 %) via CFAR; the workplace
   differs from the legal entity in 33 cases (branches, municipal units).
   Municipality is on the case page but not ingested yet.
8. **Explain "why now"?** Partially. "Arbetsmiljöverket inspected this
   workplace under campaign X on date D and found deficiencies" is always
   available; the deficiency itself is not. For incident-triggered and
   immediate-prohibition cases the title carries the reason.

## 8. Examples (real, companies and public bodies only)

| Document | Date | Organisation | Workplace | Case title | Tier | Plausible supplier |
| --- | --- | --- | --- | --- | --- | --- |
| 2026/045007-5 | 2026-09-23 | TRANSDEV SVERIGE AB | OMRÅDE NORRTÄLJE | Återkommande besiktning - Fordonslyft flerpelare | HIGH | Accredited inspection body; equipment repair/replacement |
| 2026/049324-5 | 2026-09-23 | KL INDUSTRI AB | KL INDUSTRI AB | Inspektion inom Ett säkert arbetsliv - Kvarts och kemi inom markarbete och industri | HIGH | Occupational hygiene measurement; ventilation/dust extraction; respiratory PPE; occupational health (medical checks) |
| 2026/057413-3 | 2026-09-23 | MICKE & KENT TRÄDGÅRD AKTIEBOLAG | MICKE & KENT TRÄDGÅRD AKTIEBOLAG | Inspektion inom Ett tryggt arbetsliv Arbete och säkerhet intill väg | HIGH | Traffic safety equipment (TMA, barriers, signage) rental; TA-plan services; Arbete på väg training |
| 2026/059590-4 | 2026-09-23 | PEAB ANLÄGGNING AB | PEAB ANLÄGGNING AB | Olycka 20260910 Maskin eller transportanordning | HIGH | Machine guarding; machine safety consulting (risk assessment); maintenance |
| 2026/061001-2 | 2026-09-23 | ÖE TERMINAL AKTIEBOLAG | ÖE TERMINAL AKTIEBOLAG | Inspektion inom Ett tryggt arbetsliv – truck (oanmäld) | HIGH | Forklift training providers; equipment maintenance/inspection; floor marking/barriers |
| 2026/059693-6 | 2026-09-22 | ÖREBRO PROLACK AB | ÖREBRO PROLACK AB | Inspektion 20260917 - Omedelbart förbud - Kemikalier - Singelvägen 16 , Örebro | HIGH | Named by hazard: chemical handling/storage, fall protection, safety consulting |

## 9. Recommendation and next step

The technical kill conditions are not met, and the product-side estimate
sits above the 25 % threshold with a wide uncertainty band. Proceed as
**CONTINUE_WITH_LIMITATIONS**, with one verification experiment before any
customer conversation:

1. Order about 25 notices through the diary cart (free by email),
   stratified over the HIGH families (silica/chemicals, roadwork, forklift,
   fall protection, incident-triggered machine/fall, periodic inspection,
   immediate prohibition) plus a few *Riskmiljöer* controls. Record the
   delivery lag and, per document, whether the deficiency names a
   purchasable remedy. This converts the priors above into measured rates
   and shows whether *Riskmiljöer*, the largest family, hides good signals.
2. If the HIGH rate holds, the first subverticals to validate with
   customers are occupational hygiene and occupational health providers
   (silica, chemicals, vibration: 12 events in three days) and
   fall-protection and access-equipment suppliers (6 events).
3. Enrich events with the case page (county and municipality) if
   geography matters to those suppliers; it costs one request per event.

Do not build a classifier on titles yet. The rule table in this report is
a reading aid for the evaluation, and the families will change when
Arbetsmiljöverket rotates its campaigns.
