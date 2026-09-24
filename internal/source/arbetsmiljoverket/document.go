// Package arbetsmiljoverket reads Arbetsmiljöverket's public web diary
// (webbdiarium) and turns inspection notices into public events.
//
// The package owns every source-specific concern: request construction,
// pagination, HTML parsing, source identifiers, dates and normalisation.
// It decides nothing about commercial value. See
// docs/sources/arbetsmiljoverket.md for the verified behaviour of the
// source.
package arbetsmiljoverket

import "github.com/sK4rdell/signal-engine/internal/publicevent"

// Source-specific constants. The codes are the values of the diary's form
// fields, verified against the live service.
const (
	// Source is the name recorded on observations and events.
	Source = publicevent.SourceArbetsmiljoverket
	// SearchPath is the web diary search page, relative to the base URL.
	SearchPath = "/om-oss/diarium-och-allmanna-handlingar/bestall-handlingar/"
	// SubjectAreaInspection is the ämnesområde code of "Bedriva inspektion".
	SubjectAreaInspection = "6.1"
	// DocumentTypeInspectionNotice is the handlingstyp code of
	// "Inspektionsmeddelande".
	DocumentTypeInspectionNotice = "6.1-23"
	// InspectionNoticeTypeName is the document type label shown on a row.
	InspectionNoticeTypeName = "Inspektionsmeddelande"
	// PageSize is the fixed number of rows per search page.
	PageSize = 10
)

// Document is one row of the diary's search result: a registered document
// (handling) in a case (ärende). Field names follow the source's
// vocabulary; the JSON form is what gets stored as the observation payload.
type Document struct {
	// DocumentNumber (handlingsnummer, "2026/060943-2") is the stable
	// source identifier: case number plus running number.
	DocumentNumber string `json:"document_number"`
	// DocumentType is the handlingstyp label, e.g. "Inspektionsmeddelande".
	DocumentType string `json:"document_type"`
	// DocumentDate is "handlingens datum" as published (YYYY-MM-DD): when
	// the document was sent, received or finalised.
	DocumentDate string `json:"document_date"`
	// Origin is "handlingens ursprung": Utgående, Inkommande or Upprättad.
	Origin string `json:"origin"`
	// SubjectArea is the ämnesområde label, e.g. "Bedriva inspektion".
	SubjectArea string `json:"subject_area"`
	// CaseNumber is the ärendenummer ("2026/060943").
	CaseNumber string `json:"case_number"`
	// CaseTitle is the case title: the inspection campaign or incident.
	CaseTitle string `json:"case_title"`
	// CaseStatus is Pågående or Avslutat.
	CaseStatus string `json:"case_status"`
	// OrganisationNumber as published (ten digits), "" when the row shows
	// no organisation.
	OrganisationNumber string `json:"organisation_number"`
	OrganisationName   string `json:"organisation_name"`
	// WorkplaceName is the arbetsställe; the source prints "Saknas" when
	// unknown, kept verbatim here.
	WorkplaceName string `json:"workplace_name"`
	// WorkplaceCFAR is the arbetsställenummer (eight digits).
	WorkplaceCFAR string `json:"workplace_cfar"`
	// CaseURL is the absolute URL of the case page.
	CaseURL string `json:"case_url"`

	// RawHTML is the row's markup, kept on the observation for re-parsing.
	// It is not part of the payload.
	RawHTML string `json:"-"`
}

// RowError is a row that could not be parsed. The page as a whole is still
// usable; the raw markup lets the failure be diagnosed.
type RowError struct {
	Index   int
	Err     error
	RawHTML string
}

// SearchPage is one parsed page of search results.
type SearchPage struct {
	// URL is the request that produced the page.
	URL       string
	Documents []Document
	RowErrors []RowError
	// Total is the result count the source reports, 0 when it is missing.
	Total int
}
