// Package klimatklivet reads Naturvårdsverket's published list of approved
// Klimatklivet applications (an Excel file linked from the results page)
// and turns each approved application into a public event.
//
// The package owns every source-specific concern: locating and downloading
// the dataset, parsing the workbook, row identity, dates, amounts and
// normalisation. It decides nothing about commercial value. See
// docs/sources/klimatklivet.md for the verified behaviour of the source.
package klimatklivet

import "github.com/sK4rdell/signal-engine/internal/publicevent"

const (
	// Source is the name recorded on observations and events.
	Source = publicevent.SourceKlimatklivet
	// ResultsPagePath is the page that links to the current dataset file.
	// The file URL itself changes with every publication.
	ResultsPagePath = "/amnesomraden/klimatomstallningen/klimatklivet/sa-fungerar-klimatklivet/resultat--hur-har-det-gatt-for-klimatklivet/"
	// DatasetRecordID is the source record id under which each published
	// version of the dataset file is recorded as an observation.
	DatasetRecordID = "dataset:beviljade-ansokningar"
	// PreferredSheet is read when present; otherwise the first sheet. The
	// three sheets of the published file hold the same rows sorted
	// differently.
	PreferredSheet = "Per åtgärdskategori"
)

// Application is one approved application (beviljad ansökan): one row of
// the dataset. Field names follow the source's column vocabulary; the JSON
// form is what gets stored as the observation payload.
type Application struct {
	// CaseNumber (ärendenummer, "NV-25-043146" or "KKL-02186-2017") is the
	// stable source identifier, trimmed.
	CaseNumber       string `json:"case_number"`
	OrganisationName string `json:"organisation_name"`
	// Title is the applicant's short project title (rubrik).
	Title string `json:"title"`
	// MeasureCategory is Naturvårdsverket's åtgärdskategori.
	MeasureCategory string `json:"measure_category"`
	// County and Municipality are empty for charging-station rows, which
	// the source publishes without location.
	County       string `json:"county"`
	Municipality string `json:"municipality"`
	// GrantAmountSEK is "senast tillgängligt beviljat stödbelopp": the
	// latest known granted amount, nil when the source has none.
	GrantAmountSEK *int64 `json:"grant_amount_sek"`
	// ChargingPoints and ChargingAccess are set for charging rows only.
	ChargingPoints *int   `json:"charging_points"`
	ChargingAccess string `json:"charging_access"`
	// DecisionDate is bifallsdatum (YYYY-MM-DD).
	DecisionDate string `json:"decision_date"`
	// EndDate is the latest known end date or final decision date, "" when absent.
	EndDate string `json:"end_date"`
	// Status is "Pågående åtgärd" or "Slutförd åtgärd".
	Status string `json:"status"`
	// Regulation names the ordinance the grant was given under.
	Regulation string `json:"regulation"`

	// Raw holds the untouched cell text per source column header. It is
	// stored as the observation's raw form, not in the payload.
	Raw map[string]string `json:"-"`
}

// RowError is a row that could not be parsed. Row is the 1-based row
// number in the sheet.
type RowError struct {
	Row int
	Err error
	Raw map[string]string
}

// Workbook is one parsed dataset file.
type Workbook struct {
	Sheet        string
	Applications []Application
	RowErrors    []RowError
}

// Dataset describes one published version of the dataset file. It is the
// payload of the dataset observation and is the provenance of every row
// observed from that file.
type Dataset struct {
	URL              string `json:"url"`
	Label            string `json:"label"`
	PublishedThrough string `json:"published_through"`
	ETag             string `json:"etag"`
	LastModified     string `json:"last_modified"`
	SHA256           string `json:"sha256"`
	SizeBytes        int64  `json:"size_bytes"`
	Sheet            string `json:"sheet"`
	RowCount         int    `json:"row_count"`
}
