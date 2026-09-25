package stockholm

import (
	"errors"
	"net/url"
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

var base = &url.URL{Scheme: "https", Host: "etjanster.stockholm.se"}

func TestParseSearchPage_RealPage(t *testing.T) {
	// A real search result for cases started 2026-09-23 (class 7.2).
	cases, err := ParseSearchPage(openFixture(t, "search_page.html"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cases) != 8 {
		t.Fatalf("cases = %d", len(cases))
	}
	got := cases[0]
	want := CaseSummary{RecNo: "1327421", DiaryNumber: "2026-15984", Title: "Anmaning att låta åtgärda hissar samt utföra förnyad besiktning", StartedOn: "2026-09-23", PropertyDesignation: "Kronkvarnen 39", Address: "Artillerigatan 48", CaseGroup: "Funktionskontroll, Tillstånd"}
	if got != want {
		t.Errorf("case[0] =\n%+v\nwant\n%+v", got, want)
	}
	if cases[1].RecNo != "1327424" || cases[1].DiaryNumber != "2026-15987" || cases[1].PropertyDesignation != "Minan 4" || cases[1].Address != "Karlavägen 76" {
		t.Errorf("case[1] = %+v", cases[1])
	}
	for _, c := range cases {
		if c.Archived {
			t.Errorf("%s unexpectedly archived", c.RecNo)
		}
	}
}

func TestParseSearchPage_Failures(t *testing.T) {
	if _, err := ParseSearchPage(strings.NewReader("<html><body>maintenance</body></html>")); !errors.Is(err, ErrUnexpectedMarkup) {
		t.Errorf("no model: err = %v", err)
	}
	if _, err := ParseSearchPage(strings.NewReader(`<script>var CaseSearchResultsViewModel = {"OtherCases":{"CaseSearchDetails":[{"RecNo":"1",</script>`)); err == nil || errors.Is(err, ErrUnexpectedMarkup) {
		t.Errorf("malformed json: err = %v", err)
	}
	if _, err := ParseSearchPage(strings.NewReader(`var CaseSearchResultsViewModel = {"OtherCases":{"CaseSearchDetails":[{"RecNo":"abc","StartDate":"2026-09-23T00:00:00","Name":"2026-1"}]}};`)); err == nil {
		t.Error("non-numeric RecNo must fail")
	}
	if _, err := ParseSearchPage(strings.NewReader(`var CaseSearchResultsViewModel = {"OtherCases":{"CaseSearchDetails":[{"RecNo":"1","StartDate":"yesterday","Name":"2026-1"}]}};`)); err == nil {
		t.Error("malformed start date must fail")
	}
	// An empty result set is a valid, empty page.
	cases, err := ParseSearchPage(strings.NewReader(`var CaseSearchResultsViewModel = {"BuildCases":{"CaseSearchDetails":[]},"RealEstateCases":{"CaseSearchDetails":[]},"OtherCases":{"CaseSearchDetails":[]}};`))
	if err != nil || len(cases) != 0 {
		t.Errorf("empty = %v, %v", cases, err)
	}
}

func TestParseCasePage_RealPages(t *testing.T) {
	c, err := ParseCasePage(openFixture(t, "case_1327421_failed.html"), base)
	if err != nil {
		t.Fatal(err)
	}
	raw := c.RawHTML
	c.RawHTML = ""
	if c.RecNo != "1327421" || c.DiaryNumber != "2026-15984" || c.CaseGroup != "Funktionskontroll, Tillstånd" || c.ClassCode != "7.2 Funktionskontroll, Tillstånd" ||
		c.PropertyDesignation != "Kronkvarnen 39" || c.Address != "Artillerigatan 48" || c.District != "Östermalm" ||
		c.StartedOn != "2026-09-23" || c.ClosedOn != "" || c.Officer != "Ej utsedd" ||
		c.Title != "Anmaning att låta åtgärda hissar samt utföra förnyad besiktning" || c.DocumentCount != 1 || len(c.Documents) != 1 ||
		c.CaseURL != "https://etjanster.stockholm.se/Byggochplantjansten/arende/arende/1327421?dataSource=Active" {
		t.Errorf("case = %+v", c)
	}
	d := c.Documents[0]
	if d.Description != "Intyg återkommande besiktning, ej godkänt, 53119" || d.Category != "Skrivelse In" || d.Timestamp != "2026-09-23T13:29:08" || d.FileName != nil {
		t.Errorf("document = %+v", d)
	}
	if !strings.Contains(raw, "documentList") {
		t.Error("raw html not kept")
	}

	// A closed case with the full lifecycle: failure, reminder, approval.
	c, err = ParseCasePage(openFixture(t, "case_1304022_failed_reminder_approved.html"), base)
	if err != nil {
		t.Fatal(err)
	}
	if c.RecNo != "1304022" || c.DiaryNumber != "2025-13184" || c.PropertyDesignation != "Piloten 2" || c.Address != "Gondolgatan 16" || c.District != "Skarpnäcks Gård" ||
		c.StartedOn != "2025-09-01" || c.ClosedOn != "2026-05-19" || c.DocumentCount != 4 || len(c.Documents) != 4 {
		t.Errorf("closed case = %+v", c)
	}
	descriptions := make([]string, 0, 4)
	for _, d := range c.Documents {
		descriptions = append(descriptions, d.Category+" | "+d.Description)
	}
	want := "Skrivelse In | Intyg återkommande besiktning, godkänt, L1142264, Skrivelse Ut | Påminnelse om användningsförbud, Skrivelse In | Följebrev, Skrivelse In | Intyg återkommande besiktning, ej godkänt, L1142264"
	if strings.Join(descriptions, ", ") != want {
		t.Errorf("documents = %v", descriptions)
	}

	for name, wantCount := range map[string]int{"case_1327289_approved_only.html": 1, "case_1326126_partly_approved.html": 1, "case_1327424_failed_multi_id.html": 1, "case_1304487_failed_twice.html": 4} {
		c, err := ParseCasePage(openFixture(t, name), base)
		if err != nil || len(c.Documents) != wantCount || c.PropertyDesignation == "" || c.District == "" {
			t.Errorf("%s: %+v, %v", name, c, err)
		}
	}
}

func TestParseCasePage_Failures(t *testing.T) {
	page := func(t *testing.T, name string) string {
		t.Helper()
		b, err := os.ReadFile("testdata/" + name)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	good := page(t, "case_1327421_failed.html")

	if _, err := ParseCasePage(strings.NewReader("<html><body>maintenance</body></html>"), base); !errors.Is(err, ErrUnexpectedMarkup) {
		t.Errorf("no metadata: err = %v", err)
	}
	// Document list JSON broken.
	broken := strings.Replace(good, `"category":"Skrivelse In"`, `"category":`, 1)
	if _, err := ParseCasePage(strings.NewReader(broken), base); err == nil || errors.Is(err, ErrUnexpectedMarkup) {
		t.Errorf("malformed document json: err = %v", err)
	}
	// The page says two documents but the list has one: incomplete.
	if _, err := ParseCasePage(strings.NewReader(strings.Replace(good, `"headingSubtext":"(1 st)"`, `"headingSubtext":"(2 st)"`, 1)), base); !errors.Is(err, ErrIncompleteCase) {
		t.Errorf("count mismatch: err = %v", err)
	}
	// Malformed document timestamp.
	if _, err := ParseCasePage(strings.NewReader(strings.Replace(good, `"date":"2026-09-23T13:29:08"`, `"date":"2026-09-23"`, 1)), base); err == nil {
		t.Error("malformed timestamp must fail")
	}
	// Missing address placeholder becomes an empty address.
	c, err := ParseCasePage(strings.NewReader(strings.Replace(good, `<dd class="mb-4">Artillerigatan 48</dd>`, `<dd class="mb-4">Information saknas f&#246;r "Adress"</dd>`, 1)), base)
	if err != nil || c.Address != "" || c.PropertyDesignation != "Kronkvarnen 39" {
		t.Errorf("missing address: %+v, %v", c, err)
	}
}

func TestIsFailedCertificate(t *testing.T) {
	in := func(desc string) Document {
		return Document{Description: desc, Category: "Skrivelse In", Timestamp: "2026-09-23T13:29:08"}
	}
	for desc, want := range map[string]bool{
		"Intyg återkommande besiktning, ej godkänt, L2992070":               true,
		"Intyg återkommande besiktning, ej godkänt, 55117, 53489, 55116":    true,
		"Intyg återkommande besiktning, ej godkänt, 2 st, S529092, S529093": true,
		"Intyg återkommande besiktning, ej godkänt,":                        true,
		"intyg återkommande besiktning, ej godkänd, D1626077":               true,
		"Intyg första besiktning, ej godkänt, L1":                           true,
		"Intyg revisionsbesiktning, ej godkänt, L1":                         true,
		"Intyg ombesiktning, ej godkänt, L1":                                true,
		"  Intyg   återkommande besiktning, ej godkänt, L1  ":               true,
		"Intyg återkommande besiktning, godkänt, S927732":                   false,
		"Intyg ombesiktning, godkänt, L3671153":                             false,
		"Intyg återkommande besiktning, delvis godkänt, L5935770":           false,
		"Godkänt hissintyg 12345":                                           false,
		"Påminnelse om användningsförbud":                                   false,
		"Påminnelse om återkommande besiktning":                             false,
		"Klagomål på hiss":                                                  false,
		"Besiktningsanteckningar (samt 2 ej godkända besiktningsintyg)":     false,
		"Anmaning att låta åtgärda hiss samt utföra förnyad besiktning":     false,
		"Följebrev": false,
	} {
		if got := IsFailedCertificate(in(desc)); got != want {
			t.Errorf("%q: failed = %v, want %v", desc, got, want)
		}
	}
	// Only incoming documents count.
	if IsFailedCertificate(Document{Description: "Intyg återkommande besiktning, ej godkänt, L1", Category: "Skrivelse Ut"}) {
		t.Error("outgoing document must not be a failure")
	}
}

func TestEventID(t *testing.T) {
	d := Document{Description: "Intyg återkommande besiktning, ej godkänt, L2992070", Category: "Skrivelse In", Timestamp: "2026-09-23T13:35:13"}
	id := EventID("1327427", d)
	if !strings.HasPrefix(id, "1327427:") || len(id) != len("1327427:")+32 {
		t.Errorf("id = %q", id)
	}
	// Stable across whitespace and re-fetches; distinct across any stable
	// metadata difference.
	same := Document{Description: "  Intyg återkommande  besiktning, ej godkänt, L2992070 ", Category: "Skrivelse In ", Timestamp: " 2026-09-23T13:35:13"}
	if EventID("1327427", same) != id {
		t.Error("whitespace must not change the id")
	}
	for name, other := range map[string]Document{
		"other timestamp":   {Description: d.Description, Category: d.Category, Timestamp: "2026-09-23T13:35:14"},
		"other description": {Description: "Intyg återkommande besiktning, ej godkänt, L2992071", Category: d.Category, Timestamp: d.Timestamp},
		"other category":    {Description: d.Description, Category: "Skrivelse Ut", Timestamp: d.Timestamp},
	} {
		if EventID("1327427", other) == id {
			t.Errorf("%s: id must differ", name)
		}
	}
	if EventID("1327428", d) == id {
		t.Error("other case must differ")
	}
}

func TestToEvent(t *testing.T) {
	c := Case{RecNo: "1327427", DiaryNumber: "2026-15990", PropertyDesignation: "Skären 4", Address: "Biblioteksgatan 3", District: "Norrmalm", Title: "Anmaning att låta åtgärda hiss samt utföra förnyad besiktning", CaseURL: "https://etjanster.stockholm.se/Byggochplantjansten/arende/arende/1327427?dataSource=Active"}
	d := Document{Description: "Intyg återkommande besiktning, ej godkänt, L2992070", Category: "Skrivelse In", Timestamp: "2026-09-23T13:35:13"}
	ev, err := ToEvent(c, d)
	if err != nil {
		t.Fatal(err)
	}
	if ev.Source != "stockholm" || ev.SourceEventID != EventID(c.RecNo, d) || ev.EventType != "MOTORISED_BUILDING_EQUIPMENT_INSPECTION_FAILED" ||
		ev.OccurredOn.Format(dateLayout) != "2026-09-23" || ev.Title != d.Description ||
		ev.OrganisationNumber != "" || ev.OrganisationName != "" || ev.WorkplaceCFAR != "" || ev.WorkplaceName != "" ||
		ev.PropertyMunicipalityCode != "0180" || ev.PropertyDesignation != "Skären 4" || ev.PropertyAddress != "Biblioteksgatan 3" || ev.SourceURL != c.CaseURL {
		t.Errorf("event = %+v", ev)
	}
	c.Address = ""
	ev, err = ToEvent(c, d)
	if err != nil || ev.PropertyAddress != "" || ev.PropertyDesignation != "Skären 4" {
		t.Errorf("event without address = %+v, %v", ev, err)
	}
	c.PropertyDesignation = ""
	if _, err := ToEvent(c, d); err == nil {
		t.Error("a case without property must not become an event")
	}
}
