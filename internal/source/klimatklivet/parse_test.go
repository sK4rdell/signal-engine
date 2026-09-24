package klimatklivet

import (
	"os"
	"strings"
	"testing"
)

func openFixture(t *testing.T, name string) *os.File {
	t.Helper()
	f, err := os.Open("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

func i64(v int64) *int64 { return &v }

func TestParseWorkbook_RealRows(t *testing.T) {
	wb, err := ParseWorkbook(openFixture(t, "beviljade.xlsx"))
	if err != nil {
		t.Fatal(err)
	}
	if wb.Sheet != PreferredSheet {
		t.Errorf("sheet = %q", wb.Sheet)
	}
	if len(wb.RowErrors) != 0 {
		t.Fatalf("row errors: %+v", wb.RowErrors)
	}
	if len(wb.Applications) != 6 {
		t.Fatalf("applications = %d", len(wb.Applications))
	}

	// A completed local climate investment with every column filled.
	got := wb.Applications[0]
	raw := got.Raw
	got.Raw = nil
	want := Application{
		CaseNumber:       "KKL-02186-2017",
		OrganisationName: "Stockholm Exergi Materialåtervinning AB",
		Title:            "Sorteringsanläggning restavfall",
		MeasureCategory:  "Avfall",
		County:           "Stockholms län",
		Municipality:     "Sigtuna",
		GrantAmountSEK:   i64(134000000),
		DecisionDate:     "2017-12-01",
		EndDate:          "2021-07-31",
		Status:           "Slutförd åtgärd",
		Regulation:       "Förordning (2015:517) stöd till lokala klimatinvesteringar",
	}
	if got.CaseNumber != want.CaseNumber || got.OrganisationName != want.OrganisationName || got.Title != want.Title ||
		got.MeasureCategory != want.MeasureCategory || got.County != want.County || got.Municipality != want.Municipality ||
		got.GrantAmountSEK == nil || *got.GrantAmountSEK != *want.GrantAmountSEK || got.ChargingPoints != nil || got.ChargingAccess != "" ||
		got.DecisionDate != want.DecisionDate || got.EndDate != want.EndDate || got.Status != want.Status || got.Regulation != want.Regulation {
		t.Errorf("application[0] =\n%+v\nwant\n%+v", got, want)
	}
	if raw["Ärendenummer"] != "KKL-02186-2017" || raw["Bifallsdatum"] == "" || len(raw) != 13 {
		t.Errorf("raw = %v", raw)
	}

	// A charging-station row under the charging ordinance: no location,
	// no end date, charging fields set.
	got = wb.Applications[1]
	if got.CaseNumber != "NV-19-006817" || got.MeasureCategory != "Laddstation" || got.County != "" || got.Municipality != "" ||
		got.ChargingPoints == nil || *got.ChargingPoints != 17 || got.ChargingAccess != "Icke-publik" || got.EndDate != "" ||
		got.GrantAmountSEK == nil || *got.GrantAmountSEK != 164312 || got.DecisionDate != "2019-11-22" ||
		!strings.HasPrefix(got.Regulation, "Förordning (2019:525)") {
		t.Errorf("application[1] = %+v", got)
	}

	// The source pads some early case numbers with a trailing space.
	got = wb.Applications[2]
	if got.CaseNumber != "NV-05928-15" || got.Raw["Ärendenummer"] != "NV-05928-15 " {
		t.Errorf("application[2] = %q raw %q", got.CaseNumber, got.Raw["Ärendenummer"])
	}

	// A recent ongoing industrial conversion.
	got = wb.Applications[3]
	if got.CaseNumber != "NV-25-043146" || got.OrganisationName != "Orkla Snacks Sverige AB" || got.DecisionDate != "2025-12-18" ||
		got.EndDate != "2028-04-01" || got.Status != "Pågående åtgärd" || got.GrantAmountSEK == nil || *got.GrantAmountSEK != 45900000 {
		t.Errorf("application[3] = %+v", got)
	}
}

func TestParseWorkbook_HeaderLookupSurvivesReorderedColumns(t *testing.T) {
	reference, err := ParseWorkbook(openFixture(t, "beviljade.xlsx"))
	if err != nil {
		t.Fatal(err)
	}
	wb, err := ParseWorkbook(openFixture(t, "beviljade_reordered.xlsx"))
	if err != nil {
		t.Fatal(err)
	}
	if wb.Sheet != "Data" {
		t.Errorf("sheet = %q, want the only sheet", wb.Sheet)
	}
	if len(wb.RowErrors) != 0 || len(wb.Applications) != len(reference.Applications) {
		t.Fatalf("applications = %d, errors = %+v", len(wb.Applications), wb.RowErrors)
	}
	for n := range wb.Applications {
		a, b := wb.Applications[n], reference.Applications[n]
		a.Raw, b.Raw = nil, nil
		if a.CaseNumber != b.CaseNumber || a.DecisionDate != b.DecisionDate || a.EndDate != b.EndDate || a.Title != b.Title ||
			a.County != b.County || a.Municipality != b.Municipality || a.Status != b.Status || a.Regulation != b.Regulation ||
			(a.GrantAmountSEK == nil) != (b.GrantAmountSEK == nil) || (a.GrantAmountSEK != nil && *a.GrantAmountSEK != *b.GrantAmountSEK) ||
			(a.ChargingPoints == nil) != (b.ChargingPoints == nil) || (a.ChargingPoints != nil && *a.ChargingPoints != *b.ChargingPoints) {
			t.Errorf("row %d differs:\n%+v\n%+v", n, a, b)
		}
	}
	if wb.Applications[0].Raw["Kommentar"] != "extra" {
		t.Errorf("unknown columns should be kept in raw: %v", wb.Applications[0].Raw)
	}
}

func TestParseWorkbook_MalformedRowsAreReportedNotFatal(t *testing.T) {
	wb, err := ParseWorkbook(openFixture(t, "beviljade_malformed.xlsx"))
	if err != nil {
		t.Fatal(err)
	}
	if len(wb.Applications) != 2 {
		t.Fatalf("applications = %+v", wb.Applications)
	}
	if wb.Applications[0].CaseNumber != "NV-25-043146" {
		t.Errorf("application[0] = %+v", wb.Applications[0])
	}
	// A missing grant amount is allowed (one real row has none).
	if wb.Applications[1].CaseNumber != "NV-05928-15" || wb.Applications[1].GrantAmountSEK != nil {
		t.Errorf("application[1] = %+v", wb.Applications[1])
	}
	if len(wb.RowErrors) != 3 {
		t.Fatalf("row errors = %+v", wb.RowErrors)
	}
	for n, want := range []struct {
		row int
		msg string
	}{{3, "case number"}, {4, "decision date"}, {5, "grant amount"}} {
		re := wb.RowErrors[n]
		if re.Row != want.row || !strings.Contains(re.Err.Error(), want.msg) || len(re.Raw) == 0 {
			t.Errorf("row error %d = %+v, want row %d mentioning %q", n, re, want.row, want.msg)
		}
	}
}

func TestParseWorkbook_MissingRequiredColumnFails(t *testing.T) {
	_, err := ParseWorkbook(openFixture(t, "beviljade_missing_header.xlsx"))
	if err == nil || !strings.Contains(err.Error(), "bifallsdatum") {
		t.Fatalf("err = %v, want missing bifallsdatum", err)
	}
}

func TestParseWorkbook_NotAWorkbook(t *testing.T) {
	if _, err := ParseWorkbook(strings.NewReader("<html>maintenance</html>")); err == nil {
		t.Fatal("expected error for non-xlsx input")
	}
}

func TestParseDateAndAmount(t *testing.T) {
	for in, want := range map[string]string{
		"43070":               "2017-12-01", // Excel serial
		"46009":               "2025-12-18",
		"2026-05-20 00:00:00": "2026-05-20",
		"2026-05-20":          "2026-05-20",
		"":                    "",
	} {
		got, err := parseDate(in)
		if err != nil || got != want {
			t.Errorf("parseDate(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	if _, err := parseDate("ej beslutad"); err == nil {
		t.Error("parseDate accepted text")
	}
	for in, want := range map[string]int64{"134000000": 134000000, "1929278.0": 1929278, "45 900 000": 45900000, "17": 17} {
		got, err := parseAmount(in)
		if err != nil || got != want {
			t.Errorf("parseAmount(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
	if _, err := parseAmount("ej fastställt"); err == nil {
		t.Error("parseAmount accepted text")
	}
}

func TestPublishedThrough(t *testing.T) {
	for in, want := range map[string]string{
		"Beviljade ansökningar till Klimatklivet till och med 30 juni 2026 (xlsx)":     "2026-06-30",
		"Beviljade ansökningar till Klimatklivet till och med 31 december 2025 (xlsx)": "2025-12-31",
		"Beviljade ansökningar till Klimatklivet (xlsx)":                               "",
		"till och med 31 juni 2026":                                                    "", // no such date
	} {
		if got := publishedThrough(in); got != want {
			t.Errorf("publishedThrough(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestToEvent(t *testing.T) {
	app := Application{CaseNumber: "NV-25-043146", OrganisationName: "Orkla Snacks Sverige AB", Title: "Energikonvertering industri", DecisionDate: "2025-12-18", GrantAmountSEK: i64(45900000)}
	ev := ToEvent(app, "https://www.naturvardsverket.se/results")
	if ev.Source != Source || ev.SourceEventID != "NV-25-043146" || ev.EventType != "CLIMATE_INVESTMENT_GRANT_APPROVED" ||
		ev.OccurredOn.Format(dateLayout) != "2025-12-18" || ev.Title != app.Title || ev.OrganisationName != app.OrganisationName ||
		ev.OrganisationNumber != "" || ev.WorkplaceCFAR != "" || ev.WorkplaceName != "" || ev.SourceURL != "https://www.naturvardsverket.se/results" {
		t.Errorf("event = %+v", ev)
	}
}
