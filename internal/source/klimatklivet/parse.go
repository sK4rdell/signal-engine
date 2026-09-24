package klimatklivet

import (
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

// column maps a field to the normalised header prefixes that identify it.
// Headers are matched by prefix because the long ones carry qualifiers
// ("… stödbelopp (kr)") that may change.
type column struct {
	field    string
	prefixes []string
	required bool
}

var columns = []column{
	{"case_number", []string{"ärendenummer"}, true},
	{"organisation_name", []string{"organisationsnamn"}, true},
	{"title", []string{"rubrik"}, true},
	{"measure_category", []string{"åtgärdskategori"}, true},
	{"county", []string{"län"}, false},
	{"grant_amount", []string{"senast tillgängligt beviljat stödbelopp"}, true},
	{"charging_points", []string{"summering antal laddpunkter"}, false},
	{"charging_access", []string{"laddinfra"}, false},
	{"municipality", []string{"kommun"}, false},
	{"decision_date", []string{"bifallsdatum"}, true},
	{"end_date", []string{"senast tillgängligt slutdatum"}, false},
	{"status", []string{"status"}, true},
	{"regulation", []string{"förordning"}, true},
}

var caseNumberPattern = regexp.MustCompile(`^[A-Z]{2,4}-\d+-\d+(-[a-z])?$`)

const dateLayout = "2006-01-02"

// ErrNoSheets is returned for a workbook without worksheets.
var ErrNoSheets = errors.New("klimatklivet: workbook has no sheets")

// ParseWorkbook parses the dataset file. Rows that cannot be parsed are
// reported in RowErrors without failing the file; a missing required
// column fails the file, because it means the source changed shape.
func ParseWorkbook(r io.Reader) (Workbook, error) {
	f, err := excelize.OpenReader(r)
	if err != nil {
		return Workbook{}, fmt.Errorf("klimatklivet: open workbook: %w", err)
	}
	defer func() { _ = f.Close() }()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return Workbook{}, ErrNoSheets
	}
	sheet := sheets[0]
	if idx, _ := f.GetSheetIndex(PreferredSheet); idx >= 0 {
		sheet = PreferredSheet
	}

	rows, err := f.Rows(sheet)
	if err != nil {
		return Workbook{}, fmt.Errorf("klimatklivet: read sheet %q: %w", sheet, err)
	}
	defer func() { _ = rows.Close() }()

	wb := Workbook{Sheet: sheet}
	var headers []string
	var fieldIndex map[string]int
	rowNumber := 0
	for rows.Next() {
		rowNumber++
		cells, err := rows.Columns(excelize.Options{RawCellValue: true})
		if err != nil {
			return Workbook{}, fmt.Errorf("klimatklivet: read row %d: %w", rowNumber, err)
		}
		if isEmpty(cells) {
			continue
		}
		if headers == nil {
			headers = cells
			fieldIndex, err = mapHeaders(headers)
			if err != nil {
				return Workbook{}, err
			}
			continue
		}
		raw := rawRow(headers, cells)
		app, err := parseRow(raw, headers, cells, fieldIndex)
		if err != nil {
			wb.RowErrors = append(wb.RowErrors, RowError{Row: rowNumber, Err: err, Raw: raw})
			continue
		}
		wb.Applications = append(wb.Applications, app)
	}
	if headers == nil {
		return Workbook{}, fmt.Errorf("klimatklivet: sheet %q has no header row", sheet)
	}
	return wb, nil
}

func normaliseHeader(h string) string {
	return strings.ToLower(strings.Join(strings.Fields(h), " "))
}

// mapHeaders resolves the column index of every known field by header
// prefix. Unknown columns are ignored; a missing required column is an
// error naming the field.
func mapHeaders(headers []string) (map[string]int, error) {
	index := map[string]int{}
	for i, h := range headers {
		n := normaliseHeader(h)
		if n == "" {
			continue
		}
		for _, c := range columns {
			if _, taken := index[c.field]; taken {
				continue
			}
			for _, p := range c.prefixes {
				if strings.HasPrefix(n, p) {
					index[c.field] = i
					break
				}
			}
		}
	}
	var missing []string
	for _, c := range columns {
		if _, ok := index[c.field]; !ok && c.required {
			missing = append(missing, c.prefixes[0])
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("klimatklivet: required columns missing: %s", strings.Join(missing, ", "))
	}
	return index, nil
}

func isEmpty(cells []string) bool {
	for _, c := range cells {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}

func rawRow(headers, cells []string) map[string]string {
	raw := make(map[string]string, len(headers))
	for i, h := range headers {
		if strings.TrimSpace(h) == "" {
			continue
		}
		if i < len(cells) {
			raw[h] = cells[i]
		} else {
			raw[h] = ""
		}
	}
	return raw
}

func parseRow(raw map[string]string, headers, cells []string, index map[string]int) (Application, error) {
	cell := func(field string) string {
		i, ok := index[field]
		if !ok || i >= len(cells) {
			return ""
		}
		return strings.TrimSpace(cells[i])
	}

	app := Application{Raw: raw}
	app.CaseNumber = cell("case_number")
	if !caseNumberPattern.MatchString(app.CaseNumber) {
		return Application{}, fmt.Errorf("case number %q is missing or malformed", app.CaseNumber)
	}
	app.OrganisationName = cell("organisation_name")
	app.Title = cell("title")
	app.MeasureCategory = cell("measure_category")
	app.County = cell("county")
	app.Municipality = cell("municipality")
	app.ChargingAccess = cell("charging_access")
	app.Status = cell("status")
	app.Regulation = cell("regulation")

	decision, err := parseDate(cell("decision_date"))
	if err != nil || decision == "" {
		return Application{}, fmt.Errorf("decision date %q is missing or malformed", cell("decision_date"))
	}
	app.DecisionDate = decision

	if v := cell("end_date"); v != "" {
		end, err := parseDate(v)
		if err != nil {
			return Application{}, fmt.Errorf("end date %q is malformed", v)
		}
		app.EndDate = end
	}
	if v := cell("grant_amount"); v != "" {
		amount, err := parseAmount(v)
		if err != nil {
			return Application{}, fmt.Errorf("grant amount %q is malformed", v)
		}
		app.GrantAmountSEK = &amount
	}
	if v := cell("charging_points"); v != "" {
		n, err := parseAmount(v)
		if err != nil {
			return Application{}, fmt.Errorf("charging points %q is malformed", v)
		}
		points := int(n)
		app.ChargingPoints = &points
	}
	return app, nil
}

// parseDate accepts an Excel serial number (how the published file stores
// dates) or an ISO date with optional time.
func parseDate(v string) (string, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return "", nil
	}
	if serial, err := strconv.ParseFloat(v, 64); err == nil {
		t, err := excelize.ExcelDateToTime(serial, false)
		if err != nil {
			return "", err
		}
		return t.Format(dateLayout), nil
	}
	for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02T15:04:05Z07:00", "2006-01-02T15:04:05", dateLayout} {
		if t, err := time.Parse(layout, v); err == nil {
			return t.Format(dateLayout), nil
		}
	}
	return "", fmt.Errorf("not a date: %q", v)
}

// parseAmount reads an integer amount that Excel may store as a float.
func parseAmount(v string) (int64, error) {
	v = strings.ReplaceAll(strings.TrimSpace(v), " ", "")
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0, err
	}
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, fmt.Errorf("not a number: %q", v)
	}
	return int64(math.Round(f)), nil
}
