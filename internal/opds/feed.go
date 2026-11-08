// Package opds reads and serves OPDS 1 catalogs: Atom feeds of books
// that reading apps and servers (calibre, Calibre-Web, Kavita, Standard
// Ebooks…) use to browse and download e-books.
//
// A catalog is a tree of feeds. A navigation feed lists other feeds ("By
// author", "New books"); an acquisition feed lists books with links to
// their files. Dalibre reads both as Feed, and tells a book entry from a
// navigation entry by its links.
package opds

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/url"
	"strconv"
	"strings"
)

// Link relations and media types OPDS uses.
const (
	RelAcquisition = "http://opds-spec.org/acquisition"
	RelImage       = "http://opds-spec.org/image"
	RelThumbnail   = "http://opds-spec.org/image/thumbnail"
	RelFacet       = "http://opds-spec.org/facet"

	TypeAtom       = "application/atom+xml"
	TypeOpenSearch = "application/opensearchdescription+xml"
)

// MaxFeed is the most a feed may hold, in bytes.
const MaxFeed = 16 << 20

// Feed is an OPDS feed.
type Feed struct {
	ID      string
	Title   string
	Updated string
	// Total is how many entries the whole list has, when the feed says
	// (opensearch:totalResults); 0 when it doesn't.
	Total   int
	Links   []Link
	Entries []Entry
}

// Link is a feed's or entry's link, with its href made absolute.
type Link struct {
	Rel, Href, Type, Title string
	Length                 int64 // bytes, for an acquisition link; 0 when unknown
	// FacetGroup and Active describe a facet link: a way to sort or narrow
	// the feed.
	FacetGroup string
	Active     bool
}

// Entry is a feed's entry: a book, or a link to another feed.
type Entry struct {
	ID, Title, Updated string
	Authors            []string
	// Summary is plain text; Content may be HTML.
	Summary, Content string
	Language         string
	Publisher        string
	Issued           string // dc:issued, as "2006" or "2006-01-02"
	Identifiers      []string
	Categories       []string
	Links            []Link
}

// IsBook reports whether the entry is a book: it links to a file.
func (e *Entry) IsBook() bool {
	for _, l := range e.Links {
		if isAcquisition(l.Rel) {
			return true
		}
	}
	return false
}

// Navigation is where a navigation entry leads: the first link to a
// feed. nil for a book.
func (e *Entry) Navigation() *Link {
	if e.IsBook() {
		return nil
	}
	for i, l := range e.Links {
		if isFeedType(l.Type) {
			return &e.Links[i]
		}
	}
	for i, l := range e.Links {
		if l.Rel == "subsection" || l.Rel == "http://opds-spec.org/sort/new" || l.Rel == "http://opds-spec.org/sort/popular" {
			return &e.Links[i]
		}
	}
	return nil
}

// Link is the feed's first link with rel, nil when it has none.
func (f *Feed) Link(rel string) *Link {
	for i, l := range f.Links {
		if l.Rel == rel {
			return &f.Links[i]
		}
	}
	return nil
}

// Next is the next page's URL, "" on the last.
func (f *Feed) Next() string {
	if l := f.Link("next"); l != nil {
		return l.Href
	}
	return ""
}

// Search is the feed's search link: an OpenSearch description, or a feed
// URL template with {searchTerms}. nil when the catalog can't search.
func (f *Feed) Search() *Link {
	for i, l := range f.Links {
		if l.Rel == "search" && (mediaType(l.Type) == TypeOpenSearch || strings.Contains(l.Href, "{searchTerms}")) {
			return &f.Links[i]
		}
	}
	return nil
}

// Facets are the feed's facet links, in their groups' order.
func (f *Feed) Facets() []Link {
	var out []Link
	for _, l := range f.Links {
		if l.Rel == RelFacet {
			out = append(out, l)
		}
	}
	return out
}

func isAcquisition(rel string) bool {
	if !strings.HasPrefix(rel, RelAcquisition) {
		return false
	}
	// Buying, borrowing and subscribing go through the catalog's site.
	switch strings.TrimPrefix(rel, RelAcquisition) {
	case "", "/open-access", "/sample", "/preview":
		return true
	}
	return false
}

func isFeedType(t string) bool {
	return mediaType(t) == TypeAtom || strings.HasPrefix(t, "application/atom+xml;")
}

func mediaType(t string) string {
	mt, _, err := mime.ParseMediaType(t)
	if err != nil {
		return strings.ToLower(strings.TrimSpace(t))
	}
	return mt
}

// ErrNotOPDS is what Parse fails with for something that isn't an OPDS
// feed: a web page, or an OPDS 2 (JSON) catalog.
var ErrNotOPDS = errors.New("isn’t an OPDS feed")

// Parse reads an OPDS feed. base is the URL it came from; relative links
// are made absolute against it.
func Parse(r io.Reader, base string) (*Feed, error) {
	data, err := io.ReadAll(io.LimitReader(r, MaxFeed+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxFeed {
		return nil, fmt.Errorf("the feed is over %d MB", MaxFeed>>20)
	}
	trimmed := bytes.TrimSpace(data)
	if bytes.HasPrefix(trimmed, []byte("{")) {
		return nil, fmt.Errorf("%w: it is an OPDS 2 catalog, which Dalibre can’t read yet", ErrNotOPDS)
	}
	b, err := url.Parse(base)
	if err != nil {
		return nil, err
	}
	var x xmlFeed
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.Strict = false
	dec.CharsetReader = charsetReader
	if err := dec.Decode(&x); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNotOPDS, err)
	}
	if x.XMLName.Local != "feed" {
		return nil, fmt.Errorf("%w: it is %s, not an Atom feed", ErrNotOPDS, describeRoot(x.XMLName.Local))
	}
	f := &Feed{ID: strings.TrimSpace(x.ID), Title: text(x.Title), Updated: strings.TrimSpace(x.Updated), Links: links(x.Links, b)}
	f.Total, _ = strconv.Atoi(strings.TrimSpace(x.Total))
	for _, xe := range x.Entries {
		e := Entry{
			ID: strings.TrimSpace(xe.ID), Title: text(xe.Title), Updated: strings.TrimSpace(xe.Updated),
			Summary: text(xe.Summary), Content: text(xe.Content),
			Language: first(xe.Language), Publisher: first(xe.Publisher), Issued: first(xe.Issued),
			Links: links(xe.Links, b),
		}
		for _, a := range xe.Authors {
			if n := strings.TrimSpace(a.Name); n != "" {
				e.Authors = append(e.Authors, n)
			}
		}
		for _, id := range xe.Identifiers {
			if id = strings.TrimSpace(id); id != "" {
				e.Identifiers = append(e.Identifiers, id)
			}
		}
		for _, c := range xe.Categories {
			label := strings.TrimSpace(c.Label)
			if label == "" {
				label = strings.TrimSpace(c.Term)
			}
			if label != "" {
				e.Categories = append(e.Categories, label)
			}
		}
		if e.Title == "" {
			continue
		}
		f.Entries = append(f.Entries, e)
	}
	return f, nil
}

func describeRoot(name string) string {
	switch strings.ToLower(name) {
	case "html":
		return "a web page"
	case "":
		return "empty"
	}
	return "<" + name + ">"
}

func links(xs []xmlLink, base *url.URL) []Link {
	var out []Link
	for _, l := range xs {
		href := strings.TrimSpace(l.Href)
		if href == "" {
			continue
		}
		// Templates keep their braces, which url.Parse would escape.
		abs := href
		if u, err := url.Parse(strings.NewReplacer("{", "%7B", "}", "%7D").Replace(href)); err == nil {
			abs = strings.NewReplacer("%7B", "{", "%7D", "}").Replace(base.ResolveReference(u).String())
		}
		n, _ := strconv.ParseInt(strings.TrimSpace(l.Length), 10, 64)
		out = append(out, Link{Rel: strings.TrimSpace(l.Rel), Href: abs, Type: strings.TrimSpace(l.Type), Title: strings.TrimSpace(l.Title),
			Length: n, FacetGroup: strings.TrimSpace(l.FacetGroup), Active: l.ActiveFacet == "true"})
	}
	return out
}

func first(xs []string) string {
	for _, s := range xs {
		if s = strings.TrimSpace(s); s != "" {
			return s
		}
	}
	return ""
}

func text(t xmlText) string { return strings.TrimSpace(t.Body) }

// The feed as XML. Element names match any namespace, so Dublin Core in
// either of its namespaces reads the same.
type xmlFeed struct {
	XMLName xml.Name
	ID      string     `xml:"id"`
	Title   xmlText    `xml:"title"`
	Updated string     `xml:"updated"`
	Total   string     `xml:"totalResults"`
	Links   []xmlLink  `xml:"link"`
	Entries []xmlEntry `xml:"entry"`
}

type xmlEntry struct {
	ID          string        `xml:"id"`
	Title       xmlText       `xml:"title"`
	Updated     string        `xml:"updated"`
	Authors     []xmlAuthor   `xml:"author"`
	Summary     xmlText       `xml:"summary"`
	Content     xmlText       `xml:"content"`
	Language    []string      `xml:"language"`
	Publisher   []string      `xml:"publisher"`
	Issued      []string      `xml:"issued"`
	Identifiers []string      `xml:"identifier"`
	Categories  []xmlCategory `xml:"category"`
	Links       []xmlLink     `xml:"link"`
}

type xmlAuthor struct {
	Name string `xml:"name"`
}

type xmlCategory struct {
	Term  string `xml:"term,attr"`
	Label string `xml:"label,attr"`
}

type xmlLink struct {
	Rel         string `xml:"rel,attr"`
	Href        string `xml:"href,attr"`
	Type        string `xml:"type,attr"`
	Title       string `xml:"title,attr"`
	Length      string `xml:"length,attr"`
	FacetGroup  string `xml:"facetGroup,attr"`
	ActiveFacet string `xml:"activeFacet,attr"`
}

// xmlText is an element's text, with any XHTML inside it as markup.
type xmlText struct {
	Type string `xml:"type,attr"`
	Body string `xml:",innerxml"`
}

func (t *xmlText) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, a := range start.Attr {
		if a.Name.Local == "type" {
			t.Type = a.Value
		}
	}
	var b strings.Builder
	depth := 0
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch v := tok.(type) {
		case xml.StartElement:
			depth++
			b.WriteString("<" + v.Name.Local + ">")
		case xml.EndElement:
			if depth == 0 {
				t.Body = b.String()
				return nil
			}
			depth--
			b.WriteString("</" + v.Name.Local + ">")
		case xml.CharData:
			if depth > 0 || t.Type == "xhtml" {
				xml.EscapeText(&b, v)
			} else {
				b.Write(v)
			}
		}
	}
}

// charsetReader reads the one other encoding feeds use in practice; UTF-8
// and ASCII need none.
func charsetReader(charset string, input io.Reader) (io.Reader, error) {
	switch strings.ToLower(charset) {
	case "utf-8", "us-ascii", "ascii":
		return input, nil
	case "iso-8859-1", "latin1", "latin-1", "windows-1252":
		data, err := io.ReadAll(input)
		if err != nil {
			return nil, err
		}
		runes := make([]rune, len(data))
		for i, c := range data {
			runes[i] = rune(c)
		}
		return strings.NewReader(string(runes)), nil
	}
	return nil, fmt.Errorf("the feed is in %s, which Dalibre can’t read", charset)
}
