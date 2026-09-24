package klimatklivet_test

import (
	"bytes"
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

	"github.com/xuri/excelize/v2"

	"github.com/sK4rdell/signal-engine/internal/platform/config"
	"github.com/sK4rdell/signal-engine/internal/platform/testutil"
	"github.com/sK4rdell/signal-engine/internal/publicevent"
	"github.com/sK4rdell/signal-engine/internal/source/klimatklivet"
)

const filePath = "/49f446/globalassets/amnen/klimatomstallning/klimatklivet/dokument/beviljade-ansokningar-till-klimatklivet-260701.xlsx"

// fakeSite serves the results page and one dataset file version with an
// ETag, honouring If-None-Match like the real CDN does.
type fakeSite struct {
	mu       sync.Mutex
	page     []byte
	file     []byte
	etag     string
	status   int
	requests []string
}

func (f *fakeSite) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, r.Method+" "+r.URL.Path)
	if f.status != 0 {
		http.Error(w, "boom", f.status)
		return
	}
	switch r.URL.Path {
	case klimatklivet.ResultsPagePath:
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(f.page)
	case filePath:
		if r.Header.Get("If-None-Match") == f.etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", f.etag)
		w.Header().Set("Last-Modified", "Tue, 07 Jul 2026 12:21:10 GMT")
		_, _ = w.Write(f.file)
	default:
		http.NotFound(w, r)
	}
}

func (f *fakeSite) set(file []byte, etag string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.file, f.etag = file, etag
}

func (f *fakeSite) fileRequests() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, r := range f.requests {
		if strings.HasSuffix(r, ".xlsx") {
			n++
		}
	}
	return n
}

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func newSite(t *testing.T, file string) (*fakeSite, *httptest.Server, *testutil.App) {
	t.Helper()
	site := &fakeSite{page: fixture(t, "results_page.html"), file: fixture(t, file), etag: `"v1"`}
	server := httptest.NewServer(site)
	t.Cleanup(server.Close)
	app := testutil.NewApp(t, testutil.WithConfig(func(cfg *config.Config) {
		cfg.Sources.Klimatklivet.BaseURL = server.URL
	}))
	return site, server, app
}

func countRows(t *testing.T, app *testutil.App, sql string) int {
	t.Helper()
	var n int
	if err := app.Pool.QueryRow(context.Background(), sql).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func recordField(t *testing.T, record []byte, key string) any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(record, &m); err != nil {
		t.Fatalf("record is not JSON: %v\n%s", err, record)
	}
	return m[key]
}

// modifiedFixture returns the fixture with one row changed: the Orkla
// application is marked completed with a lower final amount.
func modifiedFixture(t *testing.T) []byte {
	t.Helper()
	f, err := excelize.OpenReader(bytes.NewReader(fixture(t, "beviljade.xlsx")))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	// Row 5 is the Orkla row (header + 4 rows above it); columns F=amount, L=status.
	if err := f.SetCellValue(klimatklivet.PreferredSheet, "F5", 40000000); err != nil {
		t.Fatal(err)
	}
	if err := f.SetCellValue(klimatklivet.PreferredSheet, "L5", "Slutförd åtgärd"); err != nil {
		t.Fatal(err)
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestIngest_PersistsDatasetRowsAndEventsIdempotently(t *testing.T) {
	site, server, app := newSite(t, "beviljade.xlsx")
	repo := publicevent.NewRepository()
	ctx := context.Background()

	// First run: the file version and all six rows are new.
	stats, err := app.Klimatklivet.Run(ctx, klimatklivet.Options{})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	want := klimatklivet.Stats{
		DatasetURL: server.URL + filePath, DatasetPublishedThrough: "2026-06-30", DatasetChanged: true,
		RowsInFile: 6, RowsInWindow: 6, ObservationsInserted: 7, EventsInserted: 6,
	}
	if stats != want {
		t.Errorf("stats = %+v\nwant   %+v", stats, want)
	}
	if n := countRows(t, app, "SELECT count(*) FROM public_events WHERE source = 'klimatklivet'"); n != 6 {
		t.Errorf("public_events = %d", n)
	}

	page, err := repo.List(ctx, app.Pool, publicevent.Filter{Source: klimatklivet.Source}, 10, "")
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]publicevent.PublicEvent{}
	for _, e := range page.Items {
		byID[e.SourceEventID] = e
	}
	orkla := byID["NV-25-043146"]
	if orkla.EventType != publicevent.EventTypeClimateInvestmentGrantApproved || orkla.OccurredOn.Format("2006-01-02") != "2025-12-18" ||
		orkla.Title != "Energikonvertering industri" || orkla.OrganisationName != "Orkla Snacks Sverige AB" || orkla.OrganisationNumber != "" ||
		orkla.WorkplaceCFAR != "" || orkla.SourceURL != server.URL+klimatklivet.ResultsPagePath ||
		recordField(t, orkla.Record, "measure_category") != "Energikonvertering" || recordField(t, orkla.Record, "grant_amount_sek") != float64(45900000) ||
		recordField(t, orkla.Record, "municipality") != "Filipstad" || recordField(t, orkla.Record, "status") != "Pågående åtgärd" {
		t.Errorf("orkla event = %+v\nrecord %s", orkla, orkla.Record)
	}
	// Decision date, not ingestion time, is the event time; old rows keep old dates.
	if byID["NV-05928-15"].OccurredOn.Format("2006-01-02") != "2015-12-16" {
		t.Errorf("historical event date = %v", byID["NV-05928-15"].OccurredOn)
	}
	obs, err := repo.ListObservations(ctx, app.Pool, klimatklivet.Source, "NV-25-043146")
	if err != nil || len(obs) != 1 || obs[0].SourceURL != server.URL+filePath || !strings.Contains(obs[0].Raw, `"Ärendenummer":"NV-25-043146"`) {
		t.Fatalf("row observations = %+v, %v", obs, err)
	}
	datasets, err := repo.ListObservations(ctx, app.Pool, klimatklivet.Source, klimatklivet.DatasetRecordID)
	if err != nil || len(datasets) != 1 {
		t.Fatalf("dataset observations = %+v, %v", datasets, err)
	}
	var ds klimatklivet.Dataset
	if err := json.Unmarshal(datasets[0].Payload, &ds); err != nil {
		t.Fatal(err)
	}
	if ds.URL != server.URL+filePath || ds.ETag != `"v1"` || ds.SHA256 == "" || ds.RowCount != 6 || ds.PublishedThrough != "2026-06-30" || ds.Sheet != klimatklivet.PreferredSheet || ds.SizeBytes == 0 || ds.LastModified == "" {
		t.Errorf("dataset = %+v", ds)
	}

	// Second run: the server reports the file unchanged; nothing is
	// downloaded or processed, but the check is recorded.
	before := site.fileRequests()
	stats, err = app.Klimatklivet.Run(ctx, klimatklivet.Options{})
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if !stats.DatasetNotModified || stats.RowsInFile != 0 || stats.ObservationsInserted != 0 || stats.EventsInserted != 0 {
		t.Errorf("second run stats = %+v", stats)
	}
	if site.fileRequests() != before+1 {
		t.Errorf("expected exactly one conditional file request")
	}
	datasets, _ = repo.ListObservations(ctx, app.Pool, klimatklivet.Source, klimatklivet.DatasetRecordID)
	if len(datasets) != 1 || !datasets[0].LastObservedAt.After(datasets[0].FirstObservedAt) {
		t.Errorf("unchanged dataset should be re-observed, not re-recorded: %+v", datasets)
	}

	// Forced run over the same file: everything is already known.
	stats, err = app.Klimatklivet.Run(ctx, klimatklivet.Options{Force: true})
	if err != nil {
		t.Fatalf("forced run: %v", err)
	}
	if stats.DatasetNotModified || stats.DatasetChanged || stats.ObservationsInserted != 0 || stats.EventsInserted != 0 || stats.EventsUnchanged != 6 {
		t.Errorf("forced run stats = %+v", stats)
	}
	if n := countRows(t, app, "SELECT count(*) FROM source_observations WHERE source = 'klimatklivet'"); n != 7 {
		t.Errorf("source_observations after forced re-run = %d", n)
	}

	// A new file version with one changed row: the version and the changed
	// row get new observations, the event follows, the rest is untouched.
	site.set(modifiedFixture(t), `"v2"`)
	stats, err = app.Klimatklivet.Run(ctx, klimatklivet.Options{})
	if err != nil {
		t.Fatalf("changed run: %v", err)
	}
	if !stats.DatasetChanged || stats.ObservationsInserted != 2 || stats.EventsUpdated != 1 || stats.EventsUnchanged != 5 || stats.EventsInserted != 0 {
		t.Errorf("changed run stats = %+v", stats)
	}
	obs, _ = repo.ListObservations(ctx, app.Pool, klimatklivet.Source, "NV-25-043146")
	if len(obs) != 2 || obs[0].ContentHash == obs[1].ContentHash {
		t.Fatalf("observations after change = %+v", obs)
	}
	updated, err := repo.Get(ctx, app.Pool, orkla.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.ObservationID != obs[1].ID || recordField(t, updated.Record, "status") != "Slutförd åtgärd" || recordField(t, updated.Record, "grant_amount_sek") != float64(40000000) ||
		!updated.UpdatedAt.After(orkla.UpdatedAt) || !updated.FirstObservedAt.Equal(orkla.FirstObservedAt) || updated.OccurredOn != orkla.OccurredOn {
		t.Errorf("updated event = %+v\nrecord %s", updated, updated.Record)
	}
	if n := countRows(t, app, "SELECT count(*) FROM public_events WHERE source = 'klimatklivet'"); n != 6 {
		t.Errorf("public_events after change = %d", n)
	}
}

func TestIngest_WindowFiltersOnDecisionDate(t *testing.T) {
	_, _, app := newSite(t, "beviljade.xlsx")
	from, _ := time.Parse("2006-01-02", "2025-01-01")
	stats, err := app.Klimatklivet.Run(context.Background(), klimatklivet.Options{From: from})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if stats.RowsInFile != 6 || stats.RowsInWindow != 3 || stats.RowsOutsideWindow != 3 || stats.EventsInserted != 3 || stats.ObservationsInserted != 4 {
		t.Errorf("stats = %+v", stats)
	}
	to, _ := time.Parse("2006-01-02", "2025-12-31")
	stats, err = app.Klimatklivet.Run(context.Background(), klimatklivet.Options{From: from, To: to, Force: true})
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if stats.RowsInWindow != 2 || stats.EventsUnchanged != 2 || stats.EventsInserted != 0 {
		t.Errorf("bounded stats = %+v", stats)
	}
	if _, err := app.Klimatklivet.Run(context.Background(), klimatklivet.Options{From: to, To: from}); err == nil {
		t.Error("expected error for reversed window")
	}
}

func TestIngest_SkipsMalformedRows(t *testing.T) {
	_, _, app := newSite(t, "beviljade_malformed.xlsx")
	stats, err := app.Klimatklivet.Run(context.Background(), klimatklivet.Options{})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if stats.RowsInFile != 5 || stats.RowsMalformed != 3 || stats.RowsInWindow != 2 || stats.EventsInserted != 2 {
		t.Errorf("stats = %+v", stats)
	}
	if !strings.Contains(app.Logs(), "dataset row could not be parsed") {
		t.Errorf("expected a warning per malformed row:\n%s", app.Logs())
	}
}

func TestIngest_AbortsOnSourceFailure(t *testing.T) {
	site, _, app := newSite(t, "beviljade.xlsx")
	site.status = http.StatusInternalServerError
	stats, err := app.Klimatklivet.Run(context.Background(), klimatklivet.Options{})
	var statusErr *klimatklivet.StatusError
	if !errors.As(err, &statusErr) || statusErr.StatusCode != http.StatusInternalServerError || stats.RowsInFile != 0 {
		t.Fatalf("err = %v, stats = %+v", err, stats)
	}
	site.status = 0
	site.set([]byte("<html>not a workbook</html>"), `"v9"`)
	if _, err := app.Klimatklivet.Run(context.Background(), klimatklivet.Options{}); err == nil {
		t.Fatal("expected error for a non-workbook file")
	}
	if n := countRows(t, app, "SELECT count(*) FROM public_events"); n != 0 {
		t.Errorf("no events expected, got %d", n)
	}
}
