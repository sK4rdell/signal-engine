package arbetsmiljoverket

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/sK4rdell/signal-engine/internal/platform/config"
)

// Response bodies are ~310 KB; anything far larger is not the page we expect.
const maxResponseBytes = 8 << 20

const userAgent = "signal-engine-ingest/0.1 (+https://github.com/sK4rdell/signal-engine)"

// Client fetches and parses diary search pages. It serialises requests and
// keeps a minimum interval between them.
type Client struct {
	baseURL  *url.URL
	http     *http.Client
	interval time.Duration

	mu   sync.Mutex
	last time.Time
}

// NewClient builds a client from configuration. httpClient may be nil.
func NewClient(cfg config.ArbetsmiljoverketConfig, httpClient *http.Client) (*Client, error) {
	base, err := url.Parse(cfg.BaseURL)
	if err != nil || (base.Scheme != "http" && base.Scheme != "https") || base.Host == "" {
		return nil, fmt.Errorf("arbetsmiljoverket: base URL %q is not an http(s) origin", cfg.BaseURL)
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{baseURL: base, http: httpClient, interval: cfg.RequestInterval}, nil
}

// SearchQuery is a filtered, paginated diary search. From and To are
// inclusive civil dates on "handlingens datum".
type SearchQuery struct {
	From         time.Time
	To           time.Time
	SubjectArea  string
	DocumentType string
	// Page is 1-based; 0 means the first page.
	Page int
}

// SearchURL builds the request URL for q. Results are sorted newest first,
// which the ingester relies on for its stable identifiers only, never for
// termination.
func (c *Client) SearchURL(q SearchQuery) string {
	values := url.Values{}
	values.Set("FromDate", q.From.Format(dateLayout))
	values.Set("ToDate", q.To.Format(dateLayout))
	values.Set("SelectedArendeProcess", q.SubjectArea)
	values.Set("SelectedHandlingType", q.DocumentType)
	values.Set("SelectedSortOrder", "Dokumentdatum|Desc")
	values.Set("OnlyActive", "false")
	page := q.Page
	if page < 1 {
		page = 1
	}
	values.Set("p", fmt.Sprint(page))
	u := *c.baseURL
	u.Path = SearchPath
	u.RawQuery = values.Encode()
	return u.String()
}

// StatusError is an unexpected HTTP status from the source.
type StatusError struct {
	URL        string
	StatusCode int
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("arbetsmiljoverket: unexpected status %d from %s", e.StatusCode, e.URL)
}

// Search fetches one page of results. Transport failures and unexpected
// statuses are returned as errors (the latter as *StatusError); parse
// failures of individual rows are reported inside the page.
func (c *Client) Search(ctx context.Context, q SearchQuery) (SearchPage, error) {
	if err := c.wait(ctx); err != nil {
		return SearchPage{}, err
	}
	target := c.SearchURL(q)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return SearchPage{}, fmt.Errorf("arbetsmiljoverket: build request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html")

	res, err := c.http.Do(req)
	if err != nil {
		return SearchPage{}, fmt.Errorf("arbetsmiljoverket: fetch %s: %w", target, err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return SearchPage{}, &StatusError{URL: target, StatusCode: res.StatusCode}
	}

	page, err := ParseSearchPage(io.LimitReader(res.Body, maxResponseBytes), c.baseURL)
	if err != nil {
		return SearchPage{}, fmt.Errorf("arbetsmiljoverket: %s: %w", target, err)
	}
	page.URL = target
	return page, nil
}

// wait enforces the minimum interval between requests.
func (c *Client) wait(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.last.IsZero() && c.interval > 0 {
		if d := c.interval - time.Since(c.last); d > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(d):
			}
		}
	}
	c.last = time.Now()
	return nil
}
