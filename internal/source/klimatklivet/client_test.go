package klimatklivet

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/sK4rdell/signal-engine/internal/platform/config"
)

func newTestClient(t *testing.T, baseURL string) *Client {
	t.Helper()
	c, err := NewClient(config.KlimatklivetConfig{BaseURL: baseURL}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestClient_DiscoverAndDownload(t *testing.T) {
	page := readFixture(t, "results_page.html")
	file := readFixture(t, "beviljade.xlsx")
	var lastRequest *http.Request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastRequest = r.Clone(context.Background())
		switch r.URL.Path {
		case ResultsPagePath:
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(page)
		case "/49f446/globalassets/amnen/klimatomstallning/klimatklivet/dokument/beviljade-ansokningar-till-klimatklivet-260701.xlsx":
			if r.Header.Get("If-None-Match") == `"v1"` {
				w.WriteHeader(http.StatusNotModified)
				return
			}
			w.Header().Set("ETag", `"v1"`)
			w.Header().Set("Last-Modified", "Tue, 07 Jul 2026 12:21:10 GMT")
			_, _ = w.Write(file)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	c := newTestClient(t, server.URL)
	link, err := c.DiscoverDataset(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if link.URL != server.URL+"/49f446/globalassets/amnen/klimatomstallning/klimatklivet/dokument/beviljade-ansokningar-till-klimatklivet-260701.xlsx" ||
		link.Label != "Beviljade ansökningar till Klimatklivet till och med 30 juni 2026 (xlsx)" || link.PublishedThrough != "2026-06-30" {
		t.Errorf("link = %+v", link)
	}

	dl, err := c.Download(context.Background(), link, "")
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(file)
	if dl.NotModified || dl.ETag != `"v1"` || dl.LastModified == "" || dl.SHA256 != hex.EncodeToString(sum[:]) || dl.Size != int64(len(file)) || len(dl.Body) != len(file) || dl.URL != link.URL {
		t.Errorf("download = etag %q, sha %s, size %d, notModified %v", dl.ETag, dl.SHA256, dl.Size, dl.NotModified)
	}
	if lastRequest.Header.Get("If-None-Match") != "" {
		t.Errorf("first download must not be conditional")
	}

	dl, err = c.Download(context.Background(), link, `"v1"`)
	if err != nil {
		t.Fatal(err)
	}
	if !dl.NotModified || len(dl.Body) != 0 || lastRequest.Header.Get("If-None-Match") != `"v1"` {
		t.Errorf("conditional download = %+v, header %q", dl, lastRequest.Header.Get("If-None-Match"))
	}
}

func TestClient_ResultsPageWithoutLink(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html><body><a href="/x/klimatklivet-lagesbeskrivning-2026.pdf">Lägesbeskrivning</a><a href="/x/mall.xlsx">Mall investeringskalkyl (xlsx)</a></body></html>`))
	}))
	defer server.Close()
	_, err := newTestClient(t, server.URL).DiscoverDataset(context.Background())
	if !errors.Is(err, ErrDatasetLinkNotFound) {
		t.Fatalf("err = %v, want ErrDatasetLinkNotFound", err)
	}
}

func TestClient_UnexpectedStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusBadGateway)
	}))
	defer server.Close()
	c := newTestClient(t, server.URL)
	var statusErr *StatusError
	if _, err := c.DiscoverDataset(context.Background()); !errors.As(err, &statusErr) || statusErr.StatusCode != http.StatusBadGateway {
		t.Fatalf("discover err = %v", err)
	}
	if _, err := c.Download(context.Background(), DatasetLink{URL: server.URL + "/file.xlsx"}, ""); !errors.As(err, &statusErr) || statusErr.StatusCode != http.StatusBadGateway {
		t.Fatalf("download err = %v", err)
	}
}

func TestNewClient_RejectsBadBaseURL(t *testing.T) {
	for _, bad := range []string{"", "naturvardsverket.se", "ftp://x", "https://"} {
		if _, err := NewClient(config.KlimatklivetConfig{BaseURL: bad}, nil); err == nil {
			t.Errorf("NewClient(%q) accepted", bad)
		}
	}
	c := newTestClient(t, "https://www.naturvardsverket.se")
	if c.ResultsPageURL() != "https://www.naturvardsverket.se"+ResultsPagePath {
		t.Errorf("results page url = %q", c.ResultsPageURL())
	}
}
