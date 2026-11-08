package opds

import (
	"context"
	"encoding/xml"
	"fmt"
	"mime"
	"net/url"
	"path"
	"strings"
	"sync"

	"dalibre/internal/plugin"
)

// Source is an OPDS catalog as a book source: its root feed's navigation
// entries are collections, and it searches when the catalog can.
type Source struct {
	Root string // the catalog's root feed

	mu       sync.Mutex
	template string // the search URL template, once found
}

var _ plugin.Native = (*Source)(nil)

// Fetch reads the feed at rawURL.
func Fetch(ctx context.Context, get plugin.Getter, rawURL string) (*Feed, error) {
	resp, err := get(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, plugin.HTTPError(resp)
	}
	f, err := Parse(resp.Body, resp.Request.URL.String())
	if err != nil {
		return nil, &plugin.Error{Kind: plugin.KindInvalid, Msg: fmt.Sprintf("%s %v", resp.Request.URL.Redacted(), err), Err: err}
	}
	return f, nil
}

// Collections are the root feed's navigation entries, by their URLs.
func (s *Source) Collections(ctx context.Context, get plugin.Getter) ([]plugin.Collection, error) {
	f, err := Fetch(ctx, get, s.Root)
	if err != nil {
		return nil, err
	}
	s.findSearch(ctx, get, f)
	var out []plugin.Collection
	for _, e := range f.Entries {
		if l := e.Navigation(); l != nil && len(out) < plugin.MaxCollections {
			out = append(out, plugin.Collection{ID: l.Href, Title: e.Title})
		}
	}
	return out, nil
}

// List is a page of a feed: the collection's URL, or the cursor, which is
// the next page's URL.
func (s *Source) List(ctx context.Context, get plugin.Getter, q plugin.Query) (plugin.Page, error) {
	u := q.Collection
	if q.Cursor != "" {
		u = q.Cursor
	}
	return s.page(ctx, get, u)
}

// Search fills the catalog's search template with q.Text.
func (s *Source) Search(ctx context.Context, get plugin.Getter, q plugin.Query) (plugin.Page, error) {
	if q.Cursor != "" {
		return s.page(ctx, get, q.Cursor)
	}
	t, err := s.SearchTemplate(ctx, get)
	if err != nil {
		return plugin.Page{}, err
	}
	if t == "" {
		return plugin.Page{}, &plugin.Error{Kind: plugin.KindInvalid, Msg: "this catalog can’t search"}
	}
	return s.page(ctx, get, FillTemplate(t, q.Text))
}

func (s *Source) page(ctx context.Context, get plugin.Getter, u string) (plugin.Page, error) {
	f, err := Fetch(ctx, get, u)
	if err != nil {
		return plugin.Page{}, err
	}
	pg := plugin.Page{Cursor: f.Next()}
	pg.HasMore = pg.Cursor != ""
	for _, e := range f.Entries {
		if r, ok := EntryResult(e); ok {
			pg.Items = append(pg.Items, r)
		}
	}
	return pg, nil
}

// SearchTemplate is the catalog's search URL template, with
// {searchTerms}; "" when it can't search.
func (s *Source) SearchTemplate(ctx context.Context, get plugin.Getter) (string, error) {
	s.mu.Lock()
	t := s.template
	s.mu.Unlock()
	if t != "" {
		return t, nil
	}
	f, err := Fetch(ctx, get, s.Root)
	if err != nil {
		return "", err
	}
	return s.findSearch(ctx, get, f), nil
}

func (s *Source) findSearch(ctx context.Context, get plugin.Getter, root *Feed) string {
	t := SearchTemplate(ctx, get, root)
	if t != "" {
		s.mu.Lock()
		s.template = t
		s.mu.Unlock()
	}
	return t
}

// SearchTemplate finds the search URL template of a catalog from its
// root feed: a templated search link, or the feed template in its
// OpenSearch description. "" when it has none.
func SearchTemplate(ctx context.Context, get plugin.Getter, root *Feed) string {
	l := root.Search()
	if l == nil {
		return ""
	}
	if strings.Contains(l.Href, "{searchTerms}") {
		return l.Href
	}
	resp, err := get(ctx, l.Href)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return ""
	}
	var d struct {
		URLs []struct {
			Type     string `xml:"type,attr"`
			Template string `xml:"template,attr"`
		} `xml:"Url"`
	}
	dec := xml.NewDecoder(resp.Body)
	dec.Strict = false
	dec.CharsetReader = charsetReader
	if dec.Decode(&d) != nil {
		return ""
	}
	base := resp.Request.URL
	best := ""
	for _, u := range d.URLs {
		if !strings.Contains(u.Template, "{searchTerms}") {
			continue
		}
		abs := u.Template
		if ref, err := url.Parse(strings.NewReplacer("{", "%7B", "}", "%7D").Replace(u.Template)); err == nil {
			abs = strings.NewReplacer("%7B", "{", "%7D", "}").Replace(base.ResolveReference(ref).String())
		}
		if mt, _, _ := mime.ParseMediaType(u.Type); mt == TypeAtom {
			return abs
		}
		if best == "" {
			best = abs
		}
	}
	return best
}

// FillTemplate puts terms in an OpenSearch URL template. Optional
// parameters it doesn't know are left empty; the first page is asked for.
func FillTemplate(t, terms string) string {
	var b strings.Builder
	for {
		i := strings.IndexByte(t, '{')
		if i < 0 {
			b.WriteString(t)
			return b.String()
		}
		j := strings.IndexByte(t[i:], '}')
		if j < 0 {
			b.WriteString(t)
			return b.String()
		}
		b.WriteString(t[:i])
		name := strings.TrimSuffix(t[i+1:i+j], "?")
		if k := strings.LastIndexByte(name, ':'); k >= 0 {
			name = name[k+1:]
		}
		switch name {
		case "searchTerms":
			b.WriteString(url.QueryEscape(terms))
		case "startPage":
			b.WriteString("1")
		case "startIndex":
			b.WriteString("0")
		case "count":
			b.WriteString("25")
		case "language", "inputEncoding", "outputEncoding":
			if name == "language" {
				b.WriteString("*")
			} else {
				b.WriteString("UTF-8")
			}
		}
		t = t[i+j+1:]
	}
}

// formatTypes maps the media types of acquisition links to formats.
var formatTypes = map[string]string{
	"application/epub+zip":             "EPUB",
	"application/kepub+zip":            "KEPUB",
	"application/x-kobo-epub+zip":      "KEPUB",
	"application/x-mobipocket-ebook":   "MOBI",
	"application/x-mobi8-ebook":        "AZW3",
	"application/vnd.amazon.ebook":     "AZW3",
	"application/x-amazon-ebook":       "AZW3",
	"application/pdf":                  "PDF",
	"application/x-fictionbook+xml":    "FB2",
	"application/fb2+zip":              "FB2",
	"text/fb2+xml":                     "FB2",
	"application/vnd.comicbook+zip":    "CBZ",
	"application/x-cbz":                "CBZ",
	"application/vnd.comicbook-rar":    "CBR",
	"application/x-cbr":                "CBR",
	"image/vnd.djvu":                   "DJVU",
	"application/x-djvu":               "DJVU",
	"text/plain":                       "TXT",
	"application/rtf":                  "RTF",
	"application/x-ms-reader":          "LIT",
	"application/x-zip-compressed-fb2": "FB2",
}

// formatOf is a link's format, by its media type or else its file's
// extension; "" for a file Dalibre doesn't keep.
func formatOf(l Link) string {
	if f := formatTypes[mediaType(l.Type)]; f != "" {
		return f
	}
	if u, err := url.Parse(l.Href); err == nil {
		ext := strings.ToUpper(strings.TrimPrefix(path.Ext(u.Path), "."))
		if ext == "AZW" || ext == "AZW3" || ext == "EPUB" || ext == "MOBI" || ext == "PDF" || ext == "FB2" || ext == "CBZ" || ext == "KEPUB" {
			return ext
		}
	}
	return ""
}

// order is the formats' order of preference, for the Get button.
var order = []string{"EPUB", "KEPUB", "AZW3", "MOBI", "PDF", "FB2", "CBZ", "CBR", "DJVU", "TXT", "RTF", "LIT"}

// EntryResult is a book entry as a book source's result; false for an
// entry with no file Dalibre keeps.
func EntryResult(e Entry) (plugin.Result, bool) {
	r := plugin.Result{
		ID: e.ID, Title: e.Title, Authors: e.Authors, Publisher: e.Publisher, Language: e.Language,
		Tags: e.Categories, Identifiers: map[string]string{},
	}
	r.Description = e.Content
	if r.Description == "" {
		r.Description = e.Summary
	}
	if d := issued(e.Issued); d != "" {
		r.Pubdate = d
	}
	for _, id := range append([]string{e.ID}, e.Identifiers...) {
		low := strings.ToLower(id)
		if isbn, ok := strings.CutPrefix(low, "urn:isbn:"); ok {
			r.Identifiers["isbn"] = strings.ReplaceAll(isbn, "-", "")
		}
	}
	seen := map[string]bool{}
	for _, l := range e.Links {
		switch {
		case isAcquisition(l.Rel):
			f := formatOf(l)
			if f != "" && !seen[f] {
				seen[f] = true
				r.Formats = append(r.Formats, plugin.Format{Format: f, URL: l.Href, Size: l.Length})
			}
		case l.Rel == RelImage:
			r.Cover = l.Href
		case l.Rel == RelThumbnail && r.Cover == "":
			r.Cover = l.Href
		case l.Rel == "alternate" && mediaType(l.Type) == "text/html" && r.URL == "":
			r.URL = l.Href
		}
	}
	if r.ID == "" && len(r.Formats) > 0 {
		r.ID = r.Formats[0].URL
	}
	if len(r.Formats) == 0 || r.ID == "" {
		return plugin.Result{}, false
	}
	rank := func(f string) int {
		for i, o := range order {
			if o == f {
				return i
			}
		}
		return len(order)
	}
	for i := 1; i < len(r.Formats); i++ {
		for j := i; j > 0 && rank(r.Formats[j].Format) < rank(r.Formats[j-1].Format); j-- {
			r.Formats[j], r.Formats[j-1] = r.Formats[j-1], r.Formats[j]
		}
	}
	return r, true
}

// issued keeps the date part of dc:issued, which may be a year, a day or
// a full time.
func issued(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 10 && !plugin.ParseDate(s[:10]).IsZero() {
		return s[:10]
	}
	if plugin.ParseDate(s).IsZero() {
		return ""
	}
	return s
}
