# Evaluation: deterministic deficiency signals in Arbetsmiljöverket's diary

Research spike, 2026-09-25. Question: does Arbetsmiljöverket's public
metadata already name exact regulatory deficiencies whose correction
implies a narrow supplier category, without ordering the document?

Data: every document of the relevant types registered 2025-09-25 to
2026-09-24 (twelve months) in the public diary, read with plain HTTP as
described in `docs/sources/arbetsmiljoverket.md` (section "Enforcement and
inspection-certificate document types"). Per-case rows are in
`arbetsmiljoverket-deterministic-signals.csv`. Everything marked VERIFIED
is counted from that data or quoted from AFS 2023:11; everything marked
INFERENCE is our judgement.

## 1. Recommendation: CONTINUE

Yes, with one surprise. The sanction-fee process (`6.4 Hantera
avgiftsutdömande`) is exactly as specific as hoped: 47 % of its 1,294
cases carry a title that names the deficiency ("Sanktionsavgift –
lyftanordningar m.m. – besiktning", "… – trycksatta anordningar –
kontroll", "… – trucktillstånd"). But it is a late signal: the
inspection that found the deficiency happened about 100 days earlier,
and the order arrives after the employer was already told to fix it.

The better signal was not on the task's list. Document type `6.1-49
Intyg återkommande besiktning` is an **incoming inspection certificate
from an accredited body**, and AFS 2023:11 13 kap. 13 § obliges the body
to notify Arbetsmiljöverket only when "anordningen inte erbjuder
betryggande säkerhet". The type produced 1,738 cases in twelve months.
43 of them carry other titles (inspection campaigns, asbestos and
crusher notifications, two "För kännedom - godkänd besiktning" rows) and
are filtered out by title; each of the remaining **1,695 cases titled
"Återkommande besiktning - <device>"** means one named device at one
named workplace **failed its recurring inspection**, and the title says
which device ("Återkommande besiktning - Fordonslyft flerpelare"): 1,101
lifting devices, 578 pressure devices, 16 other devices. Organisation
number is present on 97 % of those cases and CFAR on 93 %. The
regulatory case is still active when it appears: Arbetsmiljöverket's
demand and the employer's answer follow in the weeks after (section 5).
This is the "inspection performed → not approved → case appears" flow
the task asked us to look for, and it exists. What the diary does not
show is whether the employer has already chosen a repair supplier by
then; that is a commercial-validation question, not a data question.

Three recipes are worth building; two of them come from the certificate
type, one from the sanction type. The rest of the sanction dataset is
either administrative (posting of workers, working time), too thin
(medical certificates, chemical training: 15 cases a year each), or
specific-but-ambiguous (fall protection).

## 2. Dataset (VERIFIED)

| Set | Documents | Cases | Period | Notes |
| --- | ---: | ---: | --- | --- |
| `6.4-1 Avgiftsföreläggande` (sanction orders) | 1,385 | 1,294 | 2025-09-25..2026-09-24 | full population; 1,641 in 2024, 1,761 in 2025 |
| `6.1-49 Intyg återkommande besiktning` (certificates) | 1,803 | 1,738 | same | full population; 43 filtered out as non-certificate titles, 1,695 certificate-title cases; 1,706 in 2025 |
| `6.1-55 Beslut om slutligt omedelbart förbud` (immediate prohibitions) | 887 | 810 | same | full population |
| `6.1-33/35 förbud/föreläggande med vite` | 424 | 390 | same | full population |
| `6.1-24 Tillsynsmeddelande` | 1,343 | 1,319 | 2026-06-25..2026-09-24 | three-month sample |
| Case pages (geography, chronology) | | 1,294 sanction + 454 certificate cases | | see sections 5 and 9 |
| Organisation histories | | 85 sanction cases | | stratified Phase 2 sample |

Sanction cases by specificity of the case title:

| Specificity | Cases | Share |
| --- | ---: | ---: |
| DETERMINISTIC | 614 | 47.4 % |
| SPECIFIC_BUT_AMBIGUOUS | 438 | 33.8 % |
| GENERIC | 220 | 17.0 % |
| UNKNOWN | 22 | 1.7 % |

Monthly sanction volume is stable at 85–150 cases except July (31) and
January (63).

## 3. Signal families (VERIFIED counts, INFERENCE verdicts)

### 3a. Sanction orders (`6.4-1`), one row per case

| Family | Cases | Per year | Specificity | Org.nr | CFAR | Timing | Supplier | Incumbent risk | Verdict |
| --- | ---: | ---: | --- | ---: | ---: | --- | --- | --- | --- |
| LIFTING_EQUIPMENT_INSPECTION (besiktning missing / not approved / certificate not shown) | 232 | 233 | DETERMINISTIC | 88 % | 100 % | LATE | accredited inspection body; lifting service | LOW for inspection (they had none), MEDIUM for service | PROMISING |
| PRESSURE_EQUIPMENT_CONTROL (kontroll missing, routines missing) | 220 | 221 | DETERMINISTIC | 87 % | 100 % | LATE | accredited inspection body; compressor/boiler service | LOW / MEDIUM | PROMISING |
| FORKLIFT_PERMIT (trucktillstånd) | 110 | 111 | DETERMINISTIC | 99 % | 99 % | LATE | forklift training provider | LOW | PROMISING (thin) |
| FALL_PROTECTION (fallrisk, avsaknad av fallskydd) | 330 | 332 | SPECIFIC_BUT_AMBIGUOUS | 93 % | 98 % | LATE | fall protection, scaffolding, lift rental | MEDIUM | WEAK at sanction stage; see prohibitions |
| POSTING_NOTIFICATION (utstationering) | 164 | 165 | GENERIC | 18 % | 14 % | – | none | – | REJECT |
| WORK_ENVIRONMENT_PLAN (arbetsmiljöplan) | 56 | 56 | SPECIFIC_BUT_AMBIGUOUS | 98 % | 96 % | LATE | BAS-P/BAS-U consultants | LOW | WEAK |
| WORKING_TIME (ATL) | 39 | 39 | GENERIC | 79 % | 56 % | – | none | – | REJECT |
| QUARTZ_RISK_ASSESSMENT | 24 | 24 | SPECIFIC_BUT_AMBIGUOUS | 100 % | 96 % | LATE | occupational hygiene consultant | LOW | WEAK (volume) |
| CONSTRUCTION_PRE_NOTIFICATION (förhandsanmälan) | 17 | 17 | GENERIC | 100 % | 100 % | – | none | – | REJECT |
| CHEMICAL_TRAINING_CERTIFICATE (utbildningsintyg, allergena kemikalier) | 15 | 15 | DETERMINISTIC | 73 % | 100 % | TIMELY | härdplast/chemical safety training | LOW | WEAK (volume) |
| MEDICAL_FITNESS_CERTIFICATE (tjänstbarhetsintyg) | 15 | 15 | DETERMINISTIC | 73 % | 100 % | TIMELY | occupational health provider | MEDIUM | WEAK (volume) |
| SCAFFOLDING (training 5, other 11) | 16 | 16 | DETERMINISTIC / ambiguous | 94 % | 100 % | TIMELY | scaffolding training | LOW | WEAK (volume) |
| ASBESTOS (permit 7, training 4, medical 4, other 5) | 20 | 20 | mostly DETERMINISTIC | 95 % | 85 % | TIMELY | asbestos training / consultant / occupational health | LOW | WEAK (volume) |
| EXPLOSIVE_ATMOSPHERE, SHARPS_CONTAINER, unspecified, other | 29 | 29 | mixed | | | | | | REJECT |

Sanction amounts (AFS 2023:11, VERIFIED): lifting equipment used without
besiktning or without the latest certificate, 13 kap. 4 § and 16 §:
40 000 kr + (employees − 1) × 721 kr, max 400 000 kr. Pressure equipment
kept pressurised without control, 10 kap. 8 §: 15 000–300 000 kr by
class and rated power. Truck without written permit, 4 kap. 17 §:
15 000 kr + (employees − 1) × 271 kr, max 150 000 kr. Scaffolding
training: 5 000–20 000 kr per worker.

### 3b. Inspection certificates (`6.1-49`), one row per case

Population: 1,738 cases in total; 43 have titles that are not
certificate titles (inspection campaign names, asbestos and crusher
notifications, two "För kännedom - godkänd besiktning" rows) and are
filtered out before any failure reading; the deterministic claim applies
to the 1,695 cases titled "Återkommande besiktning - <device>", which
split into the three device families below.

| Family | Cases | Per year | Specificity | Org.nr | CFAR | Workplace ≠ legal entity | Timing | Supplier | Incumbent risk | Verdict |
| --- | ---: | ---: | --- | ---: | ---: | ---: | --- | --- | --- | --- |
| LIFTING_EQUIPMENT_FAILED_INSPECTION | 1,101 | 1,100 | DETERMINISTIC | 97 % | 93 % | 32 % | TIMELY for remediation; supplier selection unknown | lifting-equipment service by device type, then re-inspection | MEDIUM (HIGH for rental majors) | STRONG |
| PRESSURE_EQUIPMENT_FAILED_INSPECTION | 578 | 580 | DETERMINISTIC | 98 % | 93 % | 37 % | TIMELY for remediation; supplier selection unknown | compressor / boiler / kitchen-equipment service, then re-control | MEDIUM | STRONG |
| OTHER_EQUIPMENT_FAILED_INSPECTION (ports, pallställ, stegar) | 16 | 16 | DETERMINISTIC | 94 % | 88 % | | | equipment service | | ignore |
| Not a certificate title (inspection campaigns, notifications, "För kännedom - godkänd besiktning"); not counted as failures | 43 | 43 | UNKNOWN | | | | | | filter out by title |

Device mix, lifting: overhead cranes/hoists (traverskran, telfer,
pelarsvängkran) 284, mobile elevating work platforms 277, vehicle lifts
216, dock levellers (lastbrygga) 105, truck-mounted cranes 82,
excavators/loaders used for lifting 31, construction hoists and tower or
mobile cranes 25, trucks and tail lifts 19, lifting tables 15, other 21.
Pressure: compressed-air receivers and compressors 203, boilers 117,
cooking kettles and autoclaves 97, expansion vessels and hot-water
accumulators 48, refrigeration and heat exchangers 43, gas tanks and
transport containers 15, other 22.

### 3c. Immediate prohibitions (`6.1-55`), one row per case

810 cases; organisation number 87 %, CFAR 97 %. 609 titles follow the
pattern "Inspektion - Omedelbart förbud <date> - <hazard> - <address>":
fallrisk 454, arbetsutrustning (unguarded machines, dough mixers) 114,
trycksatt anordning 7, rasrisk 4, a few dozen free-text variants. 201
titles are campaign names without a hazard. Specificity
SPECIFIC_BUT_AMBIGUOUS: "fallrisk" at a street address tells a roofing
contractor's work has been stopped, but not whether the fix is a
scaffold, a lift, guard rails or harnesses. Timing TIMELY (work is
stopped until the hazard is removed; the address is in the title).
Verdict PROMISING for fall-protection and scaffolding rental, as a
secondary recipe.

### 3d. Orders and prohibitions with penalty (`6.1-33`, `6.1-35`)

390 cases whose titles are campaign names or "Skyddsombuds begäran om
ingripande" (103). GENERIC; REJECT as a deficiency signal.

## 4. Strongest three recipes (INFERENCE, rules VERIFIED against the data)

### Recipe 1: lifting device failed recurring inspection

```text
SelectedArendeProcess = 6.1
AND SelectedHandlingType = 6.1-49 (row heading "Intyg återkommande besiktning")
AND origin = Inkommande
AND case title matches ^Återkommande besiktning
AND device text matches (arbetsplattform|plattform|lift|travers|telfer|kran|lyft|lastbrygg|lastkaj|hiss|gräv|lastmaskin|hjullastare|materialhanterare|truck|teleskop|bakgavel|arbetskorg|pelar|sax|slamsug|winsch|kätting|gaffel|hängställning)
→ LIFTING_EQUIPMENT_FAILED_INSPECTION
```

1,101 cases in twelve months (the regex above, applied to the device
text after the prefix, matches 1,091 of them; the rest are free-text
device names such as model numbers). Regulatory basis (VERIFIED, AFS 2023:11
13 kap. 12–13 §§): after a besiktning the body issues a certificate and,
"om kontrollorganet bedömer att anordningen inte erbjuder betryggande
säkerhet, ska de snarast meddela detta till Arbetsmiljöverket". Required
action: the device may not be used until the deficiencies are corrected
and it passes re-inspection (13 kap. 4 § makes use without an approved
besiktning a sanction-fee offence). Purchase: repair or service of the
named device type, then re-inspection.

### Recipe 2: pressure equipment failed recurring control

```text
same filter as Recipe 1
AND device text matches (tryck|kompressor|receiver|reciver|panna|pannor|ång|gryt|kokskåp|autoklav|steril|expansion|ackumulator|vvb|vvx|varmvatten|vattenvärmare|förvärmare|kondens|värmeväxlare|värmepump|kyl|gas|tank|behållare|cistern|kärl|hydrofor|reaktor|vakuumtork|avgasare|blästerklocka|lokomobil|separator|slangledning|fjv|flottör)
→ PRESSURE_EQUIPMENT_FAILED_INSPECTION
```

578 cases in twelve months (regex above: 563; together the two regexes
classify 97.5 % of the 1,695 certificate-title cases, and no title matches both
except "kompressorrum lastkaj"). Same duty for boilers that may not be
operated (10 kap. 41 §) and the general 13 kap. 13 § for the pressure
devices inspected under chapter 13 (compressed-air receivers, cooking
kettles are besiktning devices in bilaga 12). Purchase: compressor,
boiler or kitchen-equipment service, then re-control.

### Recipe 3: sanction order for missing lifting or pressure inspection

```text
IF   SelectedArendeProcess = 6.4
AND  SelectedHandlingType = 6.4-1 (row heading "Avgiftsföreläggande")
AND  case title matches (lyftanordning|bakgavellyft|fordonslyft|personlyft|besiktning|arbetskorg|teknisk anordning|13 kap)
→ LIFTING_EQUIPMENT_INSPECTION_SANCTION

ELSE IF SelectedArendeProcess = 6.4
AND  SelectedHandlingType = 6.4-1
AND  case title matches (trycksatt|tryckkärl|tryckluft|över- eller undertryck|10 kap\. 8)
→ PRESSURE_EQUIPMENT_CONTROL_SANCTION
```

The two branches are alternatives, evaluated in this order (a title that
matched the lifting branch is never tested against the pressure branch;
in the data no title matches both). Together they select 452 cases in
twelve months and reproduce the family counts exactly: 232 lifting and
220 pressure. Required action: have the device inspected
or controlled by an accredited body (and stop using it until then), pay
the fee. Purchase: an accredited inspection, which the employer by
definition did not have, plus whatever service is needed to pass.

A fourth rule, `title matches truck` → FORKLIFT_PERMIT_SANCTION (110 a
year, forklift training), is deterministic and clean but below the
volume gate; keep it as a cheap add-on to Recipe 3 rather than a product.

## 5. Earliest actionable event (VERIFIED from organisation histories)

For 85 sanction cases we listed every document of the same organisation
in the 400 days before the order (search by organisation number, or by
CFAR when no organisation was shown).

| Family (sampled) | n | Any inspection notice before the order | Median days before | Deficiency visible in that earlier title? | Earlier certificate case (Recipe 1/2) | Earlier immediate prohibition |
| --- | ---: | --- | ---: | --- | --- | --- |
| Lifting inspection sanction | 18 | 11 | 98 | no (campaign names: "Myndighetsgemensamma kontroller", "Riskmiljöer") | 3 (median 70 d) | 0 |
| Pressure control sanction | 18 | 15 | 103 | no, except 1 "Omedelbart förbud – Trycksatt anordning" | 3 (median 235 d) | 1 |
| Forklift permit sanction | 10 | 10 | 104 | partly: 8 of 10 came from the "Ett tryggt arbetsliv – truck" campaign | 0 | 1 |
| Medical certificate | 6 | 6 | 162 | no | 0 | 2 |
| Chemical training | 5 | 5 | 72 | no | 0 | 0 |
| Fall protection | 10 | 4 | 245 | yes for 8 of 10: an "Omedelbart förbud – Fallrisk" 11–393 days earlier | 0 | 8 |

Population-level check: of 218 lifting/pressure sanction cases dated
from March 2026 with an organisation number, 25 (11 %) had an earlier
certificate case for the same organisation, 26 to 342 days before. So
the sanction and the certificate are mostly different populations: the
certificate catches equipment that **is** inspected and fails; the
sanction catches equipment that was **never** inspected and was found at
a general inspection whose title does not say so.

Consequences:

* Recipe 3 (sanction) is LATE by construction. The inspection notice
  that started it is public about 100 days earlier but names only the
  campaign. At order time the employer has usually already been told in
  writing to arrange the inspection; 563 of the 1,294 sanction cases were accepted outright
  and 508 contested (section 9).
* Recipes 1 and 2 (certificate) are the earliest public moment in the
  diary for that device. The chronology shows the **regulatory
  remediation window** is still open when the case appears: of 454
  certificate cases opened April–June 2026 (case pages read on
  2026-09-25), 373 (82 %) got a Tillsynsmeddelande from Arbetsmiljöverket
  a median 6 days after the certificate (75th percentile 13 days), 301
  (66 %) contain the employer's "Svar på kravskrivelse", 151 (33 %)
  needed a reminder, 356 (78 %) were closed a median 39 days after the
  certificate, and 33 (7 %) contain a second certificate from the
  re-inspection. The certificate is therefore TIMELY with respect to
  remediation: the case commonly remains active for several weeks after
  it appears. It does **not** show whether **supplier selection** is
  still open. The employer holds the certificate from inspection day and
  may already have an incumbent service provider, may have contacted a
  repair company immediately after the failed inspection, or may have
  been referred to one by the inspection body; the lag between
  inspection day and registration is also not visible. Vendor-selection
  timing is therefore not established by the diary data and remains a
  commercial-validation question (section 14).
* For fall protection the earliest signal is the immediate prohibition
  (Recipe 3c), not the sanction, and it carries the site address.

## 6. Supplier markets (INFERENCE; populations are estimates, not counted)

| Recipe | Supplier type | Rough population | Sales model | Purchase | Incumbent risk |
| --- | --- | --- | --- | --- | --- |
| 1. Lifting failed inspection | crane/hoist service, MEWP service, vehicle-lift service, dock-leveller service, tail-lift service; accredited bodies for re-inspection | HUNDREDS of service firms nationally; roughly 10–20 accredited bodies (Kiwa, DEKRA, TÜV Nord and smaller ones) | local/regional (service technicians travel) | repair or overhaul, tens of thousands of kr and up; re-inspection a few thousand | MEDIUM: many SMEs use whoever the inspector recommends; HIGH for Cramo, Ramirent, Kranpunkten, Renta and other rental majors with own workshops (7 organisations own 20 % of the cases) |
| 2. Pressure failed inspection | compressor service, boiler service, commercial-kitchen service, refrigeration service | HUNDREDS | local/regional | service or replacement of a receiver, kettle or boiler; thousands to hundreds of thousands of kr | MEDIUM |
| 3. Sanction, inspection missing | accredited inspection bodies; the same service firms as above | TENS of bodies (weak SaaS market) plus HUNDREDS of service firms | national for bodies, regional for service | inspection (thousands of kr), then whatever is needed to pass | LOW for the inspection (none existed), MEDIUM for service |
| 3c. Immediate prohibition, fallrisk | fall-protection equipment, scaffolding rental, MEWP rental, safety training | HUNDREDS | local | equipment rental or purchase, days to weeks | MEDIUM |
| Forklift permit sanction | forklift training providers | HUNDREDS | regional | course per operator, thousands of kr | LOW |

Purchase value: MEDIUM for Recipes 1–3 (the sanction alone is
40 000–400 000 kr for lifting devices, which makes the corrective
purchase easy to justify), LOW for training recipes, MEDIUM for fall
protection.

## 7. Example opportunities (VERIFIED rows)

Certificate cases, lifting (all Pågående on 2026-09-24 unless noted):

| Case | Date | Organisation | Org.nr | CFAR | Workplace | Title |
| --- | --- | --- | --- | --- | --- | --- |
| 2026/059517 | 2026-09-16 | HELSINGE DÄCKCENTER AB | 5569620726 | 54274840 | same | Återkommande besiktning - fordonslyft flerpelarlyft |
| 2026/060314 | 2026-09-20 | SMA MINERAL AB | 5562063874 | 10226074 | BODA KALKVERK | Återkommande besiktning - Mobilplattform |
| 2026/057895 | 2026-09-10 | ERASTEEL KLOSTER AKTIEBOLAG | 5560571811 | 10129211 | METALLURGI | Återkommande besiktning - Mobil arbetsplattform |
| 2026/056550 | 2026-09-04 | HILLBLOM TRANSPORT AB | 5594295445 | 70165246 | same | Återkommande besiktning - Fordonskran |
| 2026/051516 | 2026-08-18 | GAVLEFASTIGHETER GÄVLE KOMMUN AB | 5560099581 | 10163517 | same | Återkommande besiktning - Lyftblock på balk |
| 2026/056111 | 2026-09-02 | FRILLEN ENTREPRENAD AB | 5591326599 | 59057570 | same | Återkommande besiktning - Grävmaskin med band |
| 2026/050917 | 2026-08-13 | HANDELSSTÅL I GÄVLE AB | 5569574170 | 54093083 | same | Återkommande besiktning - Fordonskran |
| 2026/049050 | 2026-08-06 | MECA SWEDEN AB | 5563565612 | 43270784 | KIRUNA | Återkommande besiktning - Gaffeltruck (closed) |

Certificate cases, pressure:

| Case | Date | Organisation | Org.nr | CFAR | Workplace | Title |
| --- | --- | --- | --- | --- | --- | --- |
| 2026/049128 | 2026-08-06 | EUROMAINT COMPONENTS AND MATERIALS AB | 5591630883 | 60365152 | same | Återkommande besiktning - Tryckluftbehållare |
| 2026/056540 | 2026-09-04 | HANINGE KOMMUN | 2120000084 | 19089358 | STADSBYGGNADSFÖRVALTNINGEN | Återkommande besiktning - Ång-Janne |
| 2026/052782 | 2026-08-21 | ÅKAB PROCESS AB | 5566553185 | 42624098 | same | Återkommande besiktning -Kompressor |
| 2026/058878 | 2026-09-14 | GRAYS BAKERY AB | 5564879442 | 32157067 | same | Återkommande besiktning - kokgryta |
| 2026/058079 | 2026-09-10 | STOCKHOLMS KOMMUN | 2120000142 | 19305036 | SANDÅKRASKOLAN | Återkommande besiktning - Kokgryta |
| 2026/053307 | 2026-08-25 | STENA RECYCLING AB | 5561321752 | 18001669 | SÖDERT | Återkommande besiktning - 2 stycken tryckluftbehållare (closed) |

Sanction orders:

| Case | Date | Organisation | Org.nr | CFAR | Family | Title |
| --- | --- | --- | --- | --- | --- | --- |
| 2026/052956 | 2026-09-21 | AUTOCAR TYRESÖ AB | 5590752936 | 57734758 | lifting | Sanktionsavgift – lyftanordningar m.m. – besiktning |
| 2026/045792 | 2026-09-09 | MADAM HONG IMPORT EXPORT AB (CHINA SUPERMARKET) | 5566934377 | 39408513 | lifting | 13 kap. 4 §: Sanktionsavgift – lyftanordningar m.m. – besiktning |
| 2026/054700 | 2026-09-09 | KRISTIANSTAD STÅL- & RÖRMONTAGE AB | 5569190449 | 53189064 | pressure | Sanktionsavgift – trycksatta anordningar – kontroll |
| 2026/051623 | 2026-09-21 | KOLSTORPS GÅRD AB | 5592011034 | 62186465 | pressure | Sanktionsavgift – trycksatta anordningar – kontroll |
| 2026/058890 | 2026-09-24 | POSTI LOGISTICS SOLUTIONS AB | 5566231352 | 70186689 | forklift | Sanktionsavgift – trucktillstånd |
| 2026/046454 | 2026-08-31 | WISBY CONSULTING AB (ELGIGANTEN) | 5566658232 | 43440122 | forklift | Sanktionsavgift – trucktillstånd |
| 2026/053845 | 2026-09-08 | PATRICKS GLAS AB | 5591295182 | 58946047 | medical | Sanktionsavgift - kemisk riskkälla - tjänstbarhetsintyg |
| 2026/052688 | 2026-09-24 | ELINSTALLATÖRERNA SYDOST AB | 5593406258 | 68134337 | scaffolding | Sanktionsavgift - ställningsutbildning |

Worked chronology (VERIFIED): TAGE REJMES I LINKÖPING BIL AB received an
immediate prohibition "Omedelbart förbud 20260710 - Trycksatt anordning"
on 2026-07-10, a failed-inspection certificate "Återkommande besiktning -
Fordonslyft flerpelare" on 2026-07-30 with Arbetsmiljöverket's demand the
next day, answered it on 2026-08-26, had the prohibition lifted on
2026-08-24, and got the sanction order "trycksatta anordningar –
kontroll" on 2026-09-03, which it accepted on 2026-09-22.

## 8. Metadata sufficiency (INFERENCE on VERIFIED titles)

| Recipe | Metadata alone sufficient | Document helpful | Document required |
| --- | --- | --- | --- |
| 1–2 Certificates | yes: device type, organisation, workplace, CFAR, date; what exactly failed is in the certificate but not needed to know that service is due | which components failed, whether the device may be used with restrictions | no |
| 3 Sanction, inspection missing | yes: deficiency, organisation, workplace | the number of devices and the fee | no |
| Forklift permit | yes | | no |
| Fall-protection sanction / prohibition | partly: hazard and address | what protection is missing | for the sanction titles without hazard |
| Orders/prohibitions with penalty | no | | yes |

## 9. Geography (VERIFIED from case pages)

Case pages carry county and municipality (present on 1,125 of the 1,294
sanction cases). Deterministic sanction families (612 cases with a
county):

| County | Cases | Share |
| --- | ---: | ---: |
| Stockholms län | 161 | 26 % |
| Skåne län | 100 | 16 % |
| Västra Götalands län | 71 | 12 % |
| Östergötlands län | 39 | 6 % |
| Jönköpings län | 30 | 5 % |
| Södermanlands län | 25 | 4 % |
| Örebro län | 21 | 3 % |
| Hallands län | 20 | 3 % |
| Uppsala, Kronobergs län | 18 each | 3 % each |
| Kalmar län | 17 | 3 % |
| Remaining ten counties | 92 | 15 % |

By family: lifting-inspection sanctions are Stockholm 51, Skåne 50,
Västra Götaland 41, then 9–11 in Blekinge, Jönköping, Örebro,
Östergötland and Kronoberg; pressure-control sanctions Stockholm 53,
Skåne 41, Västra Götaland 19, Östergötland 14, Uppsala 11; forklift
permits are Stockholm-heavy (47 of 110). Certificate cases (454 case
pages, all cases opened April–June 2026) follow the same shape:
Stockholm 137, Västra Götaland 59, Skåne 52, Västerbotten 23, Gävleborg
23, Södermanland 23, Östergötland 22, Jönköping 18.

INFERENCE: outside the three metropolitan counties a regional service
firm would see roughly 10–50 lifting or pressure events a year from the
sanction feed and two to four times that from the certificate feed, i.e.
enough for a regional supplier in the larger counties and thin in the
north. Municipality is available on the same page for a finer cut; the
search rows themselves carry no geography, so the case page (one extra
request per event) is required.

Sanction outcomes (1,294 case pages): 563 orders were accepted and
forwarded for collection, 508 were contested and sent to the
administrative court, the rest were still open.

## 10. Comparison with the inspection-notice signal (Phase 8)

| Property | Inspection notices (`6.1-23`) | Deterministic certificate and sanction cases |
| --- | --- | --- |
| Volume | ~9,000 a year (35–40 per weekday) | ~1,700 certificate-title cases + ~450 lifting/pressure sanctions + ~110 forklift sanctions a year; ~450 fall-risk prohibitions |
| Specificity | campaign name only | device type (certificates) or exact regulatory offence (sanctions) |
| Metadata sufficiency | document required for the deficiency | metadata sufficient |
| Timing | early (day of visit) but unspecific | certificates: earliest public moment in the diary for that device, regulatory case active for weeks, supplier-selection timing unknown; sanctions: ~100 days after the visit |
| Supplier mapping | weak (campaign → broad category) | strong (device → service type; offence → inspection or training) |
| Incumbent risk | unknown | medium; high for rental majors |
| Identity | 96 % org.nr, 99 % CFAR | 97 % / 93 % (certificates), 88 % / 100 % (sanctions) |

They are complements, not replacements: the certificate and sanction
feeds are a smaller, high-precision subset (about a fifth of the
inspection-notice volume) for equipment-service verticals, while
inspection notices remain the broad signal.

## 11. Unexpected deterministic signal families (Phase 10)

Found by reading the full title vocabulary of the enforcement types:

* **Failed recurring inspection certificates** (`6.1-49`, section 3b):
  the strongest family in the whole diary and not on the task's list.
* **Immediate prohibitions with hazard and address in the title**
  (`6.1-55`): "Fallrisk" (454 a year) and "Arbetsutrustning" (114,
  mostly unguarded machines and dough mixers). Specific-but-ambiguous,
  timely, and the site address is public.
* **Exposure-measurement reports** (`6.1-14 Anmälan, rapport från
  yrkeshygienisk mätning`, 101 a year, titles like "Yrkeshygienisk
  mätning - Styren"): deterministic about the agent, but the measurement
  was already bought; LATE for hygienists, possibly useful for
  ventilation and PPE suppliers. Not evaluated further.
* Nothing deterministic was found for medical examinations beyond the 15
  tjänstbarhetsintyg sanctions and 2 incoming medical-control reports.

## 12. Existing adapter fit (Phase 9)

`internal/source/arbetsmiljoverket` can carry these feeds with
configuration, not a new adapter; the recipe classification lives
outside the adapter and the event (see the fourth bullet):

* `Client.SearchURL` already takes `SubjectArea` and `DocumentType` per
  query; `Ingester.Run` hard-codes `6.1`/`6.1-23` and rejects other
  document types with a warning. Parameterising `Run` with a feed
  (subject area, document type, expected row heading, event type) is the
  only structural change; the row parser needs nothing, since certificate
  and sanction rows have the same markup and fields (verified on 3,661
  rows).
* Event identity stays the document number (`2026/059517-1`); the
  certificate-title cases have one incoming document each (96 % of
  cases), so one event per failed inspection.
* Two new event types in `publicevent` (for example
  `WORK_EQUIPMENT_INSPECTION_FAILED` and `WORK_ENVIRONMENT_SANCTION_ORDER`)
  and their addition to the API's `event_type` enum.
* The title-derived classification (lifting, pressure, vehicle lift,
  compressor, boiler, …) stays **outside** the canonical event, per
  `docs/public-events.md` ("Source truth versus interpretation"): the
  public event carries a neutral event type such as
  `WORK_EQUIPMENT_INSPECTION_FAILED`, a title that keeps the
  source-described device text ("Återkommande besiktning - Fordonslyft
  flerpelare"), organisation, workplace and provenance; the original
  Arbetsmiljöverket fields (document type, origin, subject area, case
  title, case status) stay in the source observation payload. No
  source-specific `equipment_family` or `deficiency_family` field is
  added to `public_events`. The device family can later live in recipe
  matching, in an assessment layer, or in another explicitly
  source-independent classification model once such a model exists; none
  of that is introduced by this evaluation.
* Volume is trivial: certificates and sanction orders together are
  about 3,200 documents a year, i.e. one page of ten rows per weekday.

## 13. Recommendation for implementation: IMPLEMENT_NOW (Recipes 1 and 2), BACKLOG (Recipe 3 and forklift)

Not implemented in this spike, per the task. The recommendation rests
on volume, buyer identity, metadata sufficiency and the open regulatory
window; it does not assume that supplier selection is still open when
the case appears (section 5), which must be validated commercially
before the feed is sold as a lead source. Minimum plan:

1. Add a second feed to the Arbetsmiljöverket ingester: subject area
   `6.1`, document type `6.1-49`, expected heading `Intyg återkommande
   besiktning`, origin `Inkommande`, title prefix `Återkommande
   besiktning`; skip rows that fail the heading or prefix check (the 43
   non-certificate cases).
2. Map to a neutral event type (`WORK_EQUIPMENT_INSPECTION_FAILED`),
   keep the source case title as the event title so the device text is
   visible, and leave the original fields in the observation payload.
   Do not store a device family on the event: the two regexes in
   section 4 are recipe rules for a later matching or assessment layer,
   and in this step they serve only the scope check and the tests.
3. Extend `cmd/ingest` with the feed choice and the API enum with the
   event type; tests for the scope check (certificate title vs the 43
   other titles), for idempotent re-runs, and a fixture built from real
   certificate rows. The lifting/pressure/other boundaries of section 4
   are tested wherever the recipe layer is built, not in the adapter.
4. Leave Recipe 3 as a documented backlog item: same feed mechanism with
   `6.4`/`6.4-1` and a neutral sanction-order event type; the title
   rules of section 4 and the LATE timing stay in the recipe
   documentation, not on the event.

## 14. Limitations

* Certificate registration date is "handlingens datum" at
  Arbetsmiljöverket; the lag from inspection day is not visible, so
  "earliest public moment" is relative to the diary, not to the
  inspection.
* The diary shows the regulatory window (Tillsynsmeddelande, reply,
  closure) but nothing about supplier selection: whether the employer
  already has a service provider, called one on inspection day, or was
  referred by the inspection body is unmeasured. The TIMELY label in
  sections 3b and 5 refers to remediation only; vendor-selection timing
  needs commercial validation (for example a call sample of employers or
  service firms) before the certificate feed is treated as a lead source.
* Family classification is rule-based on titles; 43 of the 1,738
  certificate cases and 22 of the 1,294 sanction cases carry
  uninformative titles and are excluded from the deterministic counts.
* Organisation histories were sampled (85 cases), not exhaustive; the
  population-level certificate-to-sanction link used organisation
  number only, so workplaces without an organisation number are missed.
* Supplier populations are estimates from public company listings, not
  counted registers; the Swedac register is a JavaScript application
  that was not scraped.
* No document was ordered; all statements about content come from the
  regulation text, not from the cases.
