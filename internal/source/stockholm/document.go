// Package stockholm reads Stockholms stad's public building and planning
// service (Bygg- och plantjänsten) and turns explicitly failed inspection
// certificates of lifts and other motorised building equipment into public
// events.
//
// The package owns every source-specific concern: request construction,
// the embedded-JSON and HTML parsing of the search and case pages, source
// identifiers, the deterministic failure rule and the open-case refresh.
// It decides nothing about commercial value. See docs/sources/stockholm.md
// for the verified behaviour of the source and
// docs/evaluations/stockholm-motorised-equipment-inspections.md for the
// evidence behind the rule.
package stockholm

import "github.com/sK4rdell/signal-engine/internal/publicevent"

// Source-specific constants. The codes are the values of the search form,
// verified against the live service on 2026-09-25.
const (
	// Source is the name recorded on observations and events.
	Source = publicevent.SourceStockholm
	// MunicipalityCode is Stockholms stad's four-digit kommunkod.
	MunicipalityCode = "0180"
	// SearchPath is the case search page, relative to the base URL.
	SearchPath = "/Byggochplantjansten/arendeochhandlingar"
	// CasePath is the case page, followed by the case's RecNo.
	CasePath = "/Byggochplantjansten/arende/arende/"
	// CaseTypeFunctionalControl is the ärendegrupp "Funktionskontroll,
	// Tillstånd" (CaseTypeRecNo).
	CaseTypeFunctionalControl = "200001"
	// ClassCodeMotorisedEquipment is the diarieplansbeteckning "7.2 Hantera
	// tillsyn av hissar och andra motordrivna anordningar" (JournalPlanCode),
	// in use for cases started from 2024. Older cases carry code 587.
	ClassCodeMotorisedEquipment = "7.2"
	// CategoryIncoming is the document category of documents the city
	// received, including the inspection bodies' certificates.
	CategoryIncoming = "Skrivelse In"
)

// CaseSummary is one row of the case search: the fields the embedded
// result JSON carries for a case.
type CaseSummary struct {
	// RecNo is the stable numeric case identifier used in case URLs.
	RecNo string `json:"recno"`
	// DiaryNumber is the diarienummer ("2026-15984").
	DiaryNumber string `json:"diary_number"`
	// Title is the ärendemening.
	Title string `json:"title"`
	// StartedOn is ärendestart (YYYY-MM-DD).
	StartedOn string `json:"started_on"`
	// PropertyDesignation is the fastighetsbeteckning as the source wrote it.
	PropertyDesignation string `json:"property_designation"`
	// Address is the source's street address, "" when it printed none.
	Address string `json:"address"`
	// CaseGroup is the ärendegrupp label.
	CaseGroup string `json:"case_group"`
	// Archived is the source's IsEarchive flag; it selects the case page's
	// data source.
	Archived bool `json:"archived"`
}

// Case is one case page: the source record whose current state is stored
// as an observation. Field names follow the source's vocabulary; the JSON
// form is the observation payload. closed_on is always present so the
// open-case refresh can query it.
type Case struct {
	RecNo       string `json:"recno"`
	DiaryNumber string `json:"diary_number"`
	// CaseGroup is ärendegrupp; ClassCode is diarieplansbeteckning.
	CaseGroup string `json:"case_group"`
	ClassCode string `json:"class_code"`
	// PropertyDesignation is fastighetsbeteckning as the source wrote it.
	PropertyDesignation string `json:"property_designation"`
	// Address is the source's address; "" when the page says it is missing.
	Address string `json:"address"`
	// District is stadsdel.
	District string `json:"district"`
	// StartedOn is ärendestart; ClosedOn is ärendeavslut, "" while open.
	StartedOn string `json:"started_on"`
	ClosedOn  string `json:"closed_on"`
	// Officer is handläggare as printed ("Ej utsedd" while unassigned).
	Officer string `json:"officer"`
	// Title is ärendemening.
	Title string `json:"title"`
	// DocumentCount is the count the page prints; it must equal the number
	// of documents parsed.
	DocumentCount int        `json:"document_count"`
	Documents     []Document `json:"documents"`
	// Archived records which data source the page was read from.
	Archived bool `json:"archived"`
	// CaseURL is the absolute URL of the case page.
	CaseURL string `json:"case_url"`

	// RawHTML is the page, kept on the observation for re-parsing. It is
	// not part of the payload.
	RawHTML string `json:"-"`
}

// Document is one row of a case's document list.
type Document struct {
	// Description is the document description (beskrivning), verbatim.
	Description string `json:"description"`
	// Category is dokumenttyp, e.g. "Skrivelse In".
	Category string `json:"category"`
	// Timestamp is the source's date-time ("2026-09-23T13:29:08").
	Timestamp string `json:"timestamp"`
	// FileName is the attached file, null on every row observed so far.
	FileName *string `json:"file_name"`
}
