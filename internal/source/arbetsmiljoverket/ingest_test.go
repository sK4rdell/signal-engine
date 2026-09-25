package arbetsmiljoverket_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sK4rdell/signal-engine/internal/platform/config"
	"github.com/sK4rdell/signal-engine/internal/platform/testutil"
	"github.com/sK4rdell/signal-engine/internal/publicevent"
	"github.com/sK4rdell/signal-engine/internal/source/arbetsmiljoverket"
)

// fakeDiary serves fixture pages: page 1 is whatever body is set, every
// later page is the empty result page, unless status forces an error. A
// search with SearchText (the ingester's case-scoped check) is answered
// from caseBodies when the text is known there, otherwise like any other
// search; caseStatus forces an error on those searches only.
type fakeDiary struct {
	mu         sync.Mutex
	body       []byte
	empty      []byte
	status     int
	caseBodies map[string][]byte
	caseStatus int
	// caseEveryPage serves the case body on every page number, so the
	// case-scoped search never reaches an empty page.
	caseEveryPage bool
	requests      int
	// caseSearches records the SearchText values asked for.
	caseSearches []string
}

func (f *fakeDiary) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests++
	if f.status != 0 {
		http.Error(w, "boom", f.status)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if text := r.URL.Query().Get("SearchText"); text != "" {
		f.caseSearches = append(f.caseSearches, text)
		if f.caseStatus != 0 {
			http.Error(w, "boom", f.caseStatus)
			return
		}
		if body, ok := f.caseBodies[text]; ok && (f.caseEveryPage || r.URL.Query().Get("p") == "1") {
			_, _ = w.Write(body)
			return
		}
	}
	if r.URL.Query().Get("p") == "1" {
		_, _ = w.Write(f.body)
		return
	}
	_, _ = w.Write(f.empty)
}

func (f *fakeDiary) set(body []byte, status int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.body, f.status = body, status
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func window(t *testing.T) arbetsmiljoverket.Window {
	t.Helper()
	from, _ := time.Parse("2006-01-02", "2026-09-15")
	to, _ := time.Parse("2006-01-02", "2026-09-23")
	return arbetsmiljoverket.Window{From: from, To: to}
}

// recordField reads one string field of an event's observation payload.
func recordField(t *testing.T, record []byte, key string) string {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(record, &m); err != nil {
		t.Fatalf("record is not JSON: %v\n%s", err, record)
	}
	v, _ := m[key].(string)
	return v
}

func countRows(t *testing.T, app *testutil.App, table string) int {
	t.Helper()
	var n int
	if err := app.Pool.QueryRow(context.Background(), "SELECT count(*) FROM "+table).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestIngest_PersistsObservationsAndEventsIdempotently(t *testing.T) {
	diary := &fakeDiary{body: readFixture(t, "search_page.html"), empty: readFixture(t, "search_page_empty.html")}
	server := httptest.NewServer(diary)
	defer server.Close()
	app := testutil.NewApp(t, testutil.WithConfig(func(cfg *config.Config) {
		cfg.Sources.Arbetsmiljoverket.BaseURL = server.URL
	}))
	repo := publicevent.NewRepository()
	ctx := context.Background()

	// First run: three new records.
	stats, err := app.Arbetsmiljoverket.Run(ctx, window(t))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	want := arbetsmiljoverket.Stats{Pages: 2, RecordsObserved: 3, RecordsAccepted: 3, ObservationsInserted: 3, EventsInserted: 3}
	if stats != want {
		t.Errorf("stats = %+v\nwant   %+v", stats, want)
	}
	if n := countRows(t, app, "public_events"); n != 3 {
		t.Errorf("public_events = %d", n)
	}

	page, err := repo.List(ctx, app.Pool, publicevent.Filter{}, 10, "")
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]publicevent.PublicEvent{}
	for _, e := range page.Items {
		byID[e.SourceEventID] = e
	}
	full := byID["2026/060943-2"]
	if full.Source != "arbetsmiljoverket" || full.EventType != publicevent.EventTypeWorkEnvironmentInspectionNotice ||
		full.OccurredOn.Format("2006-01-02") != "2026-09-23" || full.Title != "Inspektion inom Fortlöpande tillsyn - Unga i arbetslivet" ||
		full.OrganisationNumber != "5594800418" || full.OrganisationName != "MITTEN MACK AB" ||
		full.WorkplaceCFAR != "71466304" || full.WorkplaceName != "MITTEN MACK AB" ||
		!strings.HasPrefix(full.SourceURL, server.URL+"/") || !strings.Contains(full.SourceURL, "Case/?id=2026/060943") ||
		recordField(t, full.Record, "case_status") != "Pågående" {
		t.Errorf("full event = %+v", full)
	}
	noOrg := byID["2026/058034-4"]
	if noOrg.OrganisationNumber != "" || noOrg.OrganisationName != "" || noOrg.WorkplaceName != "" || noOrg.WorkplaceCFAR != "73278392" {
		t.Errorf("event without organisation = %+v", noOrg)
	}
	obs, err := repo.ListObservations(ctx, app.Pool, arbetsmiljoverket.Source, "2026/060943-2")
	if err != nil || len(obs) != 1 || obs[0].ID != full.ObservationID || !strings.Contains(obs[0].Raw, "document-list__item") || !strings.Contains(obs[0].SourceURL, "p=1") {
		t.Fatalf("observations = %+v, %v", obs, err)
	}
	if strings.Contains(string(obs[0].Payload), "RawHTML") || strings.Contains(string(obs[0].Payload), "<li") {
		t.Errorf("payload leaks raw html: %.200s", obs[0].Payload)
	}

	// Second run over the same window: nothing new, nothing duplicated.
	stats, err = app.Arbetsmiljoverket.Run(ctx, window(t))
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	want = arbetsmiljoverket.Stats{Pages: 2, RecordsObserved: 3, RecordsAccepted: 3, EventsUnchanged: 3}
	if stats != want {
		t.Errorf("second run stats = %+v\nwant   %+v", stats, want)
	}
	if n := countRows(t, app, "public_events"); n != 3 {
		t.Errorf("public_events after re-run = %d", n)
	}
	if n := countRows(t, app, "source_observations"); n != 3 {
		t.Errorf("source_observations after re-run = %d", n)
	}
	obs, _ = repo.ListObservations(ctx, app.Pool, arbetsmiljoverket.Source, "2026/060943-2")
	if len(obs) != 1 || !obs[0].LastObservedAt.After(obs[0].FirstObservedAt) {
		t.Errorf("re-observation should refresh last_observed_at: %+v", obs)
	}
	unchanged, _ := repo.Get(ctx, app.Pool, full.ID)
	if !unchanged.UpdatedAt.Equal(full.UpdatedAt) {
		t.Errorf("unchanged event was touched: %v -> %v", full.UpdatedAt, unchanged.UpdatedAt)
	}

	// Third run: the source changed one record (case closed). The history
	// keeps both states; the event follows the latest.
	// The source encodes non-ASCII letters as entities.
	changed := strings.Replace(string(diary.body), "P&#229;g&#229;ende", "Avslutat", 1)
	if changed == string(diary.body) {
		t.Fatal("fixture did not contain the status to change")
	}
	diary.set([]byte(changed), 0)
	stats, err = app.Arbetsmiljoverket.Run(ctx, window(t))
	if err != nil {
		t.Fatalf("third run: %v", err)
	}
	want = arbetsmiljoverket.Stats{Pages: 2, RecordsObserved: 3, RecordsAccepted: 3, ObservationsInserted: 1, EventsUpdated: 1, EventsUnchanged: 2}
	if stats != want {
		t.Errorf("third run stats = %+v\nwant   %+v", stats, want)
	}
	obs, _ = repo.ListObservations(ctx, app.Pool, arbetsmiljoverket.Source, "2026/060943-2")
	if len(obs) != 2 || obs[0].ContentHash == obs[1].ContentHash {
		t.Fatalf("observations after change = %+v", obs)
	}
	updated, err := repo.Get(ctx, app.Pool, full.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.ObservationID != obs[1].ID || recordField(t, updated.Record, "case_status") != "Avslutat" || !updated.UpdatedAt.After(full.UpdatedAt) || !updated.FirstObservedAt.Equal(full.FirstObservedAt) {
		t.Errorf("updated event = %+v", updated)
	}
	if n := countRows(t, app, "public_events"); n != 3 {
		t.Errorf("public_events after change = %d", n)
	}
}

func TestIngest_SkipsMalformedRowsAndOtherTypes(t *testing.T) {
	diary := &fakeDiary{body: readFixture(t, "search_page_malformed.html"), empty: readFixture(t, "search_page_empty.html")}
	server := httptest.NewServer(diary)
	defer server.Close()
	app := testutil.NewApp(t, testutil.WithConfig(func(cfg *config.Config) {
		cfg.Sources.Arbetsmiljoverket.BaseURL = server.URL
	}))

	stats, err := app.Arbetsmiljoverket.Run(context.Background(), window(t))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	want := arbetsmiljoverket.Stats{Pages: 2, RecordsObserved: 2, RecordsAccepted: 1, OtherDocumentTypes: 1, ParseFailures: 2, ObservationsInserted: 1, EventsInserted: 1}
	if stats != want {
		t.Errorf("stats = %+v\nwant   %+v", stats, want)
	}
	if n := countRows(t, app, "public_events"); n != 1 {
		t.Errorf("public_events = %d", n)
	}
	logs := app.Logs()
	if !strings.Contains(logs, "source row could not be parsed") || !strings.Contains(logs, "another type despite the filter") {
		t.Errorf("expected warnings in logs:\n%s", logs)
	}
}

func TestIngest_AbortsOnSourceFailure(t *testing.T) {
	diary := &fakeDiary{empty: readFixture(t, "search_page_empty.html"), status: http.StatusInternalServerError}
	server := httptest.NewServer(diary)
	defer server.Close()
	app := testutil.NewApp(t, testutil.WithConfig(func(cfg *config.Config) {
		cfg.Sources.Arbetsmiljoverket.BaseURL = server.URL
	}))

	stats, err := app.Arbetsmiljoverket.Run(context.Background(), window(t))
	var statusErr *arbetsmiljoverket.StatusError
	if !errors.As(err, &statusErr) || statusErr.StatusCode != http.StatusInternalServerError {
		t.Fatalf("err = %v, want *StatusError 500", err)
	}
	if stats.Pages != 0 || countRows(t, app, "public_events") != 0 {
		t.Errorf("stats = %+v", stats)
	}

	// A page that lost its result list means the markup changed: abort
	// rather than treat it as "no results".
	diary.set([]byte("<html><body>maintenance</body></html>"), 0)
	_, err = app.Arbetsmiljoverket.Run(context.Background(), window(t))
	if !errors.Is(err, arbetsmiljoverket.ErrUnexpectedMarkup) {
		t.Fatalf("err = %v, want ErrUnexpectedMarkup", err)
	}
}

func TestIngest_RejectsInvalidWindow(t *testing.T) {
	app := testutil.NewApp(t)
	w := window(t)
	w.From, w.To = w.To, w.From
	if _, err := app.Arbetsmiljoverket.Run(context.Background(), w); err == nil {
		t.Fatal("expected error for reversed window")
	}
	if _, err := app.Arbetsmiljoverket.Run(context.Background(), arbetsmiljoverket.Window{}); err == nil {
		t.Fatal("expected error for empty window")
	}
}

// certificateWindow is any window: the fake diary ignores dates.
func certificateWindow(t *testing.T, from, to string) arbetsmiljoverket.Window {
	t.Helper()
	f, _ := time.Parse("2006-01-02", from)
	u, _ := time.Parse("2006-01-02", to)
	return arbetsmiljoverket.Window{From: f, To: u}
}

func TestIngest_RecurringInspectionFailures(t *testing.T) {
	diary := &fakeDiary{body: readFixture(t, "search_page_certificates.html"), empty: readFixture(t, "search_page_empty.html")}
	server := httptest.NewServer(diary)
	defer server.Close()
	app := testutil.NewApp(t, testutil.WithConfig(func(cfg *config.Config) {
		cfg.Sources.Arbetsmiljoverket.BaseURL = server.URL
	}))
	repo := publicevent.NewRepository()
	ctx := context.Background()
	feed := arbetsmiljoverket.FeedRecurringInspectionFailures

	// The fixture is one real 6.1-49 page: two failed-inspection
	// certificates (a vehicle lift and a compressed-air receiver), an
	// approved inspection sent for information, a certificate filed in an
	// inspection-campaign case, and a Tillsynsmeddelande returned despite
	// the filter. Only the two failures become events.
	stats, err := app.Arbetsmiljoverket.RunFeed(ctx, feed, certificateWindow(t, "2026-09-01", "2026-09-10"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	want := arbetsmiljoverket.Stats{Pages: 1, RecordsObserved: 5, RecordsAccepted: 2, RecordsSkipped: 2, OtherDocumentTypes: 1, ObservationsInserted: 2, EventsInserted: 2}
	if stats != want {
		t.Errorf("stats = %+v\nwant   %+v", stats, want)
	}
	if n := countRows(t, app, "public_events"); n != 2 {
		t.Errorf("public_events = %d", n)
	}
	if n := countRows(t, app, "source_observations"); n != 2 {
		t.Errorf("source_observations = %d (skipped rows must not be observed)", n)
	}

	page, err := repo.List(ctx, app.Pool, publicevent.Filter{EventType: publicevent.EventTypeWorkEquipmentInspectionFailed}, 10, "")
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]publicevent.PublicEvent{}
	for _, e := range page.Items {
		byID[e.SourceEventID] = e
	}
	lift, ok := byID["2026/059517-1"]
	if !ok || lift.Source != "arbetsmiljoverket" || lift.EventType != publicevent.EventTypeWorkEquipmentInspectionFailed ||
		lift.OccurredOn.Format("2006-01-02") != "2026-09-16" || lift.Title != "Återkommande besiktning - fordonslyft flerpelarlyft" ||
		lift.OrganisationNumber != "5569620726" || lift.OrganisationName != "HELSINGE DÄCKCENTER AB" ||
		lift.WorkplaceCFAR != "54274840" || lift.WorkplaceName != "HELSINGE DÄCKCENTER AB" ||
		!strings.HasPrefix(lift.SourceURL, server.URL+"/") || !strings.Contains(lift.SourceURL, "Case/?id=2026/059517") ||
		recordField(t, lift.Record, "document_type") != "Intyg återkommande besiktning" || recordField(t, lift.Record, "origin") != "Inkommande" ||
		recordField(t, lift.Record, "case_title") != lift.Title {
		t.Errorf("vehicle lift event = %+v", lift)
	}
	pressure, ok := byID["2026/049128-1"]
	if !ok || pressure.EventType != publicevent.EventTypeWorkEquipmentInspectionFailed ||
		pressure.OccurredOn.Format("2006-01-02") != "2026-08-06" || pressure.Title != "Återkommande besiktning - Tryckluftbehållare" ||
		pressure.OrganisationNumber != "5591630883" || pressure.WorkplaceCFAR != "60365152" {
		t.Errorf("pressure event = %+v", pressure)
	}
	for _, skipped := range []string{"2026/006641-1", "2026/057530-2", "2026/060618-2"} {
		if _, exists := byID[skipped]; exists {
			t.Errorf("%s must not become an event", skipped)
		}
		if obs, _ := repo.ListObservations(ctx, app.Pool, arbetsmiljoverket.Source, skipped); len(obs) != 0 {
			t.Errorf("%s must not be observed: %+v", skipped, obs)
		}
	}
	obs, err := repo.ListObservations(ctx, app.Pool, arbetsmiljoverket.Source, "2026/059517-1")
	if err != nil || len(obs) != 1 || obs[0].ID != lift.ObservationID || !strings.Contains(obs[0].Raw, "2026/059517-1") {
		t.Fatalf("observations = %+v, %v", obs, err)
	}
	logs := app.Logs()
	if !strings.Contains(logs, "godkänd besiktning") || !strings.Contains(logs, "does not match the feed") ||
		!strings.Contains(logs, "2026/057530-2") || !strings.Contains(logs, "another type despite the filter") ||
		strings.Contains(logs, "could not be parsed") {
		t.Errorf("expected skip and guard logs, no parse failures:\n%s", logs)
	}

	// Overlapping window: the same certificates again, no duplicates.
	stats, err = app.Arbetsmiljoverket.RunFeed(ctx, feed, certificateWindow(t, "2026-09-05", "2026-09-15"))
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	want = arbetsmiljoverket.Stats{Pages: 1, RecordsObserved: 5, RecordsAccepted: 2, RecordsSkipped: 2, OtherDocumentTypes: 1, EventsUnchanged: 2}
	if stats != want {
		t.Errorf("second run stats = %+v\nwant   %+v", stats, want)
	}
	if countRows(t, app, "public_events") != 2 || countRows(t, app, "source_observations") != 2 {
		t.Errorf("re-run changed row counts: events %d, observations %d", countRows(t, app, "public_events"), countRows(t, app, "source_observations"))
	}
	unchanged, _ := repo.Get(ctx, app.Pool, lift.ID)
	if !unchanged.UpdatedAt.Equal(lift.UpdatedAt) {
		t.Errorf("unchanged event was touched: %v -> %v", lift.UpdatedAt, unchanged.UpdatedAt)
	}

	// Changed source record: the vehicle-lift case is closed. A new
	// observation is appended and the event follows it.
	changed := strings.Replace(string(diary.body), "P&#229;g&#229;ende", "Avslutat", 1)
	if changed == string(diary.body) {
		t.Fatal("fixture did not contain the status to change")
	}
	diary.set([]byte(changed), 0)
	stats, err = app.Arbetsmiljoverket.RunFeed(ctx, feed, certificateWindow(t, "2026-09-01", "2026-09-10"))
	if err != nil {
		t.Fatalf("third run: %v", err)
	}
	want = arbetsmiljoverket.Stats{Pages: 1, RecordsObserved: 5, RecordsAccepted: 2, RecordsSkipped: 2, OtherDocumentTypes: 1, ObservationsInserted: 1, EventsUpdated: 1, EventsUnchanged: 1}
	if stats != want {
		t.Errorf("third run stats = %+v\nwant   %+v", stats, want)
	}
	updated, _ := repo.Get(ctx, app.Pool, lift.ID)
	obs, _ = repo.ListObservations(ctx, app.Pool, arbetsmiljoverket.Source, "2026/059517-1")
	if len(obs) != 2 || updated.ObservationID != obs[1].ID || recordField(t, updated.Record, "case_status") != "Avslutat" ||
		updated.Title != lift.Title || !updated.FirstObservedAt.Equal(lift.FirstObservedAt) || countRows(t, app, "public_events") != 2 {
		t.Errorf("updated event = %+v, observations = %d", updated, len(obs))
	}

	// Invalid identifiers on the source: the event keeps its identity, the
	// values are dropped from the event and kept in the payload.
	broken := strings.ReplaceAll(changed, "5569620726", "5569620727") // fails the Luhn check
	broken = strings.ReplaceAll(broken, "54274840", "5427484")        // seven digits
	if broken == changed {
		t.Fatal("fixture did not contain the identifiers to break")
	}
	diary.set([]byte(broken), 0)
	stats, err = app.Arbetsmiljoverket.RunFeed(ctx, feed, certificateWindow(t, "2026-09-01", "2026-09-10"))
	if err != nil {
		t.Fatalf("fourth run: %v", err)
	}
	want = arbetsmiljoverket.Stats{Pages: 1, RecordsObserved: 5, RecordsAccepted: 2, RecordsSkipped: 2, OtherDocumentTypes: 1, ObservationsInserted: 1, EventsUpdated: 1, EventsUnchanged: 1, InvalidOrganisationNumbers: 1, InvalidWorkplaceCFARs: 1}
	if stats != want {
		t.Errorf("fourth run stats = %+v\nwant   %+v", stats, want)
	}
	invalid, _ := repo.Get(ctx, app.Pool, lift.ID)
	if invalid.SourceEventID != "2026/059517-1" || invalid.OrganisationNumber != "" || invalid.WorkplaceCFAR != "" ||
		invalid.OrganisationName != "HELSINGE DÄCKCENTER AB" || invalid.EventType != publicevent.EventTypeWorkEquipmentInspectionFailed ||
		recordField(t, invalid.Record, "organisation_number") != "5569620727" || recordField(t, invalid.Record, "workplace_cfar") != "5427484" {
		t.Errorf("event after invalid identifiers = %+v", invalid)
	}
	if countRows(t, app, "public_events") != 2 {
		t.Errorf("public_events = %d", countRows(t, app, "public_events"))
	}
	if logs := app.Logs(); !strings.Contains(logs, "organisation number failed validation") || !strings.Contains(logs, "workplace CFAR failed validation") {
		t.Errorf("expected validation warnings:\n%s", logs)
	}
}

func TestIngest_InspectionNoticeFeedIsTheDefault(t *testing.T) {
	diary := &fakeDiary{body: readFixture(t, "search_page.html"), empty: readFixture(t, "search_page_empty.html")}
	server := httptest.NewServer(diary)
	defer server.Close()
	app := testutil.NewApp(t, testutil.WithConfig(func(cfg *config.Config) {
		cfg.Sources.Arbetsmiljoverket.BaseURL = server.URL
	}))
	ctx := context.Background()

	stats, err := app.Arbetsmiljoverket.RunFeed(ctx, arbetsmiljoverket.FeedInspectionNotices, window(t))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	want := arbetsmiljoverket.Stats{Pages: 2, RecordsObserved: 3, RecordsAccepted: 3, ObservationsInserted: 3, EventsInserted: 3}
	if stats != want {
		t.Errorf("stats = %+v\nwant   %+v", stats, want)
	}
	// Run (no feed) is the same feed: nothing new.
	stats, err = app.Arbetsmiljoverket.Run(ctx, window(t))
	if err != nil || stats.EventsUnchanged != 3 || stats.EventsInserted != 0 {
		t.Errorf("Run after RunFeed(inspection notices) = %+v, %v", stats, err)
	}
	page, err := publicevent.NewRepository().List(ctx, app.Pool, publicevent.Filter{EventType: publicevent.EventTypeWorkEnvironmentInspectionNotice}, 10, "")
	if err != nil || len(page.Items) != 3 {
		t.Errorf("inspection notices listed = %d, %v", len(page.Items), err)
	}
	if page, _ := publicevent.NewRepository().List(ctx, app.Pool, publicevent.Filter{EventType: publicevent.EventTypeWorkEquipmentInspectionFailed}, 10, ""); len(page.Items) != 0 {
		t.Errorf("inspection notices must not appear as failed inspections: %d", len(page.Items))
	}
	if _, err := app.Arbetsmiljoverket.RunFeed(ctx, arbetsmiljoverket.Feed{Name: "half-configured"}, window(t)); err == nil {
		t.Error("expected an error for an incomplete feed")
	}
}

// eventIDs lists the source event IDs of every failed-inspection event.
func eventIDs(t *testing.T, app *testutil.App) []string {
	t.Helper()
	page, err := publicevent.NewRepository().List(context.Background(), app.Pool, publicevent.Filter{EventType: publicevent.EventTypeWorkEquipmentInspectionFailed}, 100, "")
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]string, 0, len(page.Items))
	for _, e := range page.Items {
		ids = append(ids, e.SourceEventID)
	}
	return ids
}

func TestIngest_OnlyEarliestCertificateInCaseIsAFailure(t *testing.T) {
	// Real case 2026/044084: certificate -1 (2026-06-29) opened the case,
	// certificate -5 (2026-09-24) came from the re-inspection and the case
	// was closed the same day. The case-scoped search always shows both.
	diary := &fakeDiary{
		body:       readFixture(t, "search_page_certificate_later_only.html"),
		empty:      readFixture(t, "search_page_empty.html"),
		caseBodies: map[string][]byte{"2026/044084": readFixture(t, "search_page_case_044084.html")},
	}
	server := httptest.NewServer(diary)
	defer server.Close()
	app := testutil.NewApp(t, testutil.WithConfig(func(cfg *config.Config) {
		cfg.Sources.Arbetsmiljoverket.BaseURL = server.URL
	}))
	ctx := context.Background()
	feed := arbetsmiljoverket.FeedRecurringInspectionFailures

	// Out of order: a narrow window sees only the later certificate first.
	stats, err := app.Arbetsmiljoverket.RunFeed(ctx, feed, certificateWindow(t, "2026-09-20", "2026-09-25"))
	if err != nil {
		t.Fatalf("run 1: %v", err)
	}
	want := arbetsmiljoverket.Stats{Pages: 1, RecordsObserved: 1, RecordsLaterInCase: 1}
	if stats != want {
		t.Errorf("run 1 stats = %+v\nwant   %+v", stats, want)
	}
	if ids := eventIDs(t, app); len(ids) != 0 {
		t.Errorf("later certificate must not become an event: %v", ids)
	}
	if n := countRows(t, app, "source_observations"); n != 0 {
		t.Errorf("later certificate must not be observed: %d", n)
	}
	if len(diary.caseSearches) != 1 || diary.caseSearches[0] != "2026/044084" {
		t.Errorf("case-scoped searches = %v", diary.caseSearches)
	}
	if logs := app.Logs(); !strings.Contains(logs, "not the earliest document of its type") || !strings.Contains(logs, "2026/044084-5") || !strings.Contains(logs, "2026/044084-1") {
		t.Errorf("expected a skip log naming both certificates:\n%s", logs)
	}

	// A wider window later discovers the earlier certificate.
	diary.set(readFixture(t, "search_page_certificate_earlier_only.html"), 0)
	stats, err = app.Arbetsmiljoverket.RunFeed(ctx, feed, certificateWindow(t, "2026-06-01", "2026-09-25"))
	if err != nil {
		t.Fatalf("run 2: %v", err)
	}
	want = arbetsmiljoverket.Stats{Pages: 1, RecordsObserved: 1, RecordsAccepted: 1, ObservationsInserted: 1, EventsInserted: 1}
	if stats != want {
		t.Errorf("run 2 stats = %+v\nwant   %+v", stats, want)
	}
	ids := eventIDs(t, app)
	if len(ids) != 1 || ids[0] != "2026/044084-1" {
		t.Errorf("events = %v, want only the opening certificate", ids)
	}
	ev, _ := publicevent.NewRepository().List(ctx, app.Pool, publicevent.Filter{}, 10, "")
	if e := ev.Items[0]; e.OccurredOn.Format("2006-01-02") != "2026-06-29" || e.Title != "Återkommande besiktning - Lyftbord vid lastkaj" || e.OrganisationNumber != "5593182370" || e.WorkplaceCFAR != "67577551" {
		t.Errorf("event = %+v", e)
	}

	// A window that holds both certificates: still one event, idempotent.
	diary.set(readFixture(t, "search_page_case_044084.html"), 0)
	stats, err = app.Arbetsmiljoverket.RunFeed(ctx, feed, certificateWindow(t, "2026-06-01", "2026-09-25"))
	if err != nil {
		t.Fatalf("run 3: %v", err)
	}
	want = arbetsmiljoverket.Stats{Pages: 1, RecordsObserved: 2, RecordsAccepted: 1, RecordsLaterInCase: 1, EventsUnchanged: 1}
	if stats != want {
		t.Errorf("run 3 stats = %+v\nwant   %+v", stats, want)
	}
	if ids := eventIDs(t, app); len(ids) != 1 || ids[0] != "2026/044084-1" || countRows(t, app, "source_observations") != 1 {
		t.Errorf("after overlapping runs: events %v, observations %d", ids, countRows(t, app, "source_observations"))
	}

	// The case-scoped check is part of source truth: if it fails, the run
	// aborts rather than guessing.
	diary.set(readFixture(t, "search_page_certificate_later_only.html"), 0)
	diary.mu.Lock()
	diary.caseStatus = http.StatusInternalServerError
	diary.mu.Unlock()
	_, err = app.Arbetsmiljoverket.RunFeed(ctx, feed, certificateWindow(t, "2026-09-20", "2026-09-25"))
	var statusErr *arbetsmiljoverket.StatusError
	if !errors.As(err, &statusErr) || !strings.Contains(err.Error(), "2026/044084-5") {
		t.Fatalf("err = %v, want *StatusError naming the document", err)
	}
	if ids := eventIDs(t, app); len(ids) != 1 {
		t.Errorf("failed check must not create events: %v", ids)
	}
}

func TestIngest_SeparateCasesStayIndependent(t *testing.T) {
	// Real pattern: the same organisation and workplace opened two cases
	// the same day (NOBINA, two telfers). Each case's own certificate is
	// its earliest, so both are events.
	diary := &fakeDiary{body: readFixture(t, "search_page_certificates_two_cases.html"), empty: readFixture(t, "search_page_empty.html")}
	server := httptest.NewServer(diary)
	defer server.Close()
	app := testutil.NewApp(t, testutil.WithConfig(func(cfg *config.Config) {
		cfg.Sources.Arbetsmiljoverket.BaseURL = server.URL
	}))

	stats, err := app.Arbetsmiljoverket.RunFeed(context.Background(), arbetsmiljoverket.FeedRecurringInspectionFailures, certificateWindow(t, "2026-04-01", "2026-04-30"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	want := arbetsmiljoverket.Stats{Pages: 1, RecordsObserved: 2, RecordsAccepted: 2, ObservationsInserted: 2, EventsInserted: 2}
	if stats != want {
		t.Errorf("stats = %+v\nwant   %+v", stats, want)
	}
	ids := eventIDs(t, app)
	if len(ids) != 2 || (ids[0] != "2026/024193-1" && ids[1] != "2026/024193-1") || (ids[0] != "2026/024200-1" && ids[1] != "2026/024200-1") {
		t.Errorf("events = %v, want both cases", ids)
	}
	if len(diary.caseSearches) != 2 {
		t.Errorf("each certificate is checked against its own case: %v", diary.caseSearches)
	}
}

func TestIngest_CaseChronologyFailsClosed(t *testing.T) {
	later := readFixture(t, "search_page_certificate_later_only.html")
	empty := readFixture(t, "search_page_empty.html")
	caseBody := string(readFixture(t, "search_page_case_044084.html"))

	// The earlier certificate's row cannot be parsed (its date is broken)
	// while the later one parses: the chronology is not proven, so the
	// candidate must not become a failure event.
	broken := strings.Replace(caseBody, `datetime="2026-06-29"`, `datetime="not-a-date"`, 1)
	if broken == caseBody {
		t.Fatal("fixture did not contain the date to break")
	}
	// The listing claims far more rows than any page shows and every page
	// is non-empty: the bound is hit with rows remaining.
	unbounded := strings.Replace(caseBody, `class="fw-bold">2<`, `class="fw-bold">999<`, 1)
	if unbounded == caseBody {
		t.Fatal("fixture did not contain the total to change")
	}

	for name, tc := range map[string]struct {
		caseBody  string
		everyPage bool
		wantText  string
	}{
		"earlier row unparsable":    {caseBody: broken, wantText: "could not be parsed"},
		"pagination bound exceeded": {caseBody: unbounded, everyPage: true, wantText: "more than 10 pages"},
	} {
		t.Run(name, func(t *testing.T) {
			diary := &fakeDiary{body: later, empty: empty, caseBodies: map[string][]byte{"2026/044084": []byte(tc.caseBody)}, caseEveryPage: tc.everyPage}
			server := httptest.NewServer(diary)
			defer server.Close()
			app := testutil.NewApp(t, testutil.WithConfig(func(cfg *config.Config) {
				cfg.Sources.Arbetsmiljoverket.BaseURL = server.URL
			}))

			stats, err := app.Arbetsmiljoverket.RunFeed(context.Background(), arbetsmiljoverket.FeedRecurringInspectionFailures, certificateWindow(t, "2026-09-20", "2026-09-25"))
			if !errors.Is(err, arbetsmiljoverket.ErrCaseChronologyIncomplete) || !strings.Contains(err.Error(), "2026/044084") ||
				!strings.Contains(err.Error(), "2026/044084-5") || !strings.Contains(err.Error(), tc.wantText) {
				t.Fatalf("err = %v, want ErrCaseChronologyIncomplete naming case and document with %q", err, tc.wantText)
			}
			if stats.RecordsAccepted != 0 || stats.RecordsLaterInCase != 0 || stats.EventsInserted != 0 {
				t.Errorf("stats = %+v", stats)
			}
			if ids := eventIDs(t, app); len(ids) != 0 {
				t.Errorf("no event may be emitted on an unproven chronology: %v", ids)
			}
			if n := countRows(t, app, "source_observations"); n != 0 {
				t.Errorf("no observation may be recorded for the candidate: %d", n)
			}
			if tc.everyPage && len(diary.caseSearches) != 10 {
				t.Errorf("case-scoped searches = %d, want the bound of 10", len(diary.caseSearches))
			}
		})
	}
}
