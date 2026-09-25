package stockholm

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// Errors that mean the source changed shape rather than that a request had
// no results.
var (
	ErrUnexpectedMarkup = errors.New("stockholm: expected content not found in page")
	ErrIncompleteCase   = errors.New("stockholm: case page is incomplete")
)

// Labels of the case page's definition list, entities decoded.
const (
	labelDiaryNumber = "Diarienummer"
	labelCaseGroup   = "Ärendegrupp"
	labelClassCode   = "Diarieplansbeteckning"
	labelProperty    = "Fastighetsbeteckning"
	labelAddress     = "Adress"
	labelDistrict    = "Stadsdel"
	labelStartedOn   = "Ärendestart"
	labelClosedOn    = "Ärendeavslut"
	labelOfficer     = "Handläggare"
	labelTitle       = "Ärendemening"
)

// addressMissing is what the source prints as address when it has none.
const addressMissing = `Information saknas för "Adress"`

const (
	dateLayout      = "2006-01-02"
	timestampLayout = "2006-01-02T15:04:05"
)

var (
	searchModelMarker = []byte("var CaseSearchResultsViewModel = ")
	documentsMarker   = regexp.MustCompile(`createInstance\(\s*'documentList'\s*,\s*'[^']*'\s*,\s*'[^']*'\s*,`)
	documentCountRx   = regexp.MustCompile(`"id":"documentList"[^']*?"headingSubtext":"\((\d+) st\)"`)
	recNoInPageRx     = regexp.MustCompile(`/arende/arende/(\d+)`)
	digits            = regexp.MustCompile(`^\d+$`)
)

// searchViewModel mirrors the parts of the embedded result JSON we read.
type searchViewModel struct {
	BuildCases      searchList `json:"BuildCases"`
	RealEstateCases searchList `json:"RealEstateCases"`
	OtherCases      searchList `json:"OtherCases"`
}

type searchList struct {
	CaseSearchDetails []searchDetail `json:"CaseSearchDetails"`
}

type searchDetail struct {
	RecNo             string `json:"RecNo"`
	CaseTypeCode      string `json:"CaseTypeCode"`
	Description       string `json:"Description"`
	StartDate         string `json:"StartDate"`
	RealEstateName    string `json:"RealEstateName"`
	RealEstateAddress string `json:"RealEstateAddress"`
	Name              string `json:"Name"`
	IsEarchive        bool   `json:"IsEarchive"`
}

// ParseSearchPage reads the case list embedded in a search result page.
// Every list the page carries is returned; the caller filters on class
// code through the query, so the "Övriga ärenden" list is the one that
// matters. A page without the embedded model fails with
// ErrUnexpectedMarkup.
func ParseSearchPage(r io.Reader) ([]CaseSummary, error) {
	body, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("stockholm: read search page: %w", err)
	}
	i := bytes.Index(body, searchModelMarker)
	if i < 0 {
		return nil, ErrUnexpectedMarkup
	}
	var model searchViewModel
	dec := json.NewDecoder(bytes.NewReader(body[i+len(searchModelMarker):]))
	if err := dec.Decode(&model); err != nil {
		return nil, fmt.Errorf("stockholm: decode embedded search result: %w", err)
	}
	var out []CaseSummary
	for _, list := range []searchList{model.BuildCases, model.RealEstateCases, model.OtherCases} {
		for _, d := range list.CaseSearchDetails {
			c, err := d.summary()
			if err != nil {
				return nil, err
			}
			out = append(out, c)
		}
	}
	return out, nil
}

func (d searchDetail) summary() (CaseSummary, error) {
	if !digits.MatchString(d.RecNo) {
		return CaseSummary{}, fmt.Errorf("stockholm: search row has no numeric RecNo (%q)", d.RecNo)
	}
	started := d.StartDate
	if len(started) >= len(dateLayout) {
		started = started[:len(dateLayout)]
	}
	if _, err := time.Parse(dateLayout, started); err != nil {
		return CaseSummary{}, fmt.Errorf("stockholm: search row %s has a malformed start date %q", d.RecNo, d.StartDate)
	}
	return CaseSummary{
		RecNo:               d.RecNo,
		DiaryNumber:         collapse(d.Name),
		Title:               collapse(d.Description),
		StartedOn:           started,
		PropertyDesignation: collapse(d.RealEstateName),
		Address:             cleanAddress(d.RealEstateAddress),
		CaseGroup:           collapse(d.CaseTypeCode),
		Archived:            d.IsEarchive,
	}, nil
}

// documentRow mirrors one entry of the embedded document list.
type documentRow struct {
	Title    string  `json:"title"`
	Category string  `json:"category"`
	Date     string  `json:"date"`
	FileName *string `json:"fileName"`
}

// ParseCasePage parses a case page: the definition list with the case's
// metadata and the document list embedded as JSON. base resolves the case
// URL. The page's document count must match the parsed list, so a truncated
// or changed page fails (ErrIncompleteCase) rather than yielding a partial
// state.
func ParseCasePage(r io.Reader, base *url.URL) (Case, error) {
	body, err := io.ReadAll(r)
	if err != nil {
		return Case{}, fmt.Errorf("stockholm: read case page: %w", err)
	}
	root, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return Case{}, fmt.Errorf("stockholm: parse html: %w", err)
	}
	fields := definitionList(root)
	c := Case{RawHTML: string(body)}
	c.DiaryNumber = text(fields[labelDiaryNumber])
	c.CaseGroup = text(fields[labelCaseGroup])
	c.ClassCode = text(fields[labelClassCode])
	c.PropertyDesignation = text(fields[labelProperty])
	c.Address = cleanAddress(text(fields[labelAddress]))
	c.District = text(fields[labelDistrict])
	c.StartedOn = text(fields[labelStartedOn])
	c.ClosedOn = text(fields[labelClosedOn])
	c.Officer = text(fields[labelOfficer])
	c.Title = text(fields[labelTitle])
	if c.DiaryNumber == "" || c.Title == "" || c.PropertyDesignation == "" {
		return Case{}, fmt.Errorf("%w: diary number, title or property missing", ErrUnexpectedMarkup)
	}
	if _, err := time.Parse(dateLayout, c.StartedOn); err != nil {
		return Case{}, fmt.Errorf("stockholm: case %s has a malformed start date %q", c.DiaryNumber, c.StartedOn)
	}
	if c.ClosedOn != "" {
		if _, err := time.Parse(dateLayout, c.ClosedOn); err != nil {
			return Case{}, fmt.Errorf("stockholm: case %s has a malformed closure date %q", c.DiaryNumber, c.ClosedOn)
		}
	}

	// The page links to its own logged-in variant with the RecNo.
	if m := recNoInPageRx.FindSubmatch(body); m != nil {
		c.RecNo = string(m[1])
	}

	// Document list: the JSON array passed to the list component.
	loc := documentsMarker.FindIndex(body)
	if loc == nil {
		return Case{}, fmt.Errorf("%w: document list not found", ErrUnexpectedMarkup)
	}
	rest := body[loc[1]:]
	start := bytes.IndexByte(rest, '[')
	if start < 0 {
		return Case{}, fmt.Errorf("%w: document list not found", ErrUnexpectedMarkup)
	}
	var rows []documentRow
	if err := json.NewDecoder(bytes.NewReader(rest[start:])).Decode(&rows); err != nil {
		return Case{}, fmt.Errorf("stockholm: decode document list of case %s: %w", c.DiaryNumber, err)
	}
	for _, row := range rows {
		d := Document{Description: collapse(html.UnescapeString(row.Title)), Category: collapse(row.Category), Timestamp: strings.TrimSpace(row.Date), FileName: row.FileName}
		if d.Description == "" || d.Category == "" {
			return Case{}, fmt.Errorf("stockholm: case %s has a document without description or category", c.DiaryNumber)
		}
		if _, err := time.Parse(timestampLayout, d.Timestamp); err != nil {
			return Case{}, fmt.Errorf("stockholm: case %s has a document with a malformed timestamp %q", c.DiaryNumber, d.Timestamp)
		}
		c.Documents = append(c.Documents, d)
	}
	// The list component prints the count as its subtext, "(N st)".
	m := documentCountRx.FindSubmatch(body)
	if m == nil {
		return Case{}, fmt.Errorf("%w: document count not found", ErrUnexpectedMarkup)
	}
	c.DocumentCount, _ = strconv.Atoi(string(m[1]))
	if c.DocumentCount != len(c.Documents) {
		return Case{}, fmt.Errorf("%w: case %s lists %d documents but the page says %d", ErrIncompleteCase, c.DiaryNumber, len(c.Documents), c.DocumentCount)
	}
	if base != nil && c.RecNo != "" {
		u := *base
		u.Path = CasePath + c.RecNo
		u.RawQuery = "dataSource=Active"
		c.CaseURL = u.String()
	}
	return c, nil
}

// cleanAddress trims the address and drops the source's placeholder for a
// missing one.
func cleanAddress(s string) string {
	s = collapse(s)
	if s == "" || s == addressMissing || strings.HasPrefix(s, "Information saknas") {
		return ""
	}
	return s
}

// collapse trims and collapses whitespace.
func collapse(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// definitionList maps every <dt> label to the <dd> that follows it.
func definitionList(n *html.Node) map[string]*html.Node {
	fields := map[string]*html.Node{}
	for _, dt := range findAll(n, func(n *html.Node) bool { return n.Data == "dt" }) {
		label := strings.TrimSuffix(text(dt), ":")
		if label == "" {
			continue
		}
		for s := dt.NextSibling; s != nil; s = s.NextSibling {
			if s.Type != html.ElementNode {
				continue
			}
			if s.Data == "dd" {
				if _, exists := fields[label]; !exists {
					fields[label] = s
				}
			}
			break
		}
	}
	return fields
}

// --- small html helpers -----------------------------------------------------

func findAll(n *html.Node, match func(*html.Node) bool) []*html.Node {
	var out []*html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && match(n) {
			out = append(out, n)
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walk(c)
	}
	return out
}

// text returns the whitespace-collapsed text content of n ("" for nil).
func text(n *html.Node) string {
	if n == nil {
		return ""
	}
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
	return collapse(b.String())
}
