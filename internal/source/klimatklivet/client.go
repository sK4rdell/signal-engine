package klimatklivet

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"

	"github.com/sK4rdell/signal-engine/internal/platform/config"
)

// The published file is ~7 MB; anything far larger is not the file we expect.
const maxDownloadBytes = 64 << 20

const userAgent = "signal-engine-ingest/0.1 (+https://github.com/sK4rdell/signal-engine)"

// Client locates and downloads the dataset file.
type Client struct {
	baseURL *url.URL
	http    *http.Client
}

// NewClient builds a client from configuration. httpClient may be nil.
func NewClient(cfg config.KlimatklivetConfig, httpClient *http.Client) (*Client, error) {
	base, err := url.Parse(cfg.BaseURL)
	if err != nil || (base.Scheme != "http" && base.Scheme != "https") || base.Host == "" {
		return nil, fmt.Errorf("klimatklivet: base URL %q is not an http(s) origin", cfg.BaseURL)
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 2 * time.Minute}
	}
	return &Client{baseURL: base, http: httpClient}, nil
}

// ResultsPageURL is the stable page the dataset is linked from; events
// point at it because individual decisions have no URL of their own.
func (c *Client) ResultsPageURL() string {
	u := *c.baseURL
	u.Path = ResultsPagePath
	return u.String()
}

// DatasetLink is the current dataset file as linked from the results page.
type DatasetLink struct {
	URL   string
	Label string
	// PublishedThrough is the cut-off date named in the link text
	// ("till och med 30 juni 2026"), "" when it cannot be read.
	PublishedThrough string
}

// ErrDatasetLinkNotFound is returned when the results page no longer links
// an approved-applications workbook.
var ErrDatasetLinkNotFound = errors.New("klimatklivet: approved applications workbook link not found on results page")

// StatusError is an unexpected HTTP status from the source.
type StatusError struct {
	URL        string
	StatusCode int
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("klimatklivet: unexpected status %d from %s", e.StatusCode, e.URL)
}

// DiscoverDataset fetches the results page and returns the link to the
// approved-applications workbook.
func (c *Client) DiscoverDataset(ctx context.Context) (DatasetLink, error) {
	target := c.ResultsPageURL()
	res, err := c.get(ctx, target, "")
	if err != nil {
		return DatasetLink{}, err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return DatasetLink{}, &StatusError{URL: target, StatusCode: res.StatusCode}
	}
	root, err := html.Parse(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return DatasetLink{}, fmt.Errorf("klimatklivet: parse results page: %w", err)
	}
	link, ok := findDatasetLink(root, c.baseURL)
	if !ok {
		return DatasetLink{}, ErrDatasetLinkNotFound
	}
	return link, nil
}

// findDatasetLink picks the first .xlsx link whose text mentions approved
// applications. Exported for the parser tests through DiscoverDataset only.
func findDatasetLink(root *html.Node, base *url.URL) (DatasetLink, bool) {
	var found DatasetLink
	ok := false
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if ok {
			return
		}
		if n.Type == html.ElementNode && n.Data == "a" {
			href := attr(n, "href")
			label := strings.Join(strings.Fields(text(n)), " ")
			if strings.HasSuffix(strings.ToLower(href), ".xlsx") && strings.Contains(strings.ToLower(label), "beviljade ansökningar") {
				if u, err := url.Parse(href); err == nil {
					found = DatasetLink{URL: base.ResolveReference(u).String(), Label: label, PublishedThrough: publishedThrough(label)}
					ok = true
					return
				}
			}
		}
		for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
			walk(ch)
		}
	}
	walk(root)
	return found, ok
}

var swedishMonths = map[string]time.Month{
	"januari": 1, "februari": 2, "mars": 3, "april": 4, "maj": 5, "juni": 6,
	"juli": 7, "augusti": 8, "september": 9, "oktober": 10, "november": 11, "december": 12,
}

var publishedThroughPattern = regexp.MustCompile(`(?i)till och med (\d{1,2}) (\p{L}+) (\d{4})`)

// publishedThrough reads "till och med 30 juni 2026" out of a link label.
func publishedThrough(label string) string {
	m := publishedThroughPattern.FindStringSubmatch(label)
	if m == nil {
		return ""
	}
	month, ok := swedishMonths[strings.ToLower(m[2])]
	if !ok {
		return ""
	}
	day, err := strconv.Atoi(m[1])
	if err != nil {
		return ""
	}
	year, err := strconv.Atoi(m[3])
	if err != nil {
		return ""
	}
	t := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	if t.Day() != day {
		return ""
	}
	return t.Format(dateLayout)
}

// Download is one fetched dataset file, or the fact that it was unchanged.
type Download struct {
	URL          string
	ETag         string
	LastModified string
	SHA256       string
	Size         int64
	Body         []byte
	// NotModified is set when the server answered 304 to a conditional
	// request; Body is then empty.
	NotModified bool
}

// Download fetches the workbook. When etag is non-empty the request is
// conditional and an unchanged file is reported as NotModified without
// transferring it.
func (c *Client) Download(ctx context.Context, link DatasetLink, etag string) (Download, error) {
	res, err := c.get(ctx, link.URL, etag)
	if err != nil {
		return Download{}, err
	}
	defer func() { _ = res.Body.Close() }()

	switch res.StatusCode {
	case http.StatusNotModified:
		return Download{URL: link.URL, ETag: etag, NotModified: true}, nil
	case http.StatusOK:
	default:
		return Download{}, &StatusError{URL: link.URL, StatusCode: res.StatusCode}
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, maxDownloadBytes+1))
	if err != nil {
		return Download{}, fmt.Errorf("klimatklivet: read %s: %w", link.URL, err)
	}
	if len(body) > maxDownloadBytes {
		return Download{}, fmt.Errorf("klimatklivet: %s exceeds %d bytes", link.URL, maxDownloadBytes)
	}
	sum := sha256.Sum256(body)
	return Download{
		URL:          link.URL,
		ETag:         res.Header.Get("ETag"),
		LastModified: res.Header.Get("Last-Modified"),
		SHA256:       hex.EncodeToString(sum[:]),
		Size:         int64(len(body)),
		Body:         body,
	}, nil
}

func (c *Client) get(ctx context.Context, target, etag string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, fmt.Errorf("klimatklivet: build request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	res, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("klimatklivet: fetch %s: %w", target, err)
	}
	return res, nil
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func text(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
			b.WriteByte(' ')
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}
