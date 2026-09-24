package arbetsmiljoverket

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// ErrUnexpectedMarkup is returned when the page lacks the result list. It
// signals that the source changed, not that the query had no results.
var ErrUnexpectedMarkup = errors.New("arbetsmiljoverket: result list not found in page")

// Labels of the definition-list entries in a result row, as rendered by the
// source (entities decoded, trailing colon removed).
const (
	labelDocumentNumber = "Handlingsnummer"
	labelDocumentDate   = "Handlingens datum"
	labelCase           = "Ärende"
	labelCaseStatus     = "Ärendets status"
	labelOrganisation   = "Företag/organisation"
	labelOrgNumber      = "Organisationsnummer"
	labelOrigin         = "Handlingens ursprung"
	labelSubjectArea    = "Ämnesområde"
	labelWorkplace      = "Arbetsställe"
	labelCFAR           = "Arbetsställenummer (CFAR)"
)

var (
	documentNumberPattern = regexp.MustCompile(`^\d{4}/\d{6}-\d{1,3}$`)
	nonDigits             = regexp.MustCompile(`[^0-9]`)
)

// dateLayout is the published date format.
const dateLayout = "2006-01-02"

// ParseSearchPage parses a search result page. Rows that cannot be parsed
// are reported in RowErrors without failing the page; a page without the
// result list fails with ErrUnexpectedMarkup. base resolves the relative
// case links.
func ParseSearchPage(r io.Reader, base *url.URL) (SearchPage, error) {
	root, err := html.Parse(r)
	if err != nil {
		return SearchPage{}, fmt.Errorf("arbetsmiljoverket: parse html: %w", err)
	}
	list := find(root, func(n *html.Node) bool {
		return n.Data == "ul" && hasAttr(n, "data-dd-search-result")
	})
	if list == nil {
		return SearchPage{}, ErrUnexpectedMarkup
	}

	var page SearchPage
	index := 0
	for _, li := range findAll(list, func(n *html.Node) bool {
		return n.Data == "li" && hasClass(n, "document-list__item")
	}) {
		doc, err := parseRow(li, base)
		if err != nil {
			page.RowErrors = append(page.RowErrors, RowError{Index: index, Err: err, RawHTML: render(li)})
		} else {
			page.Documents = append(page.Documents, doc)
		}
		index++
	}

	if total := find(root, func(n *html.Node) bool { return n.Data == "span" && attr(n, "id") == "dd-pagination-result-total" }); total != nil {
		digits := nonDigits.ReplaceAllString(text(total), "")
		for _, r := range digits {
			page.Total = page.Total*10 + int(r-'0')
		}
	}
	return page, nil
}

// parseRow extracts one document from a result row. The row is a set of
// <dt>label</dt><dd>value</dd> pairs plus a heading with the document type.
func parseRow(li *html.Node, base *url.URL) (Document, error) {
	fields := definitionList(li)
	doc := Document{RawHTML: render(li)}

	if h2 := find(li, func(n *html.Node) bool { return n.Data == "h2" }); h2 != nil {
		for _, span := range findAll(h2, func(n *html.Node) bool { return n.Data == "span" && !hasClass(n, "visually-hidden") }) {
			doc.DocumentType = text(span)
			break
		}
	}
	if doc.DocumentType == "" {
		return Document{}, errors.New("document type missing")
	}

	doc.DocumentNumber = text(fields[labelDocumentNumber])
	if !documentNumberPattern.MatchString(doc.DocumentNumber) {
		return Document{}, fmt.Errorf("document number %q is missing or malformed", doc.DocumentNumber)
	}

	if dd := fields[labelDocumentDate]; dd != nil {
		if t := find(dd, func(n *html.Node) bool { return n.Data == "time" }); t != nil && attr(t, "datetime") != "" {
			doc.DocumentDate = strings.TrimSpace(attr(t, "datetime"))
		} else {
			doc.DocumentDate = text(dd)
		}
	}
	if _, err := time.Parse(dateLayout, doc.DocumentDate); err != nil {
		return Document{}, fmt.Errorf("document date %q is missing or malformed", doc.DocumentDate)
	}

	if dd := fields[labelCase]; dd != nil {
		if a := find(dd, func(n *html.Node) bool { return n.Data == "a" && strings.Contains(attr(n, "href"), "/Case/") }); a != nil {
			href, err := url.Parse(attr(a, "href"))
			if err == nil {
				doc.CaseNumber = strings.TrimSpace(href.Query().Get("id"))
				if base != nil {
					doc.CaseURL = base.ResolveReference(href).String()
				} else {
					doc.CaseURL = href.String()
				}
			}
			doc.CaseTitle = text(a)
		}
	}
	if doc.CaseNumber == "" {
		return Document{}, errors.New("case link missing")
	}
	doc.CaseStatus = text(fields[labelCaseStatus])

	if dd := fields[labelOrganisation]; dd != nil {
		if a := find(dd, func(n *html.Node) bool { return n.Data == "a" && strings.Contains(attr(n, "href"), "/Company/") }); a != nil {
			if href, err := url.Parse(attr(a, "href")); err == nil {
				doc.OrganisationNumber = strings.TrimSpace(href.Query().Get("orgnr"))
			}
			doc.OrganisationName = text(a)
		} else {
			doc.OrganisationName = text(dd)
		}
	}
	if doc.OrganisationNumber == "" {
		doc.OrganisationNumber = text(fields[labelOrgNumber])
	}

	doc.Origin = text(fields[labelOrigin])
	doc.SubjectArea = text(fields[labelSubjectArea])
	doc.WorkplaceName = text(fields[labelWorkplace])
	doc.WorkplaceCFAR = text(fields[labelCFAR])
	return doc, nil
}

// definitionList maps every <dt> label in n to the <dd> that follows it.
// Labels are whitespace-collapsed with any trailing colon removed.
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

func find(n *html.Node, match func(*html.Node) bool) *html.Node {
	if n.Type == html.ElementNode && match(n) {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := find(c, match); found != nil {
			return found
		}
	}
	return nil
}

func findAll(n *html.Node, match func(*html.Node) bool) []*html.Node {
	var out []*html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && match(n) {
			out = append(out, n)
			return // matches do not nest
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

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func hasAttr(n *html.Node, key string) bool {
	for _, a := range n.Attr {
		if a.Key == key {
			return true
		}
	}
	return false
}

func hasClass(n *html.Node, class string) bool {
	for _, c := range strings.Fields(attr(n, "class")) {
		if c == class {
			return true
		}
	}
	return false
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
	return strings.Join(strings.Fields(b.String()), " ")
}

func render(n *html.Node) string {
	var b strings.Builder
	_ = html.Render(&b, n)
	return b.String()
}
