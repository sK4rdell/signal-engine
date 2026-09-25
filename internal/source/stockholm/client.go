package stockholm

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

// A year of search results is about 3 MB; a case page about 25 KB.
const (
	maxSearchBytes = 32 << 20
	maxCaseBytes   = 4 << 20
)

const userAgent = "signal-engine-ingest/0.1 (+https://github.com/sK4rdell/signal-engine)"

// Client fetches and parses search and case pages. It serialises requests
// and keeps a minimum interval between them.
type Client struct {
	baseURL  *url.URL
	http     *http.Client
	interval time.Duration

	mu   sync.Mutex
	last time.Time
}

// NewClient builds a client from configuration. httpClient may be nil.
func NewClient(cfg config.StockholmConfig, httpClient *http.Client) (*Client, error) {
	base, err := url.Parse(cfg.BaseURL)
	if err != nil || (base.Scheme != "http" && base.Scheme != "https") || base.Host == "" {
		return nil, fmt.Errorf("stockholm: base URL %q is not an http(s) origin", cfg.BaseURL)
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 60 * time.Second}
	}
	return &Client{baseURL: base, http: httpClient, interval: cfg.RequestInterval}, nil
}

// Window is an inclusive range of case start dates (civil dates).
type Window struct {
	From time.Time
	To   time.Time
}

// Validate checks the window is well formed.
func (w Window) Validate() error {
	if w.From.IsZero() || w.To.IsZero() {
		return fmt.Errorf("stockholm: window needs both from and to")
	}
	if w.To.Before(w.From) {
		return fmt.Errorf("stockholm: window end is before its start")
	}
	return nil
}

// SearchURL builds the case search for motorised-equipment supervision
// cases started within w. The response embeds every matching case; the
// source paginates client-side.
func (c *Client) SearchURL(w Window) string {
	values := url.Values{}
	values.Set("CaseTypeRecNo", CaseTypeFunctionalControl)
	values.Set("JournalPlanCode", ClassCodeMotorisedEquipment)
	values.Set("CaseStartDateFrom", w.From.Format(dateLayout))
	values.Set("CaseStartDateTo", w.To.Format(dateLayout))
	u := *c.baseURL
	u.Path = SearchPath
	u.RawQuery = values.Encode()
	return u.String()
}

// CaseURL builds the case page URL for recNo.
func (c *Client) CaseURL(recNo string, archived bool) string {
	u := *c.baseURL
	u.Path = CasePath + recNo
	if archived {
		u.RawQuery = "dataSource=Archived"
	} else {
		u.RawQuery = "dataSource=Active"
	}
	return u.String()
}

// StatusError is an unexpected HTTP status from the source.
type StatusError struct {
	URL        string
	StatusCode int
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("stockholm: unexpected status %d from %s", e.StatusCode, e.URL)
}

// SearchCases lists the supervision cases started within w.
func (c *Client) SearchCases(ctx context.Context, w Window) ([]CaseSummary, string, error) {
	if err := w.Validate(); err != nil {
		return nil, "", err
	}
	target := c.SearchURL(w)
	body, err := c.get(ctx, target, maxSearchBytes)
	if err != nil {
		return nil, target, err
	}
	defer func() { _ = body.Close() }()
	cases, err := ParseSearchPage(body)
	if err != nil {
		return nil, target, fmt.Errorf("stockholm: %s: %w", target, err)
	}
	return cases, target, nil
}

// FetchCase reads one case page.
func (c *Client) FetchCase(ctx context.Context, recNo string, archived bool) (Case, error) {
	if !digits.MatchString(recNo) {
		return Case{}, fmt.Errorf("stockholm: case RecNo %q is not numeric", recNo)
	}
	target := c.CaseURL(recNo, archived)
	body, err := c.get(ctx, target, maxCaseBytes)
	if err != nil {
		return Case{}, err
	}
	defer func() { _ = body.Close() }()
	cs, err := ParseCasePage(body, c.baseURL)
	if err != nil {
		return Case{}, fmt.Errorf("stockholm: %s: %w", target, err)
	}
	if cs.RecNo == "" {
		cs.RecNo = recNo
	} else if cs.RecNo != recNo {
		return Case{}, fmt.Errorf("stockholm: %s: page identifies itself as case %s", target, cs.RecNo)
	}
	cs.Archived = archived
	cs.CaseURL = target
	return cs, nil
}

// get performs a paced GET and returns the bounded body.
func (c *Client) get(ctx context.Context, target string, limit int64) (io.ReadCloser, error) {
	if err := c.wait(ctx); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, fmt.Errorf("stockholm: build request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html")
	res, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("stockholm: fetch %s: %w", target, err)
	}
	if res.StatusCode != http.StatusOK {
		_ = res.Body.Close()
		return nil, &StatusError{URL: target, StatusCode: res.StatusCode}
	}
	return &limitedBody{Reader: io.LimitReader(res.Body, limit), closer: res.Body}, nil
}

type limitedBody struct {
	io.Reader
	closer io.Closer
}

func (b *limitedBody) Close() error { return b.closer.Close() }

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
