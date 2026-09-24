# Klimatklivet as a buying signal: evaluation of approved grants 2025-01 to 2026-06

Companion to [docs/sources/klimatklivet.md](../sources/klimatklivet.md), which
holds the verified source and timing facts. This document is our commercial
reading of the ingested data. Every family, supplier category, relevance
tier and timing tier below is a **hypothesis derived from the category and
the applicant's one-line title**; nothing here is stored in the database,
and the manual validation in section 8 is what turns it into evidence.

## 1. Dataset

| | |
| --- | --- |
| Rows in the published file (all approvals since 2015) | 33,311 |
| Rows ingested (decision date ≥ 2025-01-01) | 7074 |
| Evaluation sample: decisions 2025-01-01..2026-06-30 | 7074 rows, 7074 distinct cases (case number is unique) |
| of which charging points (`Laddstation`) | 6729 (95 %): 555 public, 6174 non-public, 68,088 charging points |
| of which other climate investments | 345 (5 %) |
| Organisation numbers present | 0 (the source publishes names only) |
| Applicant type (name-based guess), non-charging | company: 229, sole trader / unclassified: 67, energy/utility company (often municipal): 13, company (no legal form in name): 12, association / foundation: 10, public body: 8, sole trader: 6 |
| Grant amount, non-charging | median 0.56 MSEK, p90 6.3 MSEK, max 298.7 MSEK, total 1928.8 MSEK |
| Grants ≥ 10 MSEK, non-charging | 28 |
| Investment amount | not published; companies receive 20–70 % of the investment as grant, other organisations at most 50 %, so the implied investment is 1.4–5× the grant (shown as a range in the sheet) |
| Status, non-charging | Pågående åtgärd: 189, Slutförd åtgärd: 156 |
| Location | county and municipality on every non-charging row; none on charging rows |
| Estimated visibility lag (decision → semi-annual publication + 7 days) | 7–190 days, median 98 days |

Non-charging decisions per month: 2025-01: 4, 2025-02: 78, 2025-03: 54, 2025-04: 18, 2025-05: 1, 2025-09: 1, 2025-11: 8, 2025-12: 35, 2026-01: 13, 2026-02: 30, 2026-03: 36, 2026-04: 29, 2026-05: 31, 2026-06: 7.
Decisions cluster in the weeks after each application round closes
(February–April and November–December), and the file appears once per
half-year, so the freshest rows are one week old and the oldest six months.

## 2. Investment-family distribution

Families are assigned by rule (category first, then title keywords; see
the `investment_family` column of the sheet). Medians are grant amounts,
not investments.

| Investment family | Projects | Share | Median grant (MSEK) | Total grant (MSEK) | Likely supplier categories (hypothesis) |
| --- | --- | --- | --- | --- | --- |
| Charging points, not public (housing associations, workplaces) | 6174 | 87.3 % | 0.04 | 730.7 | EV chargers; electrical installation; load management |
| Public charging (tender track) | 555 | 7.8 % | 1.4 | 894.7 | DC/AC chargers; electrical and civil installation; grid connection |
| Farm electrification (feeding, manure, irrigation, bedding, dryers, loaders) | 138 | 2.0 % | 0.38 | 71.7 | Agricultural equipment dealers; electrical contractors for grid extension |
| Building heating conversion (oil/gas to heat pump, pellets, chips, district heating) | 111 | 1.6 % | 0.51 | 110.4 | HVAC / heat pump installers; geothermal drilling; pellet and chip boiler suppliers; district heating connection |
| Circular material flows / recycling lines | 18 | 0.3 % | 5.4 | 211.0 | Specialised process machinery (sorting, textile, screening); conveyors; buildings and installation; electrical; automation |
| Industrial process / fuel conversion | 17 | 0.2 % | 3.9 | 140.8 | Industrial energy engineering; boilers, furnaces, ovens, dryers; electric process heat; piping; electrical contractors; industrial automation; installation and commissioning |
| Quarry / crusher electrification | 15 | 0.2 % | 2.0 | 31.6 | Electrical contractors; grid connection; electric crushers, conveyors and drives; transformers |
| Biogas production, upgrading, liquefaction | 13 | 0.2 % | 29.1 | 995.0 | Biogas process equipment (digesters, upgrading, liquefaction, CHP); tanks; pumps; civil works; electrical; automation; gas grid connection |
| Other | 11 | 0.2 % | 0.26 | 6.1 | Unknown until description is read |
| N2O / methane abatement at utilities | 6 | 0.1 % | 0.55 | 5.8 | Process measurement; wastewater process equipment; landfill gas systems; consultants |
| Biogas / LBG filling stations | 5 | 0.1 % | 10.4 | 44.8 | Compressors, dispensers, storage; civil works; electrical |
| Hydrogen production | 3 | 0.0 % | 110.2 | 258.1 | Electrolysers; compressors and storage; electrical/grid; civil works; engineering |
| District heating expansion / connection | 3 | 0.0 % | 3.1 | 8.3 | Pipe and civil contractors; pre-insulated pipe; substations; utility engineering |
| Vehicles, machinery, transport infrastructure | 3 | 0.0 % | 0.33 | 18.1 | Machinery dealers; civil contractors |
| Waste heat recovery and heat storage | 2 | 0.0 % | 13.4 | 26.9 | Heat exchangers; HVAC; piping; storage; controls; energy engineering |

Concentration: 6729 of 7074 rows are charging points. Among the 345
other investments, building heating conversions and farm electrification
make up 249 (72 %), while the
71 industrial, biogas, hydrogen, circular, heat-network and quarry projects
carry 87 % of the granted money.

## 3. Commercial relevance (hypothesis)

Criteria: `HIGH` = multi-component physical investment where external
engineering, contractors or specialised equipment are unavoidable (biogas,
hydrogen, recycling lines, industrial conversions ≥ 1 MSEK grant, large
heat recovery). `MEDIUM` = clear external equipment or contractor need but
smaller or single-vendor scope (quarry electrification, heat networks,
utility abatement, filling stations, building conversions ≥ 1.5 MSEK
grant). `LOW` = commoditised or tiny installations where the quoting
vendor is the project (small heating swaps, farm equipment, charging).

| Tier | All rows | Non-charging |
| --- | --- | --- |
| HIGH | 49 (1 %) | 49 (14 %) |
| MEDIUM | 54 (1 %) | 54 (16 %) |
| LOW | 6960 (98 %) | 231 (67 %) |
| UNKNOWN | 11 (0 %) | 11 (3 %) |

Read across the whole file the signal is dominated by low-value charging
rows; read across the non-charging investments, roughly a quarter are
HIGH and a further quarter MEDIUM.

## 4. Supplier-market analysis

| Investment family | Projects (non-charging) | HIGH+MEDIUM | Grant volume (MSEK) | Timing tier | Supplier categories (hypothesis) |
| --- | --- | --- | --- | --- | --- |
| Industrial process / fuel conversion | 17 | 17 | 140.8 | UNCERTAIN | Industrial energy engineering; boilers, furnaces, ovens, dryers; electric process heat; piping; electrical contractors; industrial automation; installation and commissioning |
| Biogas production, upgrading, liquefaction | 13 | 13 | 995.0 | UNCERTAIN | Biogas process equipment (digesters, upgrading, liquefaction, CHP); tanks; pumps; civil works; electrical; automation; gas grid connection |
| Circular material flows / recycling lines | 18 | 18 | 211.0 | UNCERTAIN | Specialised process machinery (sorting, textile, screening); conveyors; buildings and installation; electrical; automation |
| Hydrogen production | 3 | 3 | 258.1 | UNCERTAIN | Electrolysers; compressors and storage; electrical/grid; civil works; engineering |
| Quarry / crusher electrification | 15 | 15 | 31.6 | UNCERTAIN | Electrical contractors; grid connection; electric crushers, conveyors and drives; transformers |
| District heating expansion / connection | 3 | 3 | 8.3 | LIKELY_OPEN | Pipe and civil contractors; pre-insulated pipe; substations; utility engineering |
| Waste heat recovery and heat storage | 2 | 2 | 26.9 | LIKELY_OPEN | Heat exchangers; HVAC; piping; storage; controls; energy engineering |
| N2O / methane abatement at utilities | 6 | 6 | 5.8 | LIKELY_OPEN | Process measurement; wastewater process equipment; landfill gas systems; consultants |
| Biogas / LBG filling stations | 5 | 5 | 44.8 | UNCERTAIN | Compressors, dispensers, storage; civil works; electrical |
| Building heating conversion (oil/gas to heat pump, pellets, chips, district heating) | 111 | 20 | 110.4 | LIKELY_SELECTED | HVAC / heat pump installers; geothermal drilling; pellet and chip boiler suppliers; district heating connection |
| Farm electrification (feeding, manure, irrigation, bedding, dryers, loaders) | 138 | 0 | 71.7 | LIKELY_SELECTED | Agricultural equipment dealers; electrical contractors for grid extension |
| Vehicles, machinery, transport infrastructure | 3 | 1 | 18.1 | UNCERTAIN | Machinery dealers; civil contractors |
| Other | 11 | 0 | 6.1 | UNCERTAIN | Unknown until description is read |

Recurring markets where the same signal could be sold repeatedly:

* **Industrial energy engineering, boilers/furnaces, piping, electrical
  and automation contractors** — 17 industrial conversions in 18 months, decided
  continuously, 0.5–46 MSEK grants.
* **HVAC / heat pump installers, geothermal drillers, pellet and chip
  boiler suppliers** — 111 building conversions; high volume, small tickets,
  but the installer is usually already chosen (section 6).
* **Biogas and hydrogen plant suppliers, civil and electrical contractors,
  automation** — 16 plants with 1253.2 MSEK of grants; few but very large.
* **Recycling / process machinery and industrial installation** — 18 lines.
* **Electrical contractors and grid connection** — quarries (15), farms and every charging site.
* **Agricultural equipment dealers** — 138 farm projects; small, mostly sole traders.

Poor leads by construction: farm equipment and small heating swaps (one
vendor, ordered soon after decision), charging points (commoditised;
6,729 rows), and public bodies buying through formal procurement where the
tender, not the grant list, is the actionable event.

## 5. High-value opportunities

Larger, physical, installation-heavy, multi-component projects with
obvious external engineering or contractor scope. The 28 non-charging
grants of 10 MSEK or more are almost all in this group; examples:

| Case | Decided | Applicant | Project | Municipality | Grant (MSEK) | Family | Relevance | Timing |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| NV-25-044904 | 2026-02-26 | Falk Biogas AB | FALK Biogas | Borgholm | 221.3 | Biogas production, upgrading, liquefaction | HIGH | UNCERTAIN |
| NV-25-045734 | 2026-06-11 | Skövde Energi | Vätgasproduktion i Skövde | Skövde | 110.2 | Hydrogen production | HIGH | UNCERTAIN |
| NV-25-045976 | 2026-05-13 | Holma Spinning Mill AB | Från avfall till garn: Sveriges första spinninglinje för åte | Hudiksvall | 77.4 | Circular material flows / recycling lines | HIGH | UNCERTAIN |
| NV-25-043146 | 2025-12-18 | Orkla Snacks Sverige AB | Energikonvertering industri | Filipstad | 45.9 | Industrial process / fuel conversion | HIGH | UNCERTAIN |
| NV-25-046158 | 2026-04-30 | Aluminium Casting Unnaryd | Flexibel smältning för pressgjuten aluminium med lågt klimat | Hylte | 28.4 | Industrial process / fuel conversion | HIGH | UNCERTAIN |
| NV-25-045969 | 2026-06-25 | Lidköping Energi AB | 20GWh säsongslager för tillvaratagande av spillvärme | Lidköping | 26.1 | Waste heat recovery and heat storage | HIGH | LIKELY_OPEN |
| NV-25-046025 | 2026-03-05 | Tegelmöllan AB | Etablering av tegelåterbruk i Sköldinge | Katrineholm | 20.3 | Circular material flows / recycling lines | HIGH | UNCERTAIN |
| NV-25-043069 | 2026-03-06 | Solör Bioenergi Värme AB | Konvertera från gasol till träpellets Bewi Värnamo. | Värnamo | 17.6 | Industrial process / fuel conversion | HIGH | UNCERTAIN |

## 6. Timing assessment (the decisive question)

What the rules establish (verified, see the source note): nothing may be
ordered or contracted before the decision, quotations must accompany the
application, and eligible spending, including procurement and design,
starts at the decision date. What the rules do not establish: whether the
quoting supplier gets the order. Our per-family reading:

| Investment family | Projects | Timing tier | Confidence | Why |
| --- | --- | --- | --- | --- |
| Industrial process / fuel conversion | 17 | UNCERTAIN | low | Main equipment is quoted before application (quotation required); piping, electrical, automation, installation and commissioning are usually procured after the decision, and eligible costs (incl. procurement and engineering) only start at the decision. |
| Biogas production, upgrading, liquefaction | 13 | UNCERTAIN | low | Large multi-year projects (end dates 2028–2030); the process supplier is typically quoted pre-application, civil works, electrical and automation are procured later. Some components LIKELY_OPEN. |
| Hydrogen production | 3 | UNCERTAIN | low | Few electrolyser vendors, likely selected at quotation; balance of plant, civil and electrical open. |
| Circular material flows / recycling lines | 18 | UNCERTAIN | low | The core line vendor is quoted pre-application; building, installation and electrical work is usually separate and later. |
| Quarry / crusher electrification | 15 | UNCERTAIN | low | Repeat applicants (Skanska, Svevia) likely use framework agreements; single-site contractors may still be open. |
| District heating expansion / connection | 3 | LIKELY_OPEN | medium | Municipal energy companies must procure under the procurement acts; a contract award before the decision would count as starting the measure. |
| Waste heat recovery and heat storage | 2 | LIKELY_OPEN | low | Mostly public-owned utilities (procurement after decision); private cases uncertain. |
| N2O / methane abatement at utilities | 6 | LIKELY_OPEN | medium | Public wastewater and waste utilities procure after the decision; niche vendor market. |
| Biogas / LBG filling stations | 5 | UNCERTAIN | low | One operator with several sites likely has a preferred equipment partner; civil work per site may be open. |
| Building heating conversion (oil/gas to heat pump, pellets, chips, district heating) | 111 | LIKELY_SELECTED | medium | A boiler swap is a one-vendor turnkey job; the quotation attached to the application is in practice the selection, and small tickets are ordered soon after the decision. |
| Farm electrification (feeding, manure, irrigation, bedding, dryers, loaders) | 138 | LIKELY_SELECTED | medium | Equipment quote equals brand choice; many applicants are sole traders; small tickets. |
| Charging points, not public (housing associations, workplaces) | 6174 | LIKELY_SELECTED | medium | Installer quote defines the project; commoditised. |
| Public charging (tender track) | 555 | LIKELY_SELECTED | low | Applicants are charging operators bidding with a price; hardware partners are normally set. |
| Vehicles, machinery, transport infrastructure | 3 | UNCERTAIN | low | Heterogeneous. |
| Other | 11 | UNCERTAIN | low | Title too unspecific. |

Totals, non-charging: LIKELY_SELECTED: 245, UNCERTAIN: 89, LIKELY_OPEN: 11. All rows: LIKELY_SELECTED: 6882, UNCERTAIN: 181, LIKELY_OPEN: 11.

Two structural facts cut against the signal even where selection is
open at the decision:

1. **Publication lag.** We see a decision 7–190 days after it was made
   (median 98). Eligible spending starts at the decision, so for
   one-vendor projects the order has usually been placed before the row
   is visible.
2. **The quotation is the project.** For single-equipment investments
   the attached quotation is in practice the supplier selection; only the
   secondary scopes (electrical, civil, piping, automation, commissioning)
   are plausibly open, and only for multi-component projects.

The signal is therefore most plausible for the 89 UNCERTAIN and 11 LIKELY_OPEN
non-charging projects, and within those for the sub-scopes rather than
the main equipment. Long-running projects (biogas, hydrogen, industrial
conversions with end dates in 2028–2030) remain in procurement for
months after the decision and are the least hurt by the lag.

## 7. Where the primary uncertainty lies

* **Data**: identity (no organisation number) and detail (one-line title,
  no investment amount) are weaker than for Arbetsmiljöverket, but
  sufficient to recognise the investment family and the applicant.
* **Timing**: the semi-annual publication is the largest structural
  problem and cannot be fixed from this source.
* **Supplier selection**: unknown and only answerable by asking project
  owners. This is what the shortlist is for.

## 8. Manual validation shortlist (28 projects)

Criteria: decided 2025-11-01 or later, private or utility company, grant
≥ 2 MSEK, HIGH or MEDIUM relevance, at most two per organisation (one for
Skanska). Question to ask each project owner: *When your Klimatklivet
grant was approved, had you already selected or contracted the supplier
for the investment? Which parts, if any, were still open?* Record the
answer as supplier already selected / quotations only / mixed, per
component.

| Company | Project | Decided | Grant (implied investment) | Family | Likely supplier category | Why interesting | Timing tier: main uncertainty |
| --- | --- | --- | --- | --- | --- | --- | --- |
| Biogas Nordöstra Skaraborg AB | Biogasproduktion i Tibro. | 2025-12-18 | 298.7 MSEK grant (impl. 427–1493 MSEK) | Biogas production, upgrading, liquefaction | Biogas process equipment (digesters, upgrading, liquefaction, CHP) … | Very large multi-year plant; civil, electrical, automation and gas handling are separate procurements from the process supplier. | UNCERTAIN: Process/EPC supplier may already be chosen; ask about civil, electrical, automation and upgrading packages. |
| Falk Biogas AB | FALK Biogas | 2026-02-26 | 221.3 MSEK grant (impl. 316–1106 MSEK) | Biogas production, upgrading, liquefaction | Biogas process equipment (digesters, upgrading, liquefaction, CHP) … | Very large multi-year plant; civil, electrical, automation and gas handling are separate procurements from the process supplier. | UNCERTAIN: Process/EPC supplier may already be chosen; ask about civil, electrical, automation and upgrading packages. |
| Skottorps Energi AB | Produktion och uppgradering av biogas på Skottorps Energi | 2026-05-13 | 178.8 MSEK grant (impl. 255–894 MSEK) | Biogas production, upgrading, liquefaction | Biogas process equipment (digesters, upgrading, liquefaction, CHP) … | Very large multi-year plant; civil, electrical, automation and gas handling are separate procurements from the process supplier. | UNCERTAIN: Process/EPC supplier may already be chosen; ask about civil, electrical, automation and upgrading packages. |
| Biogasbolaget i Mellansverige AB | Produktion av flytande biogas vid Biogasbolaget i Mellansverige AB:s b | 2026-05-21 | 114.8 MSEK grant (impl. 164–574 MSEK) | Biogas production, upgrading, liquefaction | Biogas process equipment (digesters, upgrading, liquefaction, CHP) … | Very large multi-year plant; civil, electrical, automation and gas handling are separate procurements from the process supplier. | UNCERTAIN: Process/EPC supplier may already be chosen; ask about civil, electrical, automation and upgrading packages. |
| Skövde Energi | Vätgasproduktion i Skövde | 2026-06-11 | 110.2 MSEK grant (impl. 157–551 MSEK) | Hydrogen production | Electrolysers … | Electrolyser plant with substantial balance-of-plant, grid and civil scope. | UNCERTAIN: Electrolyser vendor likely fixed; ask about balance of plant and installation. |
| Holma Spinning Mill AB | Från avfall till garn: Sveriges första spinninglinje för återvunna tex | 2026-05-13 | 77.4 MSEK grant (impl. 111–387 MSEK) | Circular material flows / recycling lines | Specialised process machinery (sorting, textile, screening) … | New processing line needing building works, installation, electrical and automation around the core machine. | UNCERTAIN: Line vendor likely fixed; ask about building, installation and electrical. |
| Renahav Sverige AB | Utökning av befintlig biogasproduktion samt uppgradering till biometan | 2026-04-02 | 54.6 MSEK grant (impl. 78–273 MSEK) | Biogas production, upgrading, liquefaction | Biogas process equipment (digesters, upgrading, liquefaction, CHP) … | Very large multi-year plant; civil, electrical, automation and gas handling are separate procurements from the process supplier. | UNCERTAIN: Process/EPC supplier may already be chosen; ask about civil, electrical, automation and upgrading packages. |
| Åkerholmens Lantbruk AB | CBG Åkerholmens Lantbruk | 2026-03-19 | 49.2 MSEK grant (impl. 70–246 MSEK) | Biogas production, upgrading, liquefaction | Biogas process equipment (digesters, upgrading, liquefaction, CHP) … | Very large multi-year plant; civil, electrical, automation and gas handling are separate procurements from the process supplier. | UNCERTAIN: Process/EPC supplier may already be chosen; ask about civil, electrical, automation and upgrading packages. |
| Orkla Snacks Sverige AB | Energikonvertering industri | 2025-12-18 | 45.9 MSEK grant (impl. 66–230 MSEK) | Industrial process / fuel conversion | Industrial energy engineering … | Multi-component industrial conversion: equipment plus piping, electrical, automation and installation; long lead times. | UNCERTAIN: Main equipment (boiler/furnace) probably quoted by the eventual supplier; ask which sub-scopes are still open. |
| Alvesta Biogas AB | Utbyggnad Biogasanläggning | 2026-02-26 | 29.1 MSEK grant (impl. 42–145 MSEK) | Biogas production, upgrading, liquefaction | Biogas process equipment (digesters, upgrading, liquefaction, CHP) … | Very large multi-year plant; civil, electrical, automation and gas handling are separate procurements from the process supplier. | UNCERTAIN: Process/EPC supplier may already be chosen; ask about civil, electrical, automation and upgrading packages. |
| Aluminium Casting Unnaryd | Flexibel smältning för pressgjuten aluminium med lågt klimatavtryck ge | 2026-04-30 | 28.4 MSEK grant (impl. 41–142 MSEK) | Industrial process / fuel conversion | Industrial energy engineering … | Multi-component industrial conversion: equipment plus piping, electrical, automation and installation; long lead times. | UNCERTAIN: Main equipment (boiler/furnace) probably quoted by the eventual supplier; ask which sub-scopes are still open. |
| Biogas Västra Skaraborg AB | Förvätskning av koldioxid från biogasproduktion | 2026-04-09 | 27.0 MSEK grant (impl. 39–135 MSEK) | Biogas production, upgrading, liquefaction | Biogas process equipment (digesters, upgrading, liquefaction, CHP) … | Very large multi-year plant; civil, electrical, automation and gas handling are separate procurements from the process supplier. | UNCERTAIN: Process/EPC supplier may already be chosen; ask about civil, electrical, automation and upgrading packages. |
| Lidköping Energi AB | 20GWh säsongslager för tillvaratagande av spillvärme | 2026-06-25 | 26.1 MSEK grant (impl. 37–130 MSEK) | Waste heat recovery and heat storage | Heat exchangers … | Engineering-heavy energy project with heat exchangers, storage and controls. | LIKELY_OPEN: Whether engineering and equipment were bundled in one quotation. |
| Tegelmöllan AB | Etablering av tegelåterbruk i Sköldinge | 2026-03-05 | 20.3 MSEK grant (impl. 29–102 MSEK) | Circular material flows / recycling lines | Specialised process machinery (sorting, textile, screening) … | New processing line needing building works, installation, electrical and automation around the core machine. | UNCERTAIN: Line vendor likely fixed; ask about building, installation and electrical. |
| Skanska Industrial Solutions AB | Våtsikt Sälgsjön | 2026-02-05 | 19.7 MSEK grant (impl. 28–98 MSEK) | Circular material flows / recycling lines | Specialised process machinery (sorting, textile, screening) … | New processing line needing building works, installation, electrical and automation around the core machine. | UNCERTAIN: Line vendor likely fixed; ask about building, installation and electrical. |
| Renasens AB | Textilåtervinning av textilavfall med blandat material (polyester och  | 2026-04-29 | 19.5 MSEK grant (impl. 28–97 MSEK) | Circular material flows / recycling lines | Specialised process machinery (sorting, textile, screening) … | New processing line needing building works, installation, electrical and automation around the core machine. | UNCERTAIN: Line vendor likely fixed; ask about building, installation and electrical. |
| Linde Gas AB | Utökad kapacitet för produktion av grön vätgas i Borlänge | 2026-06-11 | 17.9 MSEK grant (impl. 26–90 MSEK) | Hydrogen production | Electrolysers … | Electrolyser plant with substantial balance-of-plant, grid and civil scope. | UNCERTAIN: Electrolyser vendor likely fixed; ask about balance of plant and installation. |
| Solör Bioenergi Värme AB | Konvertera från gasol till träpellets Bewi Värnamo. | 2026-03-06 | 17.6 MSEK grant (impl. 25–88 MSEK) | Industrial process / fuel conversion | Industrial energy engineering … | Multi-component industrial conversion: equipment plus piping, electrical, automation and installation; long lead times. | UNCERTAIN: Main equipment (boiler/furnace) probably quoted by the eventual supplier; ask which sub-scopes are still open. |
| Benders Sverige AB | Edsvära Takpannor | 2026-03-20 | 13.7 MSEK grant (impl. 20–69 MSEK) | Industrial process / fuel conversion | Industrial energy engineering … | Multi-component industrial conversion: equipment plus piping, electrical, automation and installation; long lead times. | UNCERTAIN: Main equipment (boiler/furnace) probably quoted by the eventual supplier; ask which sub-scopes are still open. |
| St1 Sverige AB | BPU- Rening av matfettsavfall | 2026-04-13 | 12.1 MSEK grant (impl. 17–60 MSEK) | Circular material flows / recycling lines | Specialised process machinery (sorting, textile, screening) … | New processing line needing building works, installation, electrical and automation around the core machine. | UNCERTAIN: Line vendor likely fixed; ask about building, installation and electrical. |
| Optigas Energy Sweden AB | Avfallshantering sand/sediment från biogasproduktion -> Ökad produktio | 2026-04-30 | 11.7 MSEK grant (impl. 17–58 MSEK) | Biogas production, upgrading, liquefaction | Biogas process equipment (digesters, upgrading, liquefaction, CHP) … | Very large multi-year plant; civil, electrical, automation and gas handling are separate procurements from the process supplier. | UNCERTAIN: Process/EPC supplier may already be chosen; ask about civil, electrical, automation and upgrading packages. |
| Småländska Bränslen AB | Biogastankställe i Älmhult | 2026-05-13 | 11.5 MSEK grant (impl. 16–58 MSEK) | Biogas / LBG filling stations | Compressors, dispensers, storage … | Site build with compressors, dispensers, storage and civil works. | UNCERTAIN: Operator's equipment partner likely fixed; civil per site may be open. |
| Småländska Bränslen AB | Biogastankställe i Växjö | 2026-05-13 | 11.0 MSEK grant (impl. 16–55 MSEK) | Biogas / LBG filling stations | Compressors, dispensers, storage … | Site build with compressors, dispensers, storage and civil works. | UNCERTAIN: Operator's equipment partner likely fixed; civil per site may be open. |
| Söderhalls Renhållningsverk Aktiebolag (SÖRAB) | Optisk utsortering av plast från restavfall | 2026-02-12 | 10.0 MSEK grant (impl. 14–50 MSEK) | Circular material flows / recycling lines | Specialised process machinery (sorting, textile, screening) … | New processing line needing building works, installation, electrical and automation around the core machine. | UNCERTAIN: Line vendor likely fixed; ask about building, installation and electrical. |
| Skorpvägen Förvaltning AB | Ersätta befintliga tunnelugnar mot nya energieffektiva | 2025-12-11 | 7.3 MSEK grant (impl. 10–36 MSEK) | Industrial process / fuel conversion | Industrial energy engineering … | Multi-component industrial conversion: equipment plus piping, electrical, automation and installation; long lead times. | UNCERTAIN: Main equipment (boiler/furnace) probably quoted by the eventual supplier; ask which sub-scopes are still open. |
| Reelab AB | Maskinutrustning för att göra återvunnen plastråvara livsmedelsgodkänd | 2026-02-12 | 6.3 MSEK grant (impl. 9–32 MSEK) | Circular material flows / recycling lines | Specialised process machinery (sorting, textile, screening) … | New processing line needing building works, installation, electrical and automation around the core machine. | UNCERTAIN: Line vendor likely fixed; ask about building, installation and electrical. |
| Gunnarshögs Gård AB | Innovativ pressningsteknik för minskad användning av rapsfrö | 2026-04-15 | 5.6 MSEK grant (impl. 8–28 MSEK) | Industrial process / fuel conversion | Industrial energy engineering … | Multi-component industrial conversion: equipment plus piping, electrical, automation and installation; long lead times. | UNCERTAIN: Main equipment (boiler/furnace) probably quoted by the eventual supplier; ask which sub-scopes are still open. |
| Elis Textil Service AB | Pelletspanna Vaggeryd | 2025-12-15 | 5.3 MSEK grant (impl. 8–27 MSEK) | Building heating conversion (oil/gas to heat pump, pellets, chips, district heating) | HVAC / heat pump installers … | Larger property heating conversion; installer capacity and drilling/boiler supply. | LIKELY_SELECTED: Installer that quoted is probably contracted; ask whether drilling/electrical were separate. |

Evaluation gate for the answers: more than 70 % "already contracted"
kills Klimatklivet as a core buying signal; 40–70 % limits it to specific
project types or components; under 40 % makes it a potentially strong
signal. Components still open after the decision should be counted
separately from the main equipment.

## 9. Recommendation

**CONTINUE_WITH_LIMITATIONS**, narrowly: run the shortlist calls before
any further engineering. The source is cheap to keep ingesting (one file,
twice a year, idempotent) and the 103 HIGH/MEDIUM non-charging
projects per 18 months are a small but high-ticket population. But the
signal cannot be a real-time trigger: with a semi-annual file, the
realistic product is a **project list with a known age**, usable for
suppliers of secondary scopes on large projects, not a "why now" alert.
If the calls show the main equipment is contracted in most cases and the
secondary scopes are not reachable through the applicant, stop.
