package arbetsmiljoverket

import (
	"errors"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/sK4rdell/signal-engine/internal/publicevent"
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

var base = &url.URL{Scheme: "https", Host: "www.av.se"}

func TestParseSearchPage_RealRows(t *testing.T) {
	page, err := ParseSearchPage(openFixture(t, "search_page.html"), base)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.RowErrors) != 0 {
		t.Fatalf("row errors: %+v", page.RowErrors)
	}
	if len(page.Documents) != 3 || page.Total != 347 {
		t.Fatalf("documents = %d, total = %d", len(page.Documents), page.Total)
	}

	// A complete row: organisation and workplace both present.
	got := page.Documents[0]
	raw := got.RawHTML
	got.RawHTML = ""
	want := Document{
		DocumentNumber:     "2026/060943-2",
		DocumentType:       "Inspektionsmeddelande",
		DocumentDate:       "2026-09-23",
		Origin:             "Utgående",
		SubjectArea:        "Bedriva inspektion",
		CaseNumber:         "2026/060943",
		CaseTitle:          "Inspektion inom Fortlöpande tillsyn - Unga i arbetslivet",
		CaseStatus:         "Pågående",
		OrganisationNumber: "5594800418",
		OrganisationName:   "MITTEN MACK AB",
		WorkplaceName:      "MITTEN MACK AB",
		WorkplaceCFAR:      "71466304",
		CaseURL:            "https://www.av.se/om-oss/diarium-och-allmanna-handlingar/bestall-handlingar/Case/?id=2026/060943",
	}
	if got != want {
		t.Errorf("document[0] =\n%+v\nwant\n%+v", got, want)
	}
	if !strings.Contains(raw, `class="document-list__item"`) || !strings.Contains(raw, "2026/060943-2") {
		t.Errorf("raw html not kept: %.120s", raw)
	}

	// A row without organisation: the source prints an empty block and
	// "Saknas" as workplace name, but still a CFAR.
	got = page.Documents[1]
	if got.DocumentNumber != "2026/058034-4" || got.OrganisationNumber != "" || got.OrganisationName != "" ||
		got.WorkplaceName != "Saknas" || got.WorkplaceCFAR != "73278392" || got.CaseNumber != "2026/058034" ||
		got.DocumentDate == "" || got.CaseTitle == "" {
		t.Errorf("document[1] = %+v", got)
	}

	// Workplace and legal organisation differ.
	got = page.Documents[2]
	if got.DocumentNumber != "2026/061028-2" || got.OrganisationNumber != "5562802115" || got.OrganisationName != "RUSTA AB (PUBL)" ||
		got.WorkplaceName != "RUSTA HÖÖR" || got.WorkplaceCFAR != "72896509" ||
		got.CaseTitle != "Inspektion inom Fortlöpande tillsyn Fall från samma nivå" {
		t.Errorf("document[2] = %+v", got)
	}
}

func TestParseSearchPage_EmptyPage(t *testing.T) {
	page, err := ParseSearchPage(openFixture(t, "search_page_empty.html"), base)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Documents) != 0 || len(page.RowErrors) != 0 || page.Total != 347 {
		t.Errorf("page = %+v", page)
	}
}

func TestParseSearchPage_MalformedRowsAreReportedNotFatal(t *testing.T) {
	page, err := ParseSearchPage(openFixture(t, "search_page_malformed.html"), base)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Documents) != 2 {
		t.Fatalf("documents = %+v", page.Documents)
	}
	if page.Documents[0].DocumentNumber != "2026/060943-2" || page.Documents[1].DocumentNumber != "2026/060943-1" || page.Documents[1].DocumentType != "Registrerad kontroll" {
		t.Errorf("documents = %+v", page.Documents)
	}
	if len(page.RowErrors) != 2 {
		t.Fatalf("row errors = %+v", page.RowErrors)
	}
	if page.RowErrors[0].Index != 1 || !strings.Contains(page.RowErrors[0].Err.Error(), "document number") {
		t.Errorf("row error 0 = %+v", page.RowErrors[0])
	}
	if page.RowErrors[1].Index != 2 || !strings.Contains(page.RowErrors[1].Err.Error(), "document date") {
		t.Errorf("row error 1 = %+v", page.RowErrors[1])
	}
	for _, re := range page.RowErrors {
		if !strings.Contains(re.RawHTML, "document-list__item") {
			t.Errorf("row error without raw html: %+v", re)
		}
	}
}

func TestParseSearchPage_UnexpectedMarkup(t *testing.T) {
	_, err := ParseSearchPage(openFixture(t, "unexpected_markup.html"), base)
	if !errors.Is(err, ErrUnexpectedMarkup) {
		t.Fatalf("err = %v, want ErrUnexpectedMarkup", err)
	}
}

func TestParseSearchPage_TotalWithThousandsSeparators(t *testing.T) {
	const html = `<html><body>
		<ul data-dd-search-result></ul>
		<span id="dd-pagination-result-total" class="fw-bold">2&nbsp;064 895</span>
	</body></html>`
	page, err := ParseSearchPage(strings.NewReader(html), base)
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 2064895 {
		t.Errorf("total = %d", page.Total)
	}

	page, err = ParseSearchPage(strings.NewReader(`<html><body><ul data-dd-search-result></ul></body></html>`), base)
	if err != nil || page.Total != 0 {
		t.Errorf("without summary: total = %d, err = %v", page.Total, err)
	}
}

func TestToEvent(t *testing.T) {
	doc := Document{
		DocumentNumber:     "2026/060943-2",
		DocumentType:       "Inspektionsmeddelande",
		DocumentDate:       "2026-09-23",
		CaseNumber:         "2026/060943",
		CaseTitle:          "Inspektion inom Fortlöpande tillsyn - Unga i arbetslivet",
		OrganisationNumber: "5594800418",
		OrganisationName:   "MITTEN MACK AB",
		WorkplaceName:      "MITTEN MACK AB",
		WorkplaceCFAR:      "71466304",
		CaseURL:            "https://www.av.se/x/Case/?id=2026/060943",
	}
	ev, notes := ToEvent(doc, publicevent.EventTypeWorkEnvironmentInspectionNotice)
	if notes != (MappingNotes{}) {
		t.Errorf("notes = %+v", notes)
	}
	if ev.Source != publicevent.SourceArbetsmiljoverket || ev.SourceEventID != "2026/060943-2" ||
		ev.EventType != publicevent.EventTypeWorkEnvironmentInspectionNotice ||
		ev.OccurredOn.Format(dateLayout) != "2026-09-23" || ev.Title != doc.CaseTitle ||
		ev.OrganisationNumber != "5594800418" || ev.OrganisationName != "MITTEN MACK AB" ||
		ev.WorkplaceCFAR != "71466304" || ev.WorkplaceName != "MITTEN MACK AB" || ev.SourceURL != doc.CaseURL {
		t.Errorf("event = %+v", ev)
	}

	// Missing organisation, placeholder workplace name.
	doc.OrganisationNumber, doc.OrganisationName, doc.WorkplaceName = "", "", "Saknas"
	ev, notes = ToEvent(doc, publicevent.EventTypeWorkEnvironmentInspectionNotice)
	if notes != (MappingNotes{}) || ev.OrganisationNumber != "" || ev.OrganisationName != "" || ev.WorkplaceName != "" || ev.WorkplaceCFAR != "71466304" {
		t.Errorf("event = %+v, notes = %+v", ev, notes)
	}

	// Invalid identifiers are dropped, never guessed, and reported.
	doc.OrganisationNumber, doc.WorkplaceCFAR = "5594800419", "7146630"
	ev, notes = ToEvent(doc, publicevent.EventTypeWorkEnvironmentInspectionNotice)
	if !notes.InvalidOrganisationNumber || !notes.InvalidWorkplaceCFAR || ev.OrganisationNumber != "" || ev.WorkplaceCFAR != "" {
		t.Errorf("event = %+v, notes = %+v", ev, notes)
	}

	// "Saknas" in the CFAR field is absent, not invalid.
	doc.OrganisationNumber, doc.WorkplaceCFAR = "5594800418", "Saknas"
	ev, notes = ToEvent(doc, publicevent.EventTypeWorkEnvironmentInspectionNotice)
	if notes != (MappingNotes{}) || ev.WorkplaceCFAR != "" {
		t.Errorf("event = %+v, notes = %+v", ev, notes)
	}

	// Hyphenated organisation number is normalised.
	doc.OrganisationNumber, doc.WorkplaceCFAR = "559480-0418", "71466304"
	ev, notes = ToEvent(doc, publicevent.EventTypeWorkEnvironmentInspectionNotice)
	if notes != (MappingNotes{}) || ev.OrganisationNumber != "5594800418" {
		t.Errorf("event = %+v, notes = %+v", ev, notes)
	}

	// The event type is the feed's; the title stays the source's wording.
	doc.CaseTitle = "Återkommande besiktning - Fordonslyft flerpelarlyft"
	ev, _ = ToEvent(doc, publicevent.EventTypeWorkEquipmentInspectionFailed)
	if ev.EventType != publicevent.EventTypeWorkEquipmentInspectionFailed || ev.Title != doc.CaseTitle {
		t.Errorf("event = %+v", ev)
	}
}

func TestParseSearchPage_CertificateRowsExposeCase(t *testing.T) {
	page, err := ParseSearchPage(openFixture(t, "search_page_case_044084.html"), base)
	if err != nil || len(page.RowErrors) != 0 || len(page.Documents) != 2 || page.Total != 2 {
		t.Fatalf("page = %+v, %v", page, err)
	}
	for _, d := range page.Documents {
		if d.CaseNumber != "2026/044084" || d.DocumentType != "Intyg återkommande besiktning" || d.Origin != "Inkommande" ||
			d.CaseTitle != "Återkommande besiktning - Lyftbord vid lastkaj" || d.OrganisationNumber != "5593182370" || d.WorkplaceCFAR != "67577551" {
			t.Errorf("document = %+v", d)
		}
	}
	if page.Documents[0].DocumentNumber != "2026/044084-5" || page.Documents[0].DocumentDate != "2026-09-24" ||
		page.Documents[1].DocumentNumber != "2026/044084-1" || page.Documents[1].DocumentDate != "2026-06-29" {
		t.Errorf("documents = %+v", page.Documents)
	}
}

func TestDocumentOrderWithinCase(t *testing.T) {
	for number, want := range map[string]int{"2026/044084-5": 5, "2026/044084-1": 1, "2026/044084-12": 12, "2026/044084": 0, "": 0, "2026/044084-x": 0} {
		if got := documentSuffix(number); got != want {
			t.Errorf("documentSuffix(%q) = %d, want %d", number, got, want)
		}
	}
	a := Document{DocumentNumber: "2026/044084-1", DocumentDate: "2026-06-29"}
	b := Document{DocumentNumber: "2026/044084-5", DocumentDate: "2026-09-24"}
	sameDay := Document{DocumentNumber: "2026/044084-2", DocumentDate: "2026-06-29"}
	if !documentBefore(a, b) || documentBefore(b, a) {
		t.Error("earlier date must come first")
	}
	if !documentBefore(a, sameDay) || documentBefore(sameDay, a) {
		t.Error("same date: lower suffix must come first")
	}
	if documentBefore(a, a) {
		t.Error("a document is not before itself")
	}
}
