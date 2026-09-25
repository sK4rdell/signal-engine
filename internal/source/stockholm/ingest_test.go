package stockholm_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/sK4rdell/signal-engine/internal/platform/config"
	"github.com/sK4rdell/signal-engine/internal/platform/testutil"
	"github.com/sK4rdell/signal-engine/internal/publicevent"
	"github.com/sK4rdell/signal-engine/internal/source/stockholm"
)

// fakeService serves a search page and case pages by RecNo, the way the
// real service does, and records which case pages were requested.
type fakeService struct {
	mu       sync.Mutex
	search   []byte
	cases    map[string][]byte
	status   map[string]int
	requests []string
}

func (f *fakeService) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if r.URL.Path == stockholm.SearchPath {
		f.requests = append(f.requests, "search")
		_, _ = w.Write(f.search)
		return
	}
	recNo := strings.TrimPrefix(r.URL.Path, stockholm.CasePath)
	f.requests = append(f.requests, recNo)
	if st := f.status[recNo]; st != 0 {
		http.Error(w, "boom", st)
		return
	}
	body, ok := f.cases[recNo]
	if !ok {
		http.NotFound(w, r)
		return
	}
	_, _ = w.Write(body)
}

func (f *fakeService) caseRequests() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, r := range f.requests {
		if r != "search" {
			out = append(out, r)
		}
	}
	return out
}

func (f *fakeService) set(search []byte, cases map[string][]byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if search != nil {
		f.search = search
	}
	for k, v := range cases {
		f.cases[k] = v
	}
	f.requests = nil
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// searchPage builds a search result page listing the given cases, using
// the real page as the frame and the source's own JSON field names.
func searchPage(t *testing.T, cases ...caseSpec) []byte {
	t.Helper()
	frame := string(readFixture(t, "search_page.html"))
	details := make([]map[string]any, 0, len(cases))
	for _, c := range cases {
		details = append(details, map[string]any{"RecNo": c.recNo, "CaseTypeCode": "Funktionskontroll, Tillstånd", "ObjectType": nil, "Description": c.title(), "StartDate": c.startedOn + "T00:00:00", "RealEstateName": c.property, "RealEstateAddress": c.address, "Name": c.diary, "IsEarchive": false})
	}
	model, _ := json.Marshal(map[string]any{
		"BuildCases":      map[string]any{"CaseSearchDetails": []any{}},
		"RealEstateCases": map[string]any{"CaseSearchDetails": []any{}},
		"OtherCases":      map[string]any{"CaseSearchDetails": details},
	})
	marker := "var CaseSearchResultsViewModel = "
	i := strings.Index(frame, marker)
	j := strings.Index(frame[i:], "};")
	if i < 0 || j < 0 {
		t.Fatal("search fixture has no embedded model")
	}
	return []byte(frame[:i] + marker + string(model) + frame[i+j+1:])
}

type docSpec struct {
	desc, category, ts string
}

type caseSpec struct {
	recNo, diary, property, address, district, startedOn, closedOn string
	docs                                                           []docSpec
}

func (c caseSpec) title() string {
	return "Anmaning att låta åtgärda hiss samt utföra förnyad besiktning"
}

// casePage renders a case page with the same structure as the real one
// (definition list, document-list component and embedded JSON).
func casePage(t *testing.T, c caseSpec) []byte {
	t.Helper()
	rows := make([]map[string]any, 0, len(c.docs))
	for _, d := range c.docs {
		cat := d.category
		if cat == "" {
			cat = "Skrivelse In"
		}
		rows = append(rows, map[string]any{"title": d.desc, "category": cat, "date": d.ts, "fileName": nil, "data": []any{}})
	}
	list, _ := json.Marshal(rows)
	address := c.address
	if address == "" {
		address = `Information saknas f&#246;r "Adress"`
	}
	return []byte(fmt.Sprintf(`<!DOCTYPE html><html><body>
<div data-react data-type='info-box' data-props='{"content":[{"type":"html","content":"<a href=\u0027/Byggochplantjansten/inloggad2/arende/arende/%[1]s?dataSource=Active\u0027>Logga in</a>"}]}'></div>
<h1>&#196;rende</h1>
<dl>
<div class="flex"><div><dt class="dt-heading">Diarienummer</dt><dd class="mb-4">%[2]s</dd></div><div><dt class="dt-heading">&#196;rendegrupp</dt><dd class="mb-4">Funktionskontroll, Tillst&#229;nd</dd></div></div>
<div class="flex"><div><dt class="dt-heading">Diarieplansbeteckning</dt><dd class="mb-4">7.2 Funktionskontroll, Tillst&#229;nd</dd></div><div><dt class="dt-heading">Fastighetsbeteckning</dt><dd class="mb-4">%[3]s</dd></div></div>
<div class="flex"><div><dt class="dt-heading">Adress</dt><dd class="mb-4">%[4]s</dd></div><div><dt class="dt-heading">Stadsdel</dt><dd class="mb-4">%[5]s</dd></div></div>
<div class="flex"><div><dt class="dt-heading">&#196;rendestart</dt><dd class="mb-4">%[6]s</dd></div><div><dt class="dt-heading">&#196;rendeavslut</dt><dd class="mb-4">%[7]s</dd></div></div>
<div class="flex"><div><dt class="dt-heading">Handl&#228;ggare</dt><dd class="mb-6">Ej utsedd</dd></div></div>
<div><dt class="dt-heading">&#196;rendemening</dt><dd class="mb-6">%[8]s</dd></div>
</dl>
<div data-react data-type='list' data-props='{"id":"documentList","heading":"Alla tillh&#246;rande dokument","headingSubtext":"(%[9]d st)","headingLevel":"2","rows":[]}'></div>
<script>var documentListComponent = window.listComponent.createInstance('documentList', 'pageLoader','documentListPagination', %[10]s, 50, 1, '/Byggochplantjansten/CaseAdvanced/GetPaginationComponent')</script>
</body></html>`, c.recNo, c.diary, c.property, address, c.district, c.startedOn, c.closedOn, c.title(), len(rows), list))
}

func newApp(t *testing.T, svc *fakeService) (*testutil.App, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(svc)
	t.Cleanup(server.Close)
	app := testutil.NewApp(t, testutil.WithConfig(func(cfg *config.Config) {
		cfg.Sources.Stockholm.BaseURL = server.URL
	}))
	return app, server
}

func window(t *testing.T, from, to string) stockholm.Window {
	t.Helper()
	f, _ := time.Parse("2006-01-02", from)
	u, _ := time.Parse("2006-01-02", to)
	return stockholm.Window{From: f, To: u}
}

func countRows(t *testing.T, app *testutil.App, table, where string) int {
	t.Helper()
	var n int
	if err := app.Pool.QueryRow(context.Background(), "SELECT count(*) FROM "+table+" WHERE "+where).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func eventIDs(t *testing.T, app *testutil.App) []string {
	t.Helper()
	page, err := publicevent.NewRepository().List(context.Background(), app.Pool, publicevent.Filter{Source: stockholm.Source}, 100, "")
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]string, 0, len(page.Items))
	for _, e := range page.Items {
		ids = append(ids, e.SourceEventID)
	}
	return ids
}

// backdate moves every observation of a case back in time by age, so the
// open-case refresh considers it due. Shifting (rather than setting) keeps
// the order in which the case's states were last seen.
func backdate(t *testing.T, app *testutil.App, recNo string, age time.Duration) {
	t.Helper()
	if _, err := app.Pool.Exec(context.Background(), "UPDATE source_observations SET last_observed_at = last_observed_at - $2::interval WHERE source = 'stockholm' AND source_record_id = $1", recNo, fmt.Sprintf("%d seconds", int(age.Seconds()))); err != nil {
		t.Fatal(err)
	}
}

const (
	failedA   = "Intyg återkommande besiktning, ej godkänt, L1142264"
	approvedB = "Intyg återkommande besiktning, godkänt, L1142264"
	failedC   = "Intyg återkommande besiktning, ej godkänt, L9999999"
	reminder  = "Påminnelse om användningsförbud"
)

func TestIngest_RealPages(t *testing.T) {
	// The real search page for 2026-09-23 lists eight cases; two of their
	// real pages are served, the rest are cut from the search page.
	svc := &fakeService{cases: map[string][]byte{"1327421": readFixture(t, "case_1327421_failed.html"), "1327424": readFixture(t, "case_1327424_failed_multi_id.html")}, status: map[string]int{}}
	svc.search = searchPage(t,
		caseSpec{recNo: "1327421", diary: "2026-15984", property: "Kronkvarnen 39", address: "Artillerigatan 48", startedOn: "2026-09-23"},
		caseSpec{recNo: "1327424", diary: "2026-15987", property: "Minan 4", address: "Karlavägen 76", startedOn: "2026-09-23"},
	)
	app, server := newApp(t, svc)
	ctx := context.Background()

	stats, err := app.Stockholm.Run(ctx, window(t, "2026-09-23", "2026-09-23"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	want := stockholm.Stats{SearchCasesDiscovered: 2, NewCasesFetched: 2, CasePagesFetched: 2, RecordsObserved: 2, ObservationsInserted: 2, FailedCertificates: 2, EventsInserted: 2}
	if stats != want {
		t.Errorf("stats = %+v\nwant   %+v", stats, want)
	}
	repo := publicevent.NewRepository()
	page, _ := repo.List(ctx, app.Pool, publicevent.Filter{Source: stockholm.Source, EventType: publicevent.EventTypeMotorisedBuildingEquipmentInspectionFailed}, 10, "")
	if len(page.Items) != 2 {
		t.Fatalf("events = %d", len(page.Items))
	}
	byTitle := map[string]publicevent.PublicEvent{}
	for _, e := range page.Items {
		byTitle[e.Title] = e
	}
	e, ok := byTitle["Intyg återkommande besiktning, ej godkänt, 53119"]
	if !ok || e.Source != "stockholm" || e.EventType != publicevent.EventTypeMotorisedBuildingEquipmentInspectionFailed ||
		e.OccurredOn.Format("2006-01-02") != "2026-09-23" || e.OrganisationNumber != "" || e.OrganisationName != "" || e.WorkplaceCFAR != "" || e.WorkplaceName != "" ||
		e.PropertyMunicipalityCode != "0180" || e.PropertyDesignation != "Kronkvarnen 39" || e.PropertyAddress != "Artillerigatan 48" ||
		e.SourceURL != server.URL+"/Byggochplantjansten/arende/arende/1327421?dataSource=Active" || !strings.HasPrefix(e.SourceEventID, "1327421:") {
		t.Errorf("event = %+v", e)
	}
	var record map[string]any
	_ = json.Unmarshal(e.Record, &record)
	if record["district"] != "Östermalm" || record["diary_number"] != "2026-15984" || record["closed_on"] != "" || record["title"] == nil {
		t.Errorf("payload = %s", e.Record)
	}
	docs, _ := record["documents"].([]any)
	if len(docs) != 1 {
		t.Errorf("payload documents = %v", record["documents"])
	}
	multi, ok := byTitle["Intyg återkommande besiktning, ej godkänt, 55117, 53489, 55116"]
	if !ok || multi.PropertyDesignation != "Minan 4" {
		t.Errorf("multi-device certificate is one event: %+v", multi)
	}

	// Same window again: idempotent.
	stats, err = app.Stockholm.Run(ctx, window(t, "2026-09-23", "2026-09-23"))
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	want = stockholm.Stats{SearchCasesDiscovered: 2, NewCasesFetched: 2, CasePagesFetched: 2, RecordsObserved: 2, FailedCertificates: 2, EventsUnchanged: 2}
	if stats != want {
		t.Errorf("second run stats = %+v\nwant   %+v", stats, want)
	}
	if countRows(t, app, "public_events", "source = 'stockholm'") != 2 || countRows(t, app, "source_observations", "source = 'stockholm'") != 2 {
		t.Error("re-run must not add rows")
	}
	obs, _ := repo.ListObservations(ctx, app.Pool, stockholm.Source, "1327421")
	if len(obs) != 1 || !obs[0].LastObservedAt.After(obs[0].FirstObservedAt) || !strings.Contains(obs[0].Raw, "documentList") {
		t.Errorf("observation = %+v", obs)
	}
}

func TestIngest_OnlyExplicitFailuresBecomeEvents(t *testing.T) {
	cases := []caseSpec{
		{recNo: "1", diary: "2026-1", property: "Aida 1", address: "Gatan 1", district: "Norrmalm", startedOn: "2026-09-01", docs: []docSpec{{desc: "Intyg återkommande besiktning, godkänt, S1", ts: "2026-09-01T10:00:00"}}},
		{recNo: "2", diary: "2026-2", property: "Aida 2", address: "Gatan 2", district: "Norrmalm", startedOn: "2026-09-01", docs: []docSpec{{desc: "Intyg återkommande besiktning, delvis godkänt, L2", ts: "2026-09-01T10:00:00"}}},
		{recNo: "3", diary: "2026-3", property: "Aida 3", address: "Gatan 3", district: "Norrmalm", startedOn: "2026-09-01", docs: []docSpec{{desc: reminder, category: "Skrivelse Ut", ts: "2026-09-01T10:00:00"}, {desc: "Klagomål på hiss", ts: "2026-09-01T11:00:00"}}},
		{recNo: "4", diary: "2026-4", property: "Aida 4", address: "", district: "Norrmalm", startedOn: "2026-09-01", docs: []docSpec{{desc: "Intyg ombesiktning, ej godkänt, L4", ts: "2026-09-01T10:00:00"}}},
		{recNo: "5", diary: "2026-5", property: "Aida 5", address: "Gatan 5", district: "Norrmalm", startedOn: "2026-09-01", docs: []docSpec{{desc: failedA, category: "Skrivelse Ut", ts: "2026-09-01T10:00:00"}}},
	}
	svc := &fakeService{cases: map[string][]byte{}, status: map[string]int{}}
	for _, c := range cases {
		svc.cases[c.recNo] = casePage(t, c)
	}
	svc.search = searchPage(t, cases...)
	app, _ := newApp(t, svc)

	stats, err := app.Stockholm.Run(context.Background(), window(t, "2026-09-01", "2026-09-01"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	// Five template-titled cases, one explicit failure (the re-inspection in
	// case 4). Case 5's outgoing copy is not an incoming certificate.
	want := stockholm.Stats{SearchCasesDiscovered: 5, NewCasesFetched: 5, CasePagesFetched: 5, RecordsObserved: 5, ObservationsInserted: 5, FailedCertificates: 1, DocumentsExcluded: 5, EventsInserted: 1}
	if stats != want {
		t.Errorf("stats = %+v\nwant   %+v", stats, want)
	}
	ids := eventIDs(t, app)
	if len(ids) != 1 || !strings.HasPrefix(ids[0], "4:") {
		t.Errorf("events = %v", ids)
	}
	page, _ := publicevent.NewRepository().List(context.Background(), app.Pool, publicevent.Filter{Source: stockholm.Source}, 10, "")
	if e := page.Items[0]; e.PropertyDesignation != "Aida 4" || e.PropertyAddress != "" || e.PropertyMunicipalityCode != "0180" {
		t.Errorf("event without address = %+v", e)
	}
	if countRows(t, app, "source_observations", "source = 'stockholm'") != 5 {
		t.Error("every case page is observed, event or not")
	}
}

func TestIngest_DocumentFingerprintIdentity(t *testing.T) {
	c := caseSpec{recNo: "10", diary: "2026-10", property: "Piloten 2", address: "Gondolgatan 16", district: "Skarpnäcks Gård", startedOn: "2026-01-05", docs: []docSpec{{desc: failedA, ts: "2026-01-05T12:42:07"}}}
	svc := &fakeService{cases: map[string][]byte{"10": casePage(t, c)}, status: map[string]int{}}
	svc.search = searchPage(t, c)
	app, _ := newApp(t, svc)
	ctx := context.Background()
	repo := publicevent.NewRepository()
	run := func(label string) stockholm.Stats {
		t.Helper()
		stats, err := app.Stockholm.Run(ctx, window(t, "2026-01-05", "2026-01-05"))
		if err != nil {
			t.Fatalf("%s: %v", label, err)
		}
		return stats
	}

	// Failure A.
	stats := run("initial")
	if stats.EventsInserted != 1 || stats.ObservationsInserted != 1 {
		t.Fatalf("initial stats = %+v", stats)
	}
	first := eventIDs(t, app)
	idA := first[0]
	before, _ := repo.List(ctx, app.Pool, publicevent.Filter{Source: stockholm.Source}, 10, "")

	// Approved B appended: the case changes, A keeps its identity, nothing new.
	c.docs = []docSpec{{desc: approvedB, ts: "2026-04-01T09:00:00"}, {desc: failedA, ts: "2026-01-05T12:42:07"}}
	svc.set(nil, map[string][]byte{"10": casePage(t, c)})
	stats = run("approved appended")
	if stats.ObservationsInserted != 1 || stats.EventsInserted != 0 || stats.EventsUpdated != 1 || stats.FailedCertificates != 1 || stats.DocumentsExcluded != 1 {
		t.Errorf("approved appended stats = %+v", stats)
	}
	if ids := eventIDs(t, app); len(ids) != 1 || ids[0] != idA {
		t.Errorf("events after approval = %v, want only %s", ids, idA)
	}
	after, _ := repo.List(ctx, app.Pool, publicevent.Filter{Source: stockholm.Source}, 10, "")
	if after.Items[0].ID != before.Items[0].ID || after.Items[0].ObservationID == before.Items[0].ObservationID || !after.Items[0].FirstObservedAt.Equal(before.Items[0].FirstObservedAt) {
		t.Errorf("event A must keep its identity and follow the new observation: %+v -> %+v", before.Items[0], after.Items[0])
	}
	if obs, _ := repo.ListObservations(ctx, app.Pool, stockholm.Source, "10"); len(obs) != 2 {
		t.Errorf("observations = %d, want 2 states", len(obs))
	}

	// Reordered document list: same ids, no duplicates.
	c.docs = []docSpec{{desc: failedA, ts: "2026-01-05T12:42:07"}, {desc: approvedB, ts: "2026-04-01T09:00:00"}}
	svc.set(nil, map[string][]byte{"10": casePage(t, c)})
	stats = run("reordered")
	if stats.EventsInserted != 0 || stats.EventsUpdated != 1 {
		t.Errorf("reordered stats = %+v", stats)
	}
	if ids := eventIDs(t, app); len(ids) != 1 || ids[0] != idA {
		t.Errorf("events after reorder = %v", ids)
	}

	// A second, distinct failure C appended: exactly one new event.
	c.docs = append(c.docs, docSpec{desc: failedC, ts: "2026-09-25T08:00:00"})
	svc.set(nil, map[string][]byte{"10": casePage(t, c)})
	stats = run("failure C appended")
	if stats.EventsInserted != 1 || stats.EventsUpdated != 1 || stats.FailedCertificates != 2 {
		t.Errorf("failure C stats = %+v", stats)
	}
	ids := eventIDs(t, app)
	if len(ids) != 2 || (ids[0] != idA && ids[1] != idA) {
		t.Errorf("events after C = %v", ids)
	}
	stats = run("failure C again")
	if stats.EventsInserted != 0 || stats.EventsUnchanged != 2 || stats.ObservationsInserted != 0 {
		t.Errorf("idempotent re-run stats = %+v", stats)
	}

	// Two indistinguishable rows (same timestamp, category, description):
	// one observable event, counted as a duplicate, never an ordinal.
	c.docs = append(c.docs, docSpec{desc: failedC, ts: "2026-09-25T08:00:00"})
	svc.set(nil, map[string][]byte{"10": casePage(t, c)})
	stats = run("duplicate rows")
	if stats.FailedCertificates != 3 || stats.DuplicateCertificates != 1 || stats.EventsInserted != 0 || len(eventIDs(t, app)) != 2 {
		t.Errorf("duplicate rows stats = %+v, events = %v", stats, eventIDs(t, app))
	}
	if !strings.Contains(app.Logs(), "indistinguishable") {
		t.Error("duplicate rows must be logged")
	}
}

func TestIngest_OpenCaseRefresh(t *testing.T) {
	old := caseSpec{recNo: "100", diary: "2026-100", property: "Duvan 6", address: "Klara Södra Kyrkogata 1", district: "Norrmalm", startedOn: "2026-01-10", docs: []docSpec{{desc: failedA, ts: "2026-01-10T09:00:00"}}}
	recent := caseSpec{recNo: "200", diary: "2026-200", property: "Harpan 32", address: "Gatan 2", district: "Vasastaden", startedOn: "2026-02-01", docs: []docSpec{{desc: failedC, ts: "2026-02-01T09:00:00"}}}
	closed := caseSpec{recNo: "300", diary: "2026-300", property: "Rotundan 6", address: "Gatan 3", district: "Kista", startedOn: "2026-01-12", closedOn: "2026-03-01", docs: []docSpec{{desc: failedA, ts: "2026-01-12T09:00:00"}, {desc: approvedB, ts: "2026-02-28T09:00:00"}}}
	svc := &fakeService{cases: map[string][]byte{"100": casePage(t, old), "200": casePage(t, recent), "300": casePage(t, closed)}, status: map[string]int{}}
	svc.search = searchPage(t, old, recent, closed)
	app, _ := newApp(t, svc)
	ctx := context.Background()
	app.Stockholm.RefreshInterval = 7 * 24 * time.Hour

	// Seed: all three cases discovered through a wide window.
	if _, err := app.Stockholm.Run(ctx, window(t, "2026-01-01", "2026-02-28")); err != nil {
		t.Fatal(err)
	}
	if n := countRows(t, app, "public_events", "source = 'stockholm'"); n != 3 {
		t.Fatalf("seed events = %d", n)
	}

	// A new-case window that includes none of them: the old open case is
	// due (checked 8 days ago), the recent one is not (checked now), the
	// closed one never is.
	backdate(t, app, "100", 8*24*time.Hour)
	backdate(t, app, "300", 30*24*time.Hour)
	svc.set(searchPage(t), nil)
	stats, err := app.Stockholm.Run(ctx, window(t, "2026-09-25", "2026-09-25"))
	if err != nil {
		t.Fatal(err)
	}
	want := stockholm.Stats{OpenCasesRefreshed: 1, CasePagesFetched: 1, RecordsObserved: 1, FailedCertificates: 1, EventsUnchanged: 1}
	if stats != want {
		t.Errorf("refresh stats = %+v\nwant   %+v", stats, want)
	}
	if got := svc.caseRequests(); len(got) != 1 || got[0] != "100" {
		t.Errorf("case pages fetched = %v, want only the due open case", got)
	}

	// Refreshed and unchanged: not due again.
	svc.set(nil, nil)
	stats, _ = app.Stockholm.Run(ctx, window(t, "2026-09-25", "2026-09-25"))
	if stats.OpenCasesRefreshed != 0 || len(svc.caseRequests()) != 0 {
		t.Errorf("just-refreshed case fetched again: %+v %v", stats, svc.caseRequests())
	}

	// The old case receives a new failure long after its start: refresh
	// finds it and emits exactly one new event.
	backdate(t, app, "100", 8*24*time.Hour)
	old.docs = append(old.docs, docSpec{desc: failedC, ts: "2026-09-24T10:00:00"})
	svc.set(nil, map[string][]byte{"100": casePage(t, old)})
	stats, err = app.Stockholm.Run(ctx, window(t, "2026-09-25", "2026-09-25"))
	if err != nil {
		t.Fatal(err)
	}
	if stats.OpenCasesRefreshed != 1 || stats.ObservationsInserted != 1 || stats.EventsInserted != 1 || stats.EventsUpdated != 1 {
		t.Errorf("new failure via refresh stats = %+v", stats)
	}
	if n := countRows(t, app, "public_events", "source = 'stockholm' AND source_event_id LIKE '100:%'"); n != 2 {
		t.Errorf("events of case 100 = %d, want 2", n)
	}

	// The old case receives an approved certificate: new observation, no
	// new failure event.
	backdate(t, app, "100", 8*24*time.Hour)
	old.docs = append(old.docs, docSpec{desc: approvedB, ts: "2026-09-25T10:00:00"})
	svc.set(nil, map[string][]byte{"100": casePage(t, old)})
	stats, _ = app.Stockholm.Run(ctx, window(t, "2026-09-25", "2026-09-25"))
	if stats.ObservationsInserted != 1 || stats.EventsInserted != 0 || stats.DocumentsExcluded != 1 {
		t.Errorf("approval via refresh stats = %+v", stats)
	}

	// The old case closes: new observation, and it leaves the refresh set.
	backdate(t, app, "100", 8*24*time.Hour)
	old.closedOn = "2026-09-26"
	svc.set(nil, map[string][]byte{"100": casePage(t, old)})
	stats, _ = app.Stockholm.Run(ctx, window(t, "2026-09-25", "2026-09-25"))
	if stats.OpenCasesRefreshed != 1 || stats.ObservationsInserted != 1 {
		t.Errorf("closure stats = %+v", stats)
	}
	backdate(t, app, "100", 8*24*time.Hour)
	svc.set(nil, nil)
	stats, _ = app.Stockholm.Run(ctx, window(t, "2026-09-25", "2026-09-25"))
	if stats.OpenCasesRefreshed != 0 || len(svc.caseRequests()) != 0 {
		t.Errorf("closed case must not be refreshed: %+v %v", stats, svc.caseRequests())
	}
	if n := countRows(t, app, "public_events", "source = 'stockholm'"); n != 4 {
		t.Errorf("events total = %d", n)
	}

	// A newly discovered case that is also due is fetched once per run.
	backdate(t, app, "200", 8*24*time.Hour)
	svc.set(searchPage(t, recent), nil)
	stats, _ = app.Stockholm.Run(ctx, window(t, "2026-02-01", "2026-02-01"))
	if stats.NewCasesFetched != 1 || stats.OpenCasesRefreshed != 0 || len(svc.caseRequests()) != 1 {
		t.Errorf("discovered and due case fetched twice: %+v %v", stats, svc.caseRequests())
	}
}

func TestIngest_AbortsOnSourceFailure(t *testing.T) {
	a := caseSpec{recNo: "1", diary: "2026-1", property: "Aida 1", address: "Gatan 1", district: "Norrmalm", startedOn: "2026-09-01", docs: []docSpec{{desc: failedA, ts: "2026-09-01T10:00:00"}}}
	b := caseSpec{recNo: "2", diary: "2026-2", property: "Aida 2", address: "Gatan 2", district: "Norrmalm", startedOn: "2026-09-01", docs: []docSpec{{desc: failedC, ts: "2026-09-01T10:00:00"}}}
	svc := &fakeService{cases: map[string][]byte{"1": casePage(t, a), "2": casePage(t, b)}, status: map[string]int{"2": http.StatusInternalServerError}}
	svc.search = searchPage(t, a, b)
	app, _ := newApp(t, svc)
	ctx := context.Background()

	// One case page fails: the run aborts (it must not claim completeness)
	// but what was persisted before stays, so a re-run is cheap.
	stats, err := app.Stockholm.Run(ctx, window(t, "2026-09-01", "2026-09-01"))
	var se *stockholm.StatusError
	if !errors.As(err, &se) || se.StatusCode != http.StatusInternalServerError || !strings.Contains(err.Error(), "case 2") {
		t.Fatalf("err = %v, want a status error naming case 2", err)
	}
	if stats.CasePagesFetched != 1 || stats.EventsInserted != 1 {
		t.Errorf("stats = %+v", stats)
	}

	// Malformed case page and search page also abort.
	svc.set(nil, map[string][]byte{"2": []byte("<html><body>maintenance</body></html>")})
	svc.mu.Lock()
	svc.status["2"] = 0
	svc.mu.Unlock()
	if _, err := app.Stockholm.Run(ctx, window(t, "2026-09-01", "2026-09-01")); !errors.Is(err, stockholm.ErrUnexpectedMarkup) {
		t.Errorf("malformed case page: err = %v", err)
	}
	svc.set([]byte("<html><body>maintenance</body></html>"), nil)
	if _, err := app.Stockholm.Run(ctx, window(t, "2026-09-01", "2026-09-01")); !errors.Is(err, stockholm.ErrUnexpectedMarkup) {
		t.Errorf("malformed search page: err = %v", err)
	}
	if _, err := app.Stockholm.Run(ctx, stockholm.Window{}); err == nil {
		t.Error("empty window must fail")
	}
	if n := countRows(t, app, "public_events", "source = 'stockholm'"); n != 1 {
		t.Errorf("events = %d, want the one persisted before the failure", n)
	}
}

// TestIngest_RefreshUsesMostRecentlyObservedState guards the selection of
// a case's current state. Observations are deduplicated by content, so a
// case that goes open (A) → closed (B) → open (A again) refreshes A's
// last_observed_at instead of inserting a third row; B keeps the later
// first_observed_at. The refresh must then treat A as current and re-read
// the case, not pick B merely because it was created later.
func TestIngest_RefreshUsesMostRecentlyObservedState(t *testing.T) {
	c := caseSpec{recNo: "500", diary: "2026-500", property: "Bävern 1", address: "Gatan 5", district: "Södermalm", startedOn: "2026-01-20", docs: []docSpec{{desc: failedA, ts: "2026-01-20T09:00:00"}}}
	svc := &fakeService{cases: map[string][]byte{"500": casePage(t, c)}, status: map[string]int{}}
	svc.search = searchPage(t, c)
	app, _ := newApp(t, svc)
	ctx := context.Background()
	w := window(t, "2026-01-20", "2026-01-20")
	run := func(label string) stockholm.Stats {
		t.Helper()
		stats, err := app.Stockholm.Run(ctx, w)
		if err != nil {
			t.Fatalf("%s: %v", label, err)
		}
		return stats
	}

	// State A: open.
	if stats := run("A open"); stats.ObservationsInserted != 1 {
		t.Fatalf("A stats = %+v", stats)
	}
	// State B: closed.
	c.closedOn = "2026-03-01"
	svc.set(nil, map[string][]byte{"500": casePage(t, c)})
	if stats := run("B closed"); stats.ObservationsInserted != 1 {
		t.Fatalf("B stats = %+v", stats)
	}
	// Back to state A: open again. No new row; A's last_observed_at moves.
	c.closedOn = ""
	svc.set(nil, map[string][]byte{"500": casePage(t, c)})
	if stats := run("A again"); stats.ObservationsInserted != 0 || stats.RecordsObserved != 1 {
		t.Fatalf("A again stats = %+v", stats)
	}
	obs, err := publicevent.NewRepository().ListObservations(ctx, app.Pool, stockholm.Source, "500")
	if err != nil || len(obs) != 2 {
		t.Fatalf("observations = %d, %v; want the two states only", len(obs), err)
	}
	var a, b publicevent.SourceObservation
	for _, o := range obs {
		if strings.Contains(string(o.Payload), "2026-03-01") {
			b = o
		} else {
			a = o
		}
	}
	if a.ID == uuid.Nil || b.ID == uuid.Nil || !b.FirstObservedAt.After(a.FirstObservedAt) || !a.LastObservedAt.After(b.LastObservedAt) {
		t.Fatalf("precondition: B must be created later and A seen later: A=%+v B=%+v", a, b)
	}

	// Make the case due without disturbing that order, and run a window
	// with no new cases: the case is open by its current state and must be
	// refreshed.
	backdate(t, app, "500", 8*24*time.Hour)
	svc.set(searchPage(t), nil)
	stats := run("refresh")
	if stats.OpenCasesRefreshed != 1 || stats.CasePagesFetched != 1 {
		t.Errorf("refresh stats = %+v, want the case re-read (current state open)", stats)
	}
	if got := svc.caseRequests(); len(got) != 1 || got[0] != "500" {
		t.Errorf("case pages fetched = %v", got)
	}

	// And the mirror image: open → closed → closed again stays out.
	c.closedOn = "2026-03-01"
	svc.set(searchPage(t, c), map[string][]byte{"500": casePage(t, c)})
	run("B again")
	backdate(t, app, "500", 8*24*time.Hour)
	svc.set(searchPage(t), nil)
	stats = run("no refresh")
	if stats.OpenCasesRefreshed != 0 || len(svc.caseRequests()) != 0 {
		t.Errorf("closed by its current state must not be refreshed: %+v %v", stats, svc.caseRequests())
	}
}
