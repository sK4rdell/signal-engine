# Source: IVO Vårdgivarregistret (healthcare provider register)

Research note for the third Signal Engine experiment: new healthcare
establishments registered with Inspektionen för vård och omsorg (IVO).
Everything marked VERIFIED SOURCE FACT was read on 2026-09-24 from ivo.se,
IVO's e-service pages, IVO's annual report 2025 (IVO 2026-09), the
ordinance 2010:1369, IVO's regulation HSLF-FS 2023:7 with amendments
2024:20 and 2026:8, and IVO's code list dated 2026-04-27. Everything
marked COMMERCIAL INFERENCE is our reading.

## Verdict: STOP at the access gate, with a cheap re-entry

VERIFIED SOURCE FACT: there is no public search service, no API, no
downloadable file and no open-data publication of Vårdgivarregistret. The
only official way for an outsider to obtain register content is a
request for public records to IVO's register function
(`registerfragor@ivo.se`), answered manually, priced per page under
avgiftsförordningen. The register itself is updated immediately when a
provider submits through IVO's e-service, and activities carry the status
"Ej startad" before they open, so the *register* is fresh; our
*observation* of it would only be as fresh as IVO's handling of a
recurring request, which is unverified.

No ingestion code was written: building an adapter around a format we
have not seen, or around rows copied by hand, would not be a source
integration. The re-entry condition is one successful extract (see
"Re-entry path"), which is a manual request the team must send.

## Register purpose and legal basis (VERIFIED SOURCE FACT)

* Vårdgivarregistret contains the activities under IVO's supervision
  according to patientsäkerhetslagen (2010:659), PSL. Its main purpose is
  for IVO to know the activities it supervises. IVO's annual report also
  describes the registers as "betydelsefulla informationskällor för andra
  samhällsaktörer som myndigheter, kommuner, regioner, företag, forskare,
  journalister och allmänheten".
* A provider can be a state authority, region, municipality, sole trader
  (enskild firma) or a legal person. Private persons are not providers,
  but sole traders are identified by personal identity number.
* One notification per activity led by a verksamhetschef (HSLF-FS 2023:7
  4 §). A provider with several clinics therefore has several register
  entries.

## Legal registration timing (VERIFIED SOURCE FACT)

| Event | Rule | Source |
| --- | --- | --- |
| New activity | notify **at the latest one month before** start, **at the earliest two months before** | PSL 2 kap. 1 §; HSLF-FS 2023:7 6 § |
| Change or move of a substantial part | notify **within one month after** it happened | PSL 2 kap. 2 § |
| Closure | notify within one month after closure | HSLF-FS 2023:7 7 § |
| Non-compliance | since 2023-07-01 IVO may order compliance under penalty of a fine (PSL 7 kap. 28 a §) | ivo.se news 2023-06-28 |
| Private dentistry from 2026-01-01 | no longer notifies; must apply for a **permit** (fee 30 000 SEK). Registration in Vårdgivarregistret happens automatically when the permit is granted. Existing providers already registered have until 2029-01-02 to apply; unregistered existing providers until 2027-01-01 | ivo.se "Tillstånd för privat tandvård"; HSLF-FS 2026:8 |

COMMERCIAL INFERENCE: for everything except private dentistry the law
puts the registration 1–2 months before opening, which is the window the
thesis needs. For private dentistry the register entry now follows a
permit decision of unknown duration, so the "one month before opening"
property is lost for the family with the strongest equipment purchases.

## Access mechanism (VERIFIED SOURCE FACT)

Investigated channels:

| Channel | Finding |
| --- | --- |
| Public searchable register | none. ivo.se has no search page for Vårdgivarregistret; the site map lists no such page; probed hostnames (`vardgivarregistret.ivo.se`, `registerservice.ivo.se`, `vgr.ivo.se`) do not resolve. Omsorgsregistret has no public search either. |
| Documented API | none found on ivo.se. IVO's PXWeb statistics database holds complaints, lex Maria/Sarah, initiative cases and unexecuted decisions, not the provider register. |
| Downloadable file / open data | none on ivo.se; nothing found through dataportal.se's search page. |
| Public diary | IVO has a diary for cases; register entries are not diary documents. The "Begär ut allmän handling" e-service is for case documents. |
| Public-records request | **yes**: "Begäran om utdrag ur verksamhetsregistret skickas med e-post till registerfragor@ivo.se" or by post (Box 45184, 104 30 Stockholm). Anyone may request; anonymity and no stated purpose are constitutional rights for public information; IVO performs a secrecy review before release. |
| Recurring extract | not offered as a service on ivo.se. Whether IVO would agree to a standing monthly extract is unknown and must be asked. |
| Own-data self-service | `minasidor.ivo.se` (BankID/SITHS) lets a provider fetch an extract of *its own* activities. Not applicable to third parties; not to be bypassed. |
| Fees | avgiftsförordningen as applied by IVO: pages 1–9 free, page 10 costs 50 SEK, every further page 2 SEK; secrecy review is free; delivery by secure e-mail (link valid 60 days) or post. How a data file is counted in "pages" is not stated. |

## Freshness (VERIFIED SOURCE FACT)

* E-service FAQ: "Inskickade ändringar uppdaterar registret direkt."
  Activities have the statuses **Aktiv**, **Ej startad** and **Nedlagd**;
  only Aktiv and Ej startad can be edited.
* Annual report 2025: by the end of 2025 the e-service covered all
  functionality previously handled by form; the share of notifications
  arriving through the e-service rose during 2025 to about 79 %. Stiftelser
  and ideella föreningar still cannot use the e-service and file forms.
* Consequence: a new registration is in the register the moment it is
  submitted (e-service) or after manual handling (forms). How quickly a
  requested extract is delivered is not documented; IVO's annual report
  notes long handling times in several case types.

COMMERCIAL INFERENCE: the register is fresher than either earlier source
(Arbetsmiljöverket: one day; Klimatklivet: up to six months). The signal
lives or dies on extract cadence, not on the register.

## Fields (VERIFIED SOURCE FACT: what a notification must contain)

From patientsäkerhetsförordningen 2 kap. 1 § and HSLF-FS 2023:7 9 §.
Whether each field is *released* in an extract is IVO's secrecy decision
and was not verified.

| Field group | Content |
| --- | --- |
| Provider | name; **organisation number, or personal identity number when the provider has none**; postal address, web address, e-mail, phone |
| Activity (establishment) | name, e-mail, phone; **all digital and physical addresses where the activity is run** |
| Verksamhetschef | name, postal address, e-mail, phone (a natural person) |
| Timing | date the activity starts, changes or closes |
| Orientation | one or more codes from IVO's code list (A tandvård … J primärvård, 100+ codes, e.g. A02 allmän tandvård, I09 företagshälsovård, I19 psykologverksamhet, I21 fysioterapi, B18/B19 estetiska ingrepp/injektioner, J01 allmänmedicin, I11 hemsjukvård) |
| Flags | offers digital care contacts (yes/no); has inpatient beds (yes/no); **previously run by another provider, and that provider's name** |
| Submitter | name, e-mail, phone of the person filing; proof of authority |
| Register status | Aktiv / Ej startad / Nedlagd (e-service) |

Not required by the rules, therefore not expected: municipality or county
as separate fields (derivable from the address), legal form (derivable
from the organisation number), expected number of staff, premises size.

Mapping to the task's field list:

| Wanted | Available in the register |
| --- | --- |
| provider name, organisation number, provider contact | yes (personal identity number for sole traders) |
| establishment ID | an internal id must exist (the e-service edits individual activities); its form and stability across extracts are unverified |
| establishment name, physical address, phone, e-mail, website | name, addresses, phone, e-mail yes; website at provider level |
| municipality, county | derivable from the address, not confirmed as fields |
| activity type | yes, coded |
| registration date | not a required field; the register may hold a submission timestamp (unverified) |
| planned start date | yes ("tidpunkt för när verksamheten ska påbörjas") |
| status | yes (Aktiv / Ej startad / Nedlagd) |
| physical / digital indicator | yes: digital and physical addresses are listed separately, plus the digital-care flag |

## Identity and establishment semantics (VERIFIED SOURCE FACT + INFERENCE)

* VERIFIED: the unit of registration is the activity led by one
  verksamhetschef; a chain registers each clinic separately; a registered
  activity can list several physical addresses.
* VERIFIED: the "previously run by another provider" flag exists, which is
  the only source-side marker that separates a takeover from a new
  opening.
* INFERENCE on modelling, should data arrive: the provider maps onto the
  event's organisation (organisation number + name); the activity maps
  onto the workplace concept (name + address). `public_events` today
  carries `workplace_cfar`, a CFAR-specific column; an IVO activity id is
  not a CFAR. The smallest source-neutral change would be to keep the IVO
  activity id in the observation payload and use it only inside
  `source_event_id`, leaving `workplace_cfar` NULL and `workplace_name`
  filled. No column should be added until a real extract shows the id.
* INFERENCE on event derivation: an extract is a state snapshot. A
  "registered" event would be derived from an activity that is absent in
  snapshot N and present in snapshot N+1, or present with status Ej
  startad; `first_observed_at` is when we saw it, the planned start date
  is the source's date, and the registration date must not be invented
  when the extract lacks it.

## Change semantics (VERIFIED SOURCE FACT)

The register distinguishes new activity, change of previously submitted
data, and closure (form section headings in HSLF-FS 2023:7 bilaga 1) and
records the date of each. It does not distinguish "move" from other
changes as a separate type, and it has no explicit "administrative
correction" type. Distinguishing a genuine new establishment from a
re-registration therefore rests on: status Ej startad, a future start
date, the previous-provider flag, and comparison with earlier snapshots.

## Volume and the 2025–2026 distortions (VERIFIED SOURCE FACT)

| | 2023 | 2024 | 2025 |
| --- | --- | --- | --- |
| Active activities in Vårdgivarregistret at year end | 40 340 | 41 260 | 42 431 |

* About **3 500 new activities** were registered during 2025 (annual
  report). Net growth was 1 171, so roughly 2 300 activities were closed
  or cleaned up in the same year.
* IVO ran information campaigns in 2025 ahead of the dentistry permit
  duty and "uppmanat samtliga privata tandvårdsverksamheter att kontrollera
  och vid behov uppdatera sina uppgifter", which "medfört en ökning av
  antalet anmälningar … under årets senare del. Det omfattar både
  nyregistreringar och uppdateringar av befintliga verksamheter." IVO
  itself says the growth "delvis förklaras av ökade anmälningar inom den
  privata tandvården" and may indicate improved coverage rather than new
  activity.
* SOU 2023:82 (cited by IVO) estimated about 2 850 existing private
  dental activities needing a permit and about 250 new ones per year.
* IVO has prioritised compliance orders against aesthetic activities
  (lagen 2021:363), another source of late registrations of existing
  operations.

COMMERCIAL INFERENCE: of roughly 290 new register entries per month, an
unknown but material share in late 2025 and 2026 are existing operations
registering late. Any evaluation must separate "Ej startad with a future
start date" from "Aktiv with a past start date registered late".

## Privacy and reuse (VERIFIED SOURCE FACT)

* Sole traders are identified by personal identity number; the
  verksamhetschef and the submitter are natural persons with contact
  details. An extract will contain personal data unless IVO masks it;
  IVO's FAQ states that many released documents contain personal data and
  that secrecy applies where release would breach data-protection law.
* Public information is released without asking who requests it or why.
  IVO may ask when secrecy is in question.
* No licence terms for reuse were found on ivo.se; public records are
  released under tryckfrihetsförordningen, and downstream processing of
  personal data is the recipient's GDPR responsibility.
* IVO's own experiment rule for us: prefer aktiebolag, other legal
  entities and public bodies; never store or publish personal identity
  numbers; treat verksamhetschef names as contact data with a legal basis
  to be established before use.

## Technical feasibility (VERIFIED SOURCE FACT + INFERENCE)

* VERIFIED: no machine-readable access exists; the only channel is a
  manual request answered manually.
* INFERENCE: if IVO delivers a file, a snapshot adapter is a small
  addition (a parser for the delivered format, dataset-level provenance as
  with Klimatklivet, snapshot diffing keyed on the activity id, event type
  `HEALTHCARE_ESTABLISHMENT_REGISTERED`). The architecture is not the
  obstacle; the feed is.

## Commercial limitations (COMMERCIAL INFERENCE)

* Freshness of observation is bounded by extract cadence; a monthly
  extract would give 0–30 days of delay on top of a 30–60 day legal lead
  time, which is still earlier than any other source we have.
* Private dentistry, the family with the strongest one-time equipment
  purchases, now appears only after a permit decision.
* Volume is modest (about 3 500 per year including noise), so the value
  must come from multiple supplier verticals per event, as the task
  anticipates.
* Sole traders and solo practitioners are likely a large share of new
  entries; the B2B-relevant subset is smaller than the headline.

## Re-entry path

1. Send the request below to `registerfragor@ivo.se` (this was not sent
   by the agent; it is an outbound message the team must approve).
2. If a file arrives, check the five gate questions: activity id present
   and stable; status and planned start date present; physical addresses
   present; organisation number present; personal data masked or
   maskable. Ask whether a standing monthly extract of activities with
   status Ej startad or registered in the last month is possible, and at
   what cost.
3. Only then build `internal/source/ivo` as a snapshot adapter with the
   tests the task lists (identity of two establishments of one provider,
   changed phone is not a new establishment, same snapshot twice, absent
   then present gives one event).

Draft request (Swedish):

> Hej, jag begär utdrag ur vårdgivarregistret enligt
> offentlighetsprincipen. Önskat urval: verksamheter med status "Ej
> startad" samt verksamheter som anmälts som nyetablering under de senaste
> tre månaderna. Önskade uppgifter per verksamhet: verksamhetens id i
> registret, verksamhetens namn, fysiska och digitala adresser,
> verksamhetens inriktningskoder, uppgift om digitala vårdkontakter,
> planerat startdatum, datum för anmälan, status, uppgift om verksamheten
> tidigare bedrivits av annan vårdgivare, samt vårdgivarens namn och
> organisationsnummer. Personuppgifter om verksamhetschef, kontaktperson
> och enskilda näringsidkares personnummer behövs inte. Helst som
> Excel- eller CSV-fil via e-post. Jag vill också fråga om det är möjligt
> att få ett motsvarande utdrag återkommande, till exempel månadsvis, och
> vad det i så fall kostar.
