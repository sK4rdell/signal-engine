package arbetsmiljoverket

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/sK4rdell/signal-engine/internal/platform/config"
)

func newTestClient(t *testing.T, baseURL string, interval time.Duration) *Client {
	t.Helper()
	c, err := NewClient(config.ArbetsmiljoverketConfig{BaseURL: baseURL, RequestInterval: interval}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func date(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.Parse(dateLayout, s)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestClient_SearchBuildsQueryAndParses(t *testing.T) {
	fixture, err := os.ReadFile("testdata/search_page.html")
	if err != nil {
		t.Fatal(err)
	}
	var gotRequest *http.Request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRequest = r.Clone(context.Background())
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(fixture)
	}))
	defer server.Close()

	c := newTestClient(t, server.URL, 0)
	q := SearchQuery{From: date(t, "2026-09-15"), To: date(t, "2026-09-23"), SubjectArea: SubjectAreaInspection, DocumentType: DocumentTypeInspectionNotice, Page: 2}
	page, err := c.Search(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	if gotRequest.URL.Path != SearchPath {
		t.Errorf("path = %q", gotRequest.URL.Path)
	}
	for key, want := range map[string]string{
		"FromDate":              "2026-09-15",
		"ToDate":                "2026-09-23",
		"SelectedArendeProcess": "6.1",
		"SelectedHandlingType":  "6.1-23",
		"SelectedSortOrder":     "Dokumentdatum|Desc",
		"OnlyActive":            "false",
		"p":                     "2",
	} {
		if got := gotRequest.URL.Query().Get(key); got != want {
			t.Errorf("query %s = %q, want %q", key, got, want)
		}
	}
	if ua := gotRequest.Header.Get("User-Agent"); !strings.HasPrefix(ua, "signal-engine-ingest/") {
		t.Errorf("user agent = %q", ua)
	}
	if page.URL != c.SearchURL(q) || len(page.Documents) != 3 || page.Total != 347 {
		t.Errorf("page = url %q, %d documents, total %d", page.URL, len(page.Documents), page.Total)
	}
	// Case links are resolved against the configured base, not hard-coded.
	if !strings.HasPrefix(page.Documents[0].CaseURL, server.URL+"/") {
		t.Errorf("case url = %q", page.Documents[0].CaseURL)
	}
}

func TestClient_PageDefaultsToOne(t *testing.T) {
	c := newTestClient(t, "https://www.av.se", 0)
	u := c.SearchURL(SearchQuery{From: date(t, "2026-09-22"), To: date(t, "2026-09-22")})
	if !strings.Contains(u, "p=1") || !strings.HasPrefix(u, "https://www.av.se"+SearchPath+"?") {
		t.Errorf("url = %q", u)
	}
}

func TestClient_UnexpectedStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	_, err := newTestClient(t, server.URL, 0).Search(context.Background(), SearchQuery{From: date(t, "2026-09-22"), To: date(t, "2026-09-22")})
	var statusErr *StatusError
	if !errors.As(err, &statusErr) || statusErr.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("err = %v, want *StatusError 503", err)
	}
}

func TestClient_TransportFailure(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	url := server.URL
	server.Close()

	_, err := newTestClient(t, url, 0).Search(context.Background(), SearchQuery{From: date(t, "2026-09-22"), To: date(t, "2026-09-22")})
	var statusErr *StatusError
	if err == nil || errors.As(err, &statusErr) {
		t.Fatalf("err = %v, want a transport error", err)
	}
}

func TestClient_UnexpectedMarkupIsAnError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<html><body>maintenance</body></html>"))
	}))
	defer server.Close()

	_, err := newTestClient(t, server.URL, 0).Search(context.Background(), SearchQuery{From: date(t, "2026-09-22"), To: date(t, "2026-09-22")})
	if !errors.Is(err, ErrUnexpectedMarkup) {
		t.Fatalf("err = %v, want ErrUnexpectedMarkup", err)
	}
}

func TestClient_KeepsRequestInterval(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html><body><ul data-dd-search-result></ul></body></html>`))
	}))
	defer server.Close()

	const interval = 60 * time.Millisecond
	c := newTestClient(t, server.URL, interval)
	q := SearchQuery{From: date(t, "2026-09-22"), To: date(t, "2026-09-22")}
	start := time.Now()
	for i := 0; i < 3; i++ {
		if _, err := c.Search(context.Background(), q); err != nil {
			t.Fatal(err)
		}
	}
	if elapsed := time.Since(start); elapsed < 2*interval {
		t.Errorf("three requests took %v, want at least %v", elapsed, 2*interval)
	}

	// Cancellation interrupts the wait.
	c = newTestClient(t, server.URL, time.Hour)
	if _, err := c.Search(context.Background(), q); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.Search(ctx, q); !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

func TestNewClient_RejectsBadBaseURL(t *testing.T) {
	for _, bad := range []string{"", "av.se", "ftp://av.se", "https://"} {
		if _, err := NewClient(config.ArbetsmiljoverketConfig{BaseURL: bad}, nil); err == nil {
			t.Errorf("NewClient(%q) accepted", bad)
		}
	}
}
