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
// later page is the empty result page, unless status forces an error.
type fakeDiary struct {
	mu       sync.Mutex
	body     []byte
	empty    []byte
	status   int
	requests int
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
	want := arbetsmiljoverket.Stats{Pages: 2, RecordsObserved: 3, InspectionNotices: 3, ObservationsInserted: 3, EventsInserted: 3}
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
	want = arbetsmiljoverket.Stats{Pages: 2, RecordsObserved: 3, InspectionNotices: 3, EventsUnchanged: 3}
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
	want = arbetsmiljoverket.Stats{Pages: 2, RecordsObserved: 3, InspectionNotices: 3, ObservationsInserted: 1, EventsUpdated: 1, EventsUnchanged: 2}
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
	want := arbetsmiljoverket.Stats{Pages: 2, RecordsObserved: 2, InspectionNotices: 1, OtherDocumentTypes: 1, ParseFailures: 2, ObservationsInserted: 1, EventsInserted: 1}
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
