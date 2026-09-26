# Bolagsbild Signal Catalog

Last updated: 2026-09-26

This document is the working catalog of candidate public-data signals for Bolagsbild.

Its purpose is to prevent us from:

- rediscovering the same signals repeatedly,
- spending engineering effort before signal quality is proven,
- confusing "interesting public data" with commercially useful events,
- over-focusing on enforcement signals when project, money and deadline signals may be equally or more valuable.

This is a discovery document, not a production specification.

---

# 1. Signal thesis

A strong Bolagsbild signal should answer:

> Why is this company commercially relevant **right now**?

The preferred pattern is:

```text
observable public event
        ↓
something materially changed
        ↓
predictable need / spend within ~0–180 days
        ↓
identifiable supplier category
```

The strongest signals normally fall into one of five families.

## 1.1 Problem / remediation

Something is wrong and must be corrected.

Examples:

```text
equipment failed inspection
→ repair/service required
```

```text
regulatory requirement missing
→ specific compliance service required
```

## 1.2 Project starting

A real project is about to consume budget.

Examples:

```text
major construction project announced
→ contractors / equipment / services required
```

```text
public contract awarded
→ delivery capacity must increase
```

## 1.3 Company change

The company itself is changing.

Examples:

```text
first employees hired
→ company moving from founder-stage to organisation
```

```text
new workplace registered
→ geographical expansion
```

## 1.4 Market entry / licence

The company is becoming operational in a new regulated market.

Examples:

```text
pharmacy licence becomes valid
→ pharmacy opening
```

```text
new electrical contractor registration
→ new service capability
```

## 1.5 Deadline / renewal

A future mandatory event can be predicted.

Examples:

```text
energy declaration expires in 90 days
→ certified energy expert required
```

These can be particularly valuable because Bolagsbild may observe the need **before the buyer starts purchasing**.

---

# 2. Evaluation framework

Every candidate should be evaluated on the same dimensions.

## 2.1 Trigger precision

Can we determine exactly what happened?

Prefer:

```text
failed recurring inspection – vehicle lift
```

over:

```text
inspection performed
```

## 2.2 Timing

How close is the public event to the commercial buying window?

Preferred:

```text
before purchase decision
```

Acceptable:

```text
while remediation / procurement is ongoing
```

Weak:

```text
months after the problem was solved
```

## 2.3 Entity resolution

Prefer deterministic identifiers:

1. organisation number
2. CFAR / workplace ID
3. registered property + deterministic owner
4. other stable official identifier

Avoid fuzzy company-name matching unless necessary.

## 2.4 Supplier mapping

We should be able to answer:

> Who could reasonably sell something because this happened?

And:

> Why would the company need that supplier now?

## 2.5 Acquisition economics

Preferred source access:

1. public structured API
2. structured download
3. stable machine-readable web endpoint
4. stable HTML search
5. paid API with reasonable economics

Avoid:

- manual document requests
- email workflows
- BankID
- one-document-at-a-time ordering
- CAPTCHA-dependent acquisition
- PDF archives as the primary ingestion mechanism

## 2.6 Volume

A great signal can still be too small to justify a standalone product feature.

Low-volume deterministic signals may instead become cheap add-ons to an existing source adapter.

## 2.7 Incumbent risk

Ask:

> Has the buyer probably already selected the supplier before we observe the event?

This is especially important for:

- failed inspections,
- construction projects,
- recurring maintenance,
- regulated professional services.

---

# 3. Current high-priority discovery candidates

These are the candidates currently most worth deeper investigation.

| Candidate | Trigger | National | Access | Supplier mapping | Main unknown | Status |
|---|---|---:|---:|---:|---|---|
| AV construction-site notification | large construction project will start | yes | excellent | strong | lead time + organisation role | **SPIKE NEXT** |
| Post- och Inrikes Tidningar | official legal / permit event | yes | unknown | varies | event taxonomy + machine access | DISCOVER |
| Bolagsverket change notifications | registered company change | yes | paid | strong for some events | actual `sakfråga` taxonomy | DISCOVER |
| Energy declaration expiry | mandatory renewal approaching | yes | good | excellent | property → company owner | BACKLOG / HIGH |
| Public contract awarded | company won funded work | yes | medium | strong | winner-level freshness/access | BACKLOG / HIGH |
| Vinnova financing awarded | project budget awarded | yes | excellent | medium | commercial lead quality | BACKLOG |
| Jobtech hiring acceleration | organisation is expanding | yes | excellent | medium | orgnr coverage + recipe quality | BACKLOG |
| Producer responsibility registration | company entering product category | yes | promising | strong | bulk/change access | BACKLOG |
| Trademark filed | new brand/product activity | yes | good | medium | owner matching + buying intent | BACKLOG |
| New regulated licence | business becoming operational | yes | varies | medium/strong | volume by vertical | BACKLOG |

---

# 4. Candidate: AV construction-site notification

## Hypothesis

Arbetsmiljöverket receives a pre-notification for sufficiently large construction projects before work starts.

Potential signal:

```text
CONSTRUCTION_PROJECT_STARTING
```

Possible fields exposed through public metadata include:

- organisation
- organisation number
- project address
- municipality / county
- project description
- planned start date
- planned end date

Example project descriptions observed include:

- new construction
- elderly-care / hospital construction
- pipe replacement
- roof replacement
- ground / water / sewage work
- renovation
- demolition
- parking structure
- industrial building
- window renovation

This makes classification into supplier-specific recipes plausible.

## Potential recipes

```text
roof replacement
→ scaffolding
→ fall protection
→ lift rental
→ roofing suppliers
```

```text
pipe replacement
→ plumbing
→ construction waste
→ cleaning
→ temporary facilities
```

```text
ground / water / sewage
→ machinery
→ containers
→ traffic-control services
→ aggregate/material suppliers
```

```text
new major building project
→ temporary buildings
→ site security
→ access control
→ construction cleaning
→ equipment rental
```

## Possible derived signals

```text
CONSTRUCTION_PROJECT_STARTING
```

```text
CONSTRUCTION_PROJECT_SCHEDULE_CHANGED
```

```text
CONSTRUCTION_PROJECT_NEARING_COMPLETION
```

## Main questions

Measure over at least 12 months:

- total notifications
- unique projects
- unique organisations
- organisation-number coverage
- project-type distribution
- geography
- registration date → planned start date
- planned project duration
- how often the organisation is:
  - developer
  - main contractor
  - other actor
- whether the organisation shown is commercially contactable
- duplicate / updated notifications

Most important metric:

```text
registration_date → planned_start_date
```

We need to know whether Bolagsbild normally sees the project before meaningful supplier selection has closed.

### Status

**Highest-priority next spike.**

No new source adapter should be required if the relevant document type uses the existing Arbetsmiljöverket diary mechanics.

---

# 5. Candidate source family: Post- och Inrikes Tidningar

Post- och Inrikes Tidningar should be treated as a potential **source family**, not as one signal.

It publishes official notices concerning companies and public decisions.

Potential signal families include:

## 5.1 Building permit granted

```text
BUILDING_PERMIT_GRANTED
```

Potential thesis:

```text
permit granted
→ project likely to proceed
→ supplier window may still be open
```

This may appear materially earlier than a construction-site notification.

If sufficiently structured, this could become one of the strongest construction signals.

### Questions

- Is the data machine-readable?
- Can notices be polled incrementally?
- Are property/address fields structured?
- Is applicant / legal entity available?
- Does the notice distinguish granted permits from applications?
- What is the delay from decision to publication?
- What share can be mapped to a company/property owner?

---

## 5.2 Forced-liquidation deficiency

Possible notices may identify companies that must correct specific corporate deficiencies.

Potential examples:

```text
AUDITOR_REQUIRED_BUT_MISSING
```

```text
BOARD_MEMBER_REQUIRED
```

```text
CEO_REQUIRED
```

```text
SERVICE_RECIPIENT_REQUIRED
```

This is potentially better than predicting the problem ourselves because:

```text
Bolagsverket has explicitly determined that the deficiency exists
```

Supplier mapping could be exceptionally clear for missing auditors.

### Questions

- Does the public notice expose the exact reason?
- Is organisation number available?
- How early is the notice relative to liquidation?
- How many cases/year exist by reason?
- Is remediation still commercially open when published?

---

## 5.3 Other company events

Investigate:

```text
LIQUIDATION_STARTED
CREDITORS_CALLED
MERGER_PROCESS_STARTED
CAPITAL_REDUCTION_STARTED
BANKRUPTCY_STARTED
```

Not all of these are necessarily sales leads.

They may instead become company-intelligence events.

---

# 6. Candidate source family: Bolagsverket change notifications

Bolagsverket now supports notifications about company changes.

Notifications contain at least:

- organisation number
- legal form
- case number
- `sakfråga`

The critical discovery task is:

> What real `sakfråga` values occur?

Do not design signals before this vocabulary has been enumerated.

Potential signals include:

```text
AUDITOR_ADDED
AUDITOR_REMOVED
CEO_CHANGED
BOARD_CHANGED
COMPANY_ENTERED_LIQUIDATION
MERGER_STARTED
FINANCIAL_REPORT_FILED
BUSINESS_MORTGAGE_CHANGED
```

Potentially important higher-order recipes:

```text
company exceeded audit thresholds
+
auditor absent
→ auditor demand
```

```text
new business mortgage
+
recent growth
→ likely financing event
```

```text
new CEO
+
rapid hiring
→ organisational transition
```

## Acquisition concern

Bolagsverket's richer API access is paid.

Therefore every signal must justify:

```text
API cost
vs
number of commercially usable events
vs
expected customer value
```

Avoid using expensive calls for signals that could be derived from bulk/open datasets.

---

# 7. Candidate: energy declaration expiry

Boverket exposes structured energy-declaration data.

Energy declarations have a finite validity period.

Potential signal:

```text
ENERGY_DECLARATION_EXPIRING
```

Potential thresholds:

```text
180 days
90 days
30 days
```

Commercial chain:

```text
declaration nearing expiry
→ new valid declaration required
→ certified energy expert
```

This is attractive because the buying need is predictable before the deadline.

## Main blocker

The entity is primarily a **property/building**, not the company.

Required enrichment:

```text
building/property
→ legal owner
→ organisation number
```

This likely depends on Lantmäteriet/property ownership data.

## Status

**High priority once deterministic property-owner matching exists.**

---

# 8. Candidate: public contract awarded

Potential signal:

```text
PUBLIC_CONTRACT_AWARDED
```

Stronger derived recipe:

```text
SMALL_COMPANY_WON_LARGE_CONTRACT
```

Example thesis:

```text
company revenue: 12 MSEK
new contract: 20 MSEK
→ material delivery-capacity event
```

Possible needs:

- recruitment
- subcontractors
- vehicles
- equipment
- financing
- insurance
- payroll
- project accounting

The important signal is not simply:

```text
company won contract
```

but:

```text
contract is material relative to company size
```

## Questions

- How quickly is award data available?
- Is winner organisation number available?
- Is awarded contract value available?
- Can framework contracts be distinguished from committed spend?
- Can contract value be compared to turnover?
- Is data incremental?

---

# 9. Candidate: grant / project financing awarded

This should become a general source family.

Existing examples:

- Klimatklivet
- Vinnova
- other public financing programs

Potential canonical event:

```text
PROJECT_FINANCING_AWARDED
```

Then enrich with:

- funding amount
- total project budget
- project period
- programme
- project subject
- participating organisations

Derived signal:

```text
MATERIAL_PROJECT_FINANCING_AWARDED
```

where financing is material relative to company size.

Potential suppliers:

- recruitment
- consultants
- engineering
- laboratories
- prototypes
- project accounting
- IP/patent services
- equipment

## Vinnova

Vinnova is particularly attractive because its funded-project data is openly available and suitable for incremental ingestion.

## Risk

Receiving money does not automatically mean an external supplier is still being selected.

Lead quality must be tested on real projects.

---

# 10. Candidate: Jobtech hiring signals

Raw job advertisements are weak by themselves.

Changes in hiring behaviour are more interesting.

Potential signals:

```text
COMPANY_STARTED_HIRING
HIRING_ACCELERATED
FIRST_SALES_HIRE
FIRST_FINANCE_HIRE
FIRST_IT_HIRE
FIRST_MANAGER_HIRE
NEW_LOCATION_HIRING
NEW_ROLE_FAMILY_HIRING
```

Examples:

```text
0 ads for 12 months
→ 4 ads in 30 days
```

```text
10-person company
→ first sales manager
```

```text
company historically recruits only in Gothenburg
→ suddenly recruits in Malmö
```

These are better treated as **company-change signals** than lead events tied to one supplier.

Potential users:

- recruiters
- staffing companies
- office providers
- HR/payroll systems
- sales intelligence users

## Evaluation questions

- % ads with organisation number
- deterministic match rate
- unique companies/month
- historical baseline availability
- false positives from recruitment agencies
- multi-location companies
- meaningful hiring-burst thresholds

---

# 11. Candidate: producer responsibility

Naturvårdsverket maintains producer-responsibility registers for product categories such as:

- packaging
- batteries
- electronics
- tyres
- vehicles

Potential signal:

```text
PRODUCER_RESPONSIBILITY_REGISTERED
```

Interpretation:

```text
company has started supplying/importing a regulated product category
```

Possible supplier mapping:

- EPR/compliance services
- recycling
- packaging
- logistics
- customs
- reporting services

Potentially stronger derived signal:

```text
company appears commercially active in regulated product category
+
company missing from required register
→ compliance gap
```

This needs careful legal and data-quality evaluation before being treated as a lead.

---

# 12. Candidate: trademark events

PRV's trademark database can support several company-intelligence events.

Potential signals:

```text
TRADEMARK_FILED
TRADEMARK_REGISTERED
TRADEMARK_RENEWAL_DUE
TRADEMARK_TRANSFERRED
```

The most interesting may be:

```text
TRADEMARK_FILED
```

because it may proxy:

- new brand
- new product
- rebranding
- market expansion

Potential suppliers:

- branding agencies
- web agencies
- packaging
- IP/legal
- marketing

## Risks

- many applications are defensive
- applicant may be an IP holding company
- buying activity may already be underway
- ownership matching may be non-trivial

---

# 13. Candidate: regulated-business market entry

Several official registers may expose a company becoming legally able to start a regulated activity.

These signals are low-volume but potentially high-value.

## 13.1 Pharmacies

Potential:

```text
PHARMACY_OPENING
```

Useful metadata can include:

- organisation number
- holder
- address
- municipality
- valid-from date
- licence/case ID

Potential supplier categories:

- security
- alarms
- refrigeration
- cleaning
- IT
- staffing
- pharmacy interiors

---

## 13.2 Care / social-service operations

Possible IVO-derived events:

```text
CARE_FACILITY_APPROVED
CARE_FACILITY_OPENING
CARE_FACILITY_CHANGED
CARE_FACILITY_TEMPORARILY_CLOSED
```

Potential suppliers:

- staffing
- food
- cleaning
- access control
- alarm systems
- documentation systems
- training

Access and change history need evaluation.

---

## 13.3 Financial-services licences

Possible event:

```text
FINANCIAL_LICENSE_GRANTED
```

Supplier categories:

- AML/KYC
- cybersecurity
- audit
- compliance
- cloud
- insurance

Very low volume, but potentially high contract values.

---

## 13.4 Electrical-contractor registration

Potential event:

```text
ELECTRICAL_CONTRACTOR_REGISTERED
```

or:

```text
ELECTRICAL_ACTIVITY_SCOPE_CHANGED
```

Alone, this may have weak supplier mapping.

Combined with company lifecycle:

```text
recently registered company
+
electrical-contractor registration
→ company likely becoming operational
```

---

# 14. Candidate: Swedac accreditation changes

Potential:

```text
ACCREDITATION_GRANTED
ACCREDITATION_SCOPE_EXPANDED
ACCREDITATION_WITHDRAWN
```

Interpretation:

```text
company becomes authorised to sell a new accredited service
```

This is mainly a company-intelligence signal.

Potential customers:

- competitors
- equipment suppliers
- industry analysts
- sales-intelligence users

Likely too niche for standalone prioritisation.

---

# 15. Candidate: pharmaceutical supply interruption

Läkemedelsverket publishes structured information about medicine shortages / sales interruptions.

Potential event:

```text
MEDICINE_SUPPLY_SHORTAGE
```

This can create commercial opportunities for:

- distributors
- alternative suppliers
- procurement
- pharmacies

However, this is closer to **vertical market intelligence** than general B2B company intelligence.

Keep in the catalog, but do not prioritise for the initial Bolagsbild product.

---

# 16. Candidate: industrial environmental change

Naturvårdsverket publishes emissions/environmental data for large facilities.

Possible derived events:

```text
INDUSTRIAL_EMISSIONS_INCREASED
HAZARDOUS_WASTE_INCREASED
PRODUCTION_PROXY_INCREASED
```

Potential interpretation:

```text
facility activity materially changed
```

Potential suppliers:

- environmental consulting
- filtration
- waste handling
- process equipment
- measurement services

Main weakness:

**annual reporting latency**.

Better suited for enrichment than "why now" alerts.

---

# 17. Existing validated / implemented signals

These should not be rediscovered.

## 17.1 Arbetsmiljöverket recurring inspection failure

Canonical event:

```text
WORK_EQUIPMENT_INSPECTION_FAILED
```

Source:

```text
6.1-49 Intyg återkommande besiktning
```

Validated annual population:

- ~1,101 lifting-equipment failures
- ~578 pressure-equipment failures
- high organisation-number coverage
- high CFAR coverage

Examples include:

- vehicle lifts
- cranes
- mobile platforms
- dock levellers
- compressors
- pressure receivers
- boilers
- cooking kettles
- refrigeration equipment

This remains one of the strongest deterministic Bolagsbild signal families.

---

## 17.2 Arbetsmiljöverket sanction families

Already evaluated:

```text
lifting inspection sanction
pressure equipment control sanction
forklift permit sanction
fall protection
work-environment plan
chemical training
medical fitness certificate
scaffolding
asbestos
```

Important conclusion:

The sanction signal is generally **late**.

Use it only when:

- no earlier deterministic event exists, or
- it is a cheap add-on from an already ingested feed.

Low-volume families should not receive standalone research projects.

---

## 17.3 Asbestos training

Already measured.

Approximate annual population:

```text
all asbestos sanction cases: ~20
asbestos training: ~4
```

Verdict:

```text
deterministic
supplier-mappable
too low-volume for standalone work
```

Keep as a cheap future add-on.

---

## 17.4 Stockholm motorised-equipment inspections

Strong local signal.

Problem:

Municipal replication failed.

Eleven municipalities were investigated and no second municipality met the same combination of:

- freshness
- automation
- useful metadata
- sufficient volume

Conclusion:

```text
Stockholm remains a standalone source.
```

Do not create a generic municipality-source abstraction based on the Stockholm implementation.

---

# 18. Sources / signals currently deprioritised

## Municipal food-control deviations

Signal quality could be high:

```text
inspection
→ concrete food-safety deviation
→ remediation
```

Possible categories:

- pest
- refrigeration
- hygiene
- allergen
- traceability
- cleaning

But acquisition appears fragmented across municipal systems.

Status:

```text
BACKLOG
```

Revisit if a national or reusable platform-specific source is discovered.

---

## Vehicle inspection failures

Potential signal:

```text
company vehicle
+
inspection status = failed / prohibition
```

Strengths:

- clean semantics
- potentially large volume
- company-owned commercial vehicles

Problems:

- conditional data access
- high incumbent workshop risk
- much stronger if defect codes are available

Status:

```text
BACKLOG
```

---

## IMY sanctions

Strong semantics but low volume and late timing.

Status:

```text
LOW PRIORITY
```

---

## Consumer-protection enforcement

Can be deterministic but normally appears after a long regulatory process.

Better as intelligence than immediate lead generation.

Status:

```text
LOW PRIORITY
```

---

## Product recalls / market surveillance

Strong event when it occurs.

Generally low volume.

Potentially useful as a future vertical intelligence signal.

---

# 19. Signal combinations

The long-term value of Bolagsbild should not come only from raw source events.

The strongest opportunities may be **recipes combining several primitives**.

Examples:

## New company becoming real

```text
company_registered
+
real SNI
+
first workplace
+
first job advertisement
```

Interpretation:

```text
company is moving from legal entity to operating business
```

---

## Construction-company expansion

```text
revenue increased
+
new workplace
+
hiring accelerated
+
new major construction project
```

Interpretation:

```text
material expansion
```

---

## Contract creates capacity pressure

```text
public_contract_awarded
+
contract value > material % of revenue
+
current employee count low
```

Interpretation:

```text
company likely needs capacity / financing / subcontractors
```

---

## New geography

```text
new workplace
OR
first job ad in new municipality
OR
construction project in new municipality
```

Interpretation:

```text
company expanding geographically
```

---

## Property compliance demand

```text
property owned by company
+
energy declaration expires in 90 days
```

Interpretation:

```text
specific mandatory service must soon be purchased
```

---

# 20. Source families worth systematic enumeration

Instead of repeatedly searching the web for individual ideas, discovery should now proceed source-family by source-family.

## Tier 1

Fully enumerate:

1. Arbetsmiljöverket
2. Bolagsverket
3. Post- och Inrikes Tidningar
4. Boverket
5. Upphandlingsmyndigheten / procurement data
6. Naturvårdsverket
7. PRV
8. Arbetsförmedlingen / Jobtech
9. Lantmäteriet

For each source:

```text
list all datasets / document types / event types
↓
identify fields and identifiers
↓
identify potentially actionable changes
↓
map each to possible spend / supplier
↓
measure actual population
```

Do not only search for signals we have already imagined.

The source's own event vocabulary may reveal better signals.

This is how `Intyg återkommande besiktning` was discovered.

---

## Tier 2

Then enumerate regulated verticals:

- IVO
- Läkemedelsverket
- Finansinspektionen
- Elsäkerhetsverket
- Swedac
- Transportstyrelsen
- Kemikalieinspektionen
- Jordbruksverket
- Energimarknadsinspektionen
- MSB / fire / dangerous-goods related registers
- environmental permit registers
- county administrative boards

---

# 21. Discovery rule going forward

For every new source, do not begin with:

> What API does this authority have?

Begin with:

> What economically meaningful events does this authority observe before everyone else?

Then ask:

```text
1. What happened?
2. Is it deterministic?
3. Who is the company?
4. When does it become public?
5. What might the company buy because of it?
6. Is the purchase still open?
7. Who sells that?
8. How many events/year?
9. Can we ingest it automatically?
10. Can we combine it with another Bolagsbild primitive?
```

Only after those questions pass should implementation begin.

---

# 22. Current recommended discovery sequence

## Next

### A. AV construction-site notification

Measure:

- 12-month population
- organisation coverage
- project taxonomy
- lead-time distribution
- organisation role
- update/change events
- 50 real projects against supplier recipes

---

### B. Post- och Inrikes Tidningar enumeration

Do not investigate one hypothesis only.

Enumerate:

```text
all notice types
available fields
identifier quality
volume
freshness
incremental acquisition
```

Then identify actionable signal families.

Specific focus:

```text
BUILDING_PERMIT_GRANTED
AUDITOR_REQUIRED_BUT_MISSING
LIQUIDATION_DEFICIENCY
MERGER_STARTED
```

---

### C. Bolagsverket `sakfråga` enumeration

Get a real vocabulary of change-notification event types.

Produce:

```text
sakfråga
count
example
commercial interpretation
possible supplier
timing
```

This should determine whether Bolagsverket's paid change feed is worth the acquisition cost.

---

# 23. Permanent signal-backlog schema

Every candidate added to the backlog should contain:

```text
Signal:
Source:
Source event:
Status:

Observable event:
Company identifier:
Observed at:
Expected latency:

Why now:
Likely spend:
Supplier category:

Estimated annual volume:
Geographical coverage:

Access:
Acquisition cost:
Incremental ingestion possible:

Metadata sufficient:
Document content required:

Incumbent risk:
Timing risk:
Legal/data risk:

Existing adapter:
Implementation complexity:

Evidence:
Open questions:

Next experiment:
Decision:
```

Possible statuses:

```text
IDEA
DISCOVER
SPIKE
PROMISING
IMPLEMENT
IMPLEMENTED
BACKLOG
LOW_PRIORITY
NO_GO
```

---

# 24. Guiding principle

The best signal is not necessarily the one with the most severe event.

The best signal is the one where Bolagsbild can observe a commercial need **before the supplier relationship is decided**.

Optimise for:

```text
observable event
+
deterministic company
+
predictable spend
+
clear supplier
+
good timing
+
repeatable acquisition
```

rather than simply:

```text
interesting public data
```

The long-term moat should come from combining many boring public-data events into a coherent view of:

> **What is changing inside Swedish companies, and what are they likely to need next?**