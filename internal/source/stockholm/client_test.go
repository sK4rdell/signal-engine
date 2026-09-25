package stockholm

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/sK4rdell/signal-engine/internal/platform/config"
)

func newTestClient(t *testing.T, baseURL string, interval time.Duration) *Client {
	t.Helper()
	c, err := NewClient(config.StockholmConfig{BaseURL: baseURL, RequestInterval: interval, RefreshInterval: time.Hour}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestClient_URLs(t *testing.T) {
	c := newTestClient(t, "https://etjanster.stockholm.se", 0)
	from, _ := time.Parse(dateLayout, "2026-09-01")
	to, _ := time.Parse(dateLayout, "2026-09-25")
	u, err := url.Parse(c.SearchURL(Window{From: from, To: to}))
	if err != nil {
		t.Fatal(err)
	}
	if u.Path != SearchPath {
		t.Errorf("path = %q", u.Path)
	}
	for key, want := range map[string]string{"CaseTypeRecNo": "200001", "JournalPlanCode": "7.2", "CaseStartDateFrom": "2026-09-01", "CaseStartDateTo": "2026-09-25"} {
		if got := u.Query().Get(key); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
	if got := c.CaseURL("1327421", false); got != "https://etjanster.stockholm.se/Byggochplantjansten/arende/arende/1327421?dataSource=Active" {
		t.Errorf("case url = %q", got)
	}
	if got := c.CaseURL("1327421", true); !strings.HasSuffix(got, "dataSource=Archived") {
		t.Errorf("archived url = %q", got)
	}
	if _, err := NewClient(config.StockholmConfig{BaseURL: "etjanster.stockholm.se"}, nil); err == nil {
		t.Error("base url without scheme must be rejected")
	}
}

func TestClient_FetchAndFailures(t *testing.T) {
	search, _ := os.ReadFile("testdata/search_page.html")
	casePage, _ := os.ReadFile("testdata/case_1327421_failed.html")
	var status int
	var huge bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if status != 0 {
			http.Error(w, "boom", status)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		switch {
		case huge:
			_, _ = w.Write([]byte(strings.Repeat("x", maxCaseBytes+1)))
		case r.URL.Path == SearchPath:
			_, _ = w.Write(search)
		case strings.HasPrefix(r.URL.Path, CasePath+"1327421"):
			_, _ = w.Write(casePage)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	c := newTestClient(t, server.URL, 0)
	ctx := context.Background()
	from, _ := time.Parse(dateLayout, "2026-09-23")

	cases, _, err := c.SearchCases(ctx, Window{From: from, To: from})
	if err != nil || len(cases) != 8 {
		t.Fatalf("search = %d, %v", len(cases), err)
	}
	cs, err := c.FetchCase(ctx, "1327421", false)
	if err != nil || cs.RecNo != "1327421" || cs.CaseURL != server.URL+"/Byggochplantjansten/arende/arende/1327421?dataSource=Active" || cs.Archived {
		t.Fatalf("case = %+v, %v", cs, err)
	}
	if _, err := c.FetchCase(ctx, "999", false); err == nil {
		t.Error("404 must fail")
	} else {
		var se *StatusError
		if !errors.As(err, &se) || se.StatusCode != http.StatusNotFound {
			t.Errorf("err = %v", err)
		}
	}
	if _, err := c.FetchCase(ctx, "12a", false); err == nil {
		t.Error("non-numeric recno must fail")
	}
	status = http.StatusInternalServerError
	if _, _, err := c.SearchCases(ctx, Window{From: from, To: from}); err == nil {
		t.Error("500 must fail")
	}
	status = 0
	huge = true
	if _, err := c.FetchCase(ctx, "1327421", false); err == nil {
		t.Error("oversized response must fail to parse")
	}
	huge = false
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := c.FetchCase(cancelled, "1327421", false); err == nil {
		t.Error("cancelled context must fail")
	}
	if _, _, err := c.SearchCases(ctx, Window{From: from.AddDate(0, 0, 1), To: from}); err == nil {
		t.Error("reversed window must fail")
	}
}

func TestClient_PacesRequests(t *testing.T) {
	casePage, _ := os.ReadFile("testdata/case_1327421_failed.html")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(casePage) }))
	defer server.Close()
	c := newTestClient(t, server.URL, 40*time.Millisecond)
	start := time.Now()
	for i := 0; i < 3; i++ {
		if _, err := c.FetchCase(context.Background(), "1327421", false); err != nil {
			t.Fatal(err)
		}
	}
	if elapsed := time.Since(start); elapsed < 80*time.Millisecond {
		t.Errorf("three requests took %v, want at least two intervals", elapsed)
	}
}
