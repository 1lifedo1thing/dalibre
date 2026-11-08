package opds

import (
	"crypto/subtle"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"dalibre/internal/calibre"
)

// PageSize is how many books a page of the server's feeds lists.
const PageSize = 30

// Server serves a calibre library as an OPDS 1.2 catalog, for reading
// apps on the network: a navigation feed of ways in, acquisition feeds of
// books, search, covers and the books' files. It only reads the library.
type Server struct {
	// Library is the library to serve, nil while none is open.
	Library func() *calibre.Library
	// Title names the catalog in reading apps.
	Title string
	// Username and Password, when Username is set, are asked for with
	// HTTP Basic authentication.
	Username, Password string
	// Prefix is where the catalog is mounted, "/opds" by default.
	Prefix string

	mu     sync.Mutex
	lib    *calibre.Library
	cached []*calibre.Book
	loaded time.Time
	served map[string]time.Time // readers seen, by address, for the status line
}

func (s *Server) prefix() string {
	if s.Prefix == "" {
		return "/opds"
	}
	return strings.TrimSuffix(s.Prefix, "/")
}

// Readers is how many reading apps (by address) used the catalog in the
// last day.
func (s *Server) Readers() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, t := range s.served {
		if time.Since(t) < 24*time.Hour {
			n++
		}
	}
	return n
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.Username != "" {
		u, p, ok := r.BasicAuth()
		if !ok || subtle.ConstantTimeCompare([]byte(u), []byte(s.Username)) != 1 || subtle.ConstantTimeCompare([]byte(p), []byte(s.Password)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="Dalibre", charset="UTF-8"`)
			http.Error(w, "Sign in to this library", http.StatusUnauthorized)
			return
		}
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "Only GET", http.StatusMethodNotAllowed)
		return
	}
	s.mu.Lock()
	if s.served == nil {
		s.served = map[string]time.Time{}
	}
	host := r.RemoteAddr
	if i := strings.LastIndexByte(host, ':'); i >= 0 {
		host = host[:i]
	}
	s.served[host] = time.Now()
	s.mu.Unlock()

	rest, ok := strings.CutPrefix(r.URL.Path, s.prefix())
	if !ok {
		http.Redirect(w, r, s.prefix(), http.StatusFound)
		return
	}
	rest = strings.Trim(rest, "/")
	lib, books, err := s.load()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	page = max(page, 1)
	switch {
	case rest == "":
		s.root(w)
	case rest == "search.xml":
		s.openSearch(w)
	case rest == "new":
		sorted := slices.Clone(books)
		slices.SortStableFunc(sorted, func(a, b *calibre.Book) int { return b.Timestamp.Compare(a.Timestamp) })
		s.bookFeed(w, lib, "New books", "new", nil, sorted, page)
	case rest == "all":
		sorted := slices.Clone(books)
		slices.SortStableFunc(sorted, func(a, b *calibre.Book) int { return strings.Compare(sortKey(a), sortKey(b)) })
		s.bookFeed(w, lib, "All books", "all", nil, sorted, page)
	case rest == "search":
		terms := strings.ToLower(strings.TrimSpace(q.Get("q")))
		var found []*calibre.Book
		for _, b := range books {
			if terms != "" && matches(b, terms) {
				found = append(found, b)
			}
		}
		s.bookFeed(w, lib, "Search: "+q.Get("q"), "search", url.Values{"q": {q.Get("q")}}, found, page)
	case rest == "authors" || rest == "series" || rest == "tags":
		s.groups(w, rest, books, q.Get("name"), page, lib)
	case strings.HasPrefix(rest, "cover/"):
		s.cover(w, r, lib, books, strings.TrimPrefix(rest, "cover/"))
	case strings.HasPrefix(rest, "book/"):
		s.file(w, r, lib, books, strings.TrimPrefix(rest, "book/"))
	default:
		http.NotFound(w, r)
	}
}

// load returns the library's books, reading them again when the library
// changed or they are older than a few seconds.
func (s *Server) load() (*calibre.Library, []*calibre.Book, error) {
	var lib *calibre.Library
	if s.Library != nil {
		lib = s.Library()
	}
	if lib == nil {
		return nil, nil, fmt.Errorf("no library is open in Dalibre")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lib == lib && time.Since(s.loaded) < 5*time.Second {
		return lib, s.cached, nil
	}
	books, err := lib.Books()
	if err != nil {
		return nil, nil, err
	}
	s.lib, s.cached, s.loaded = lib, books, time.Now()
	return lib, books, nil
}

func sortKey(b *calibre.Book) string {
	if b.Sort != "" {
		return strings.ToLower(b.Sort)
	}
	return strings.ToLower(b.Title)
}

func matches(b *calibre.Book, terms string) bool {
	hay := strings.ToLower(b.Title + " " + strings.Join(b.Authors, " ") + " " + b.Series + " " + strings.Join(b.Tags, " ") + " " + b.Publisher)
	for _, t := range strings.Fields(terms) {
		if !strings.Contains(hay, t) {
			return false
		}
	}
	return true
}

func (s *Server) title() string {
	if s.Title == "" {
		return "Dalibre"
	}
	return s.Title
}

// ---- Feeds

type outFeed struct {
	XMLName   xml.Name   `xml:"feed"`
	Xmlns     string     `xml:"xmlns,attr"`
	XmlnsDC   string     `xml:"xmlns:dc,attr"`
	XmlnsOPDS string     `xml:"xmlns:opds,attr"`
	XmlnsOS   string     `xml:"xmlns:opensearch,attr"`
	ID        string     `xml:"id"`
	Title     string     `xml:"title"`
	Updated   string     `xml:"updated"`
	Author    *outAuthor `xml:"author,omitempty"`
	Total     int        `xml:"opensearch:totalResults,omitempty"`
	Links     []outLink  `xml:"link"`
	Entries   []outEntry `xml:"entry"`
}

type outAuthor struct {
	Name string `xml:"name"`
}

type outLink struct {
	Rel    string `xml:"rel,attr,omitempty"`
	Href   string `xml:"href,attr"`
	Type   string `xml:"type,attr,omitempty"`
	Title  string `xml:"title,attr,omitempty"`
	Length int64  `xml:"length,attr,omitempty"`
}

type outEntry struct {
	ID         string        `xml:"id"`
	Title      string        `xml:"title"`
	Updated    string        `xml:"updated"`
	Authors    []outAuthor   `xml:"author"`
	Language   []string      `xml:"dc:language,omitempty"`
	Publisher  string        `xml:"dc:publisher,omitempty"`
	Issued     string        `xml:"dc:issued,omitempty"`
	Identifier []string      `xml:"dc:identifier,omitempty"`
	Categories []outCategory `xml:"category"`
	Content    *outContent   `xml:"content,omitempty"`
	Links      []outLink     `xml:"link"`
}

type outCategory struct {
	Term  string `xml:"term,attr"`
	Label string `xml:"label,attr"`
}

type outContent struct {
	Type string `xml:"type,attr"`
	Body string `xml:",chardata"`
}

const (
	navType = TypeAtom + ";profile=opds-catalog;kind=navigation"
	acqType = TypeAtom + ";profile=opds-catalog;kind=acquisition"
)

func (s *Server) feed(id, title string, self, kind string) *outFeed {
	return &outFeed{
		Xmlns: "http://www.w3.org/2005/Atom", XmlnsDC: "http://purl.org/dc/terms/", XmlnsOPDS: "http://opds-spec.org/2010/catalog",
		XmlnsOS: "http://a9.com/-/spec/opensearch/1.1/",
		ID:      "urn:dalibre:" + id, Title: title, Updated: time.Now().UTC().Format(time.RFC3339), Author: &outAuthor{Name: "Dalibre"},
		Links: []outLink{
			{Rel: "self", Href: self, Type: kind},
			{Rel: "start", Href: s.prefix(), Type: navType, Title: s.title()},
			{Rel: "search", Href: s.prefix() + "/search.xml", Type: TypeOpenSearch, Title: "Search"},
		},
	}
}

func write(w http.ResponseWriter, kind string, v any) {
	w.Header().Set("Content-Type", kind+";charset=utf-8")
	w.Write([]byte(xml.Header))
	enc := xml.NewEncoder(w)
	enc.Indent("", " ")
	enc.Encode(v)
}

func (s *Server) root(w http.ResponseWriter) {
	f := s.feed("root", s.title(), s.prefix(), navType)
	nav := func(id, title, summary, href string) {
		f.Entries = append(f.Entries, outEntry{ID: "urn:dalibre:" + id, Title: title, Updated: f.Updated,
			Content: &outContent{Type: "text", Body: summary}, Links: []outLink{{Rel: "subsection", Href: href, Type: acqType}}})
	}
	p := s.prefix()
	nav("new", "New books", "The newest books in the library", p+"/new")
	nav("authors", "By author", "Every author, A–Z", p+"/authors")
	nav("series", "By series", "Every series", p+"/series")
	nav("tags", "By tag", "Subjects and genres", p+"/tags")
	nav("all", "All books", "The whole library, by title", p+"/all")
	for i := range f.Entries {
		if f.Entries[i].ID != "urn:dalibre:new" && f.Entries[i].ID != "urn:dalibre:all" {
			f.Entries[i].Links[0].Type = navType
		}
	}
	write(w, navType, f)
}

func (s *Server) openSearch(w http.ResponseWriter) {
	type urlT struct {
		Type     string `xml:"type,attr"`
		Template string `xml:"template,attr"`
	}
	d := struct {
		XMLName     xml.Name `xml:"OpenSearchDescription"`
		Xmlns       string   `xml:"xmlns,attr"`
		ShortName   string   `xml:"ShortName"`
		Description string   `xml:"Description"`
		URL         urlT     `xml:"Url"`
	}{Xmlns: "http://a9.com/-/spec/opensearch/1.1/", ShortName: s.title(), Description: "Search " + s.title(),
		URL: urlT{Type: TypeAtom, Template: s.prefix() + "/search?q={searchTerms}"}}
	write(w, TypeOpenSearch, d)
}

// books writes an acquisition feed of a page of list.
func (s *Server) bookFeed(w http.ResponseWriter, lib *calibre.Library, title, id string, params url.Values, list []*calibre.Book, page int) {
	self := func(p int) string {
		v := url.Values{}
		for k, vs := range params {
			v[k] = vs
		}
		if p > 1 {
			v.Set("page", strconv.Itoa(p))
		}
		u := s.prefix() + "/" + id
		if e := v.Encode(); e != "" {
			u += "?" + e
		}
		return u
	}
	f := s.feed(id, title, self(page), acqType)
	f.Total = len(list)
	from := (page - 1) * PageSize
	if from > len(list) {
		from = len(list)
	}
	to := min(from+PageSize, len(list))
	if to < len(list) {
		f.Links = append(f.Links, outLink{Rel: "next", Href: self(page + 1), Type: acqType})
	}
	if page > 1 {
		f.Links = append(f.Links, outLink{Rel: "previous", Href: self(page - 1), Type: acqType})
	}
	for _, b := range list[from:to] {
		f.Entries = append(f.Entries, s.entry(lib, b))
	}
	write(w, acqType, f)
}

var formatMIME = map[string]string{
	"EPUB": "application/epub+zip", "KEPUB": "application/kepub+zip", "AZW3": "application/x-mobi8-ebook",
	"AZW": "application/vnd.amazon.ebook", "MOBI": "application/x-mobipocket-ebook", "PDF": "application/pdf",
	"FB2": "application/x-fictionbook+xml", "CBZ": "application/vnd.comicbook+zip", "CBR": "application/vnd.comicbook-rar",
	"DJVU": "image/vnd.djvu", "TXT": "text/plain", "RTF": "application/rtf", "DOCX": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
}

func (s *Server) entry(lib *calibre.Library, b *calibre.Book) outEntry {
	e := outEntry{ID: "urn:uuid:" + b.UUID, Title: b.Title, Updated: b.LastModified.UTC().Format(time.RFC3339),
		Language: b.Languages, Publisher: b.Publisher}
	if b.UUID == "" {
		e.ID = fmt.Sprintf("urn:dalibre:book:%d", b.ID)
	}
	for _, a := range b.Authors {
		e.Authors = append(e.Authors, outAuthor{Name: a})
	}
	if !b.Pubdate.IsZero() {
		e.Issued = b.Pubdate.Format("2006-01-02")
	}
	for _, t := range b.Tags {
		e.Categories = append(e.Categories, outCategory{Term: t, Label: t})
	}
	d, err := lib.Details(b)
	if err == nil {
		if d.Comments != "" {
			e.Content = &outContent{Type: "html", Body: d.Comments}
		}
		for k, v := range d.Identifiers {
			if k == "isbn" {
				e.Identifier = append(e.Identifier, "urn:isbn:"+v)
			}
		}
		for _, f := range d.Files {
			mt := formatMIME[f.Format]
			if mt == "" {
				mt = "application/octet-stream"
			}
			e.Links = append(e.Links, outLink{Rel: RelAcquisition, Href: fmt.Sprintf("%s/book/%d/%s", s.prefix(), b.ID, strings.ToLower(f.Format)),
				Type: mt, Length: f.Size, Title: f.Format})
		}
	}
	if b.HasCover {
		c := fmt.Sprintf("%s/cover/%d", s.prefix(), b.ID)
		e.Links = append(e.Links, outLink{Rel: RelImage, Href: c, Type: "image/jpeg"}, outLink{Rel: RelThumbnail, Href: c, Type: "image/jpeg"})
	}
	return e
}

// groups lists a library's authors, series or tags, or, given name, the
// books of one.
func (s *Server) groups(w http.ResponseWriter, kind string, books []*calibre.Book, name string, page int, lib *calibre.Library) {
	of := func(b *calibre.Book) []string {
		switch kind {
		case "authors":
			return b.Authors
		case "series":
			if b.Series != "" {
				return []string{b.Series}
			}
			return nil
		}
		return b.Tags
	}
	if name != "" {
		var list []*calibre.Book
		for _, b := range books {
			if slices.Contains(of(b), name) {
				list = append(list, b)
			}
		}
		slices.SortStableFunc(list, func(a, b *calibre.Book) int {
			if kind == "series" && a.SeriesIndex != b.SeriesIndex {
				if a.SeriesIndex < b.SeriesIndex {
					return -1
				}
				return 1
			}
			return strings.Compare(sortKey(a), sortKey(b))
		})
		s.bookFeed(w, lib, name, kind, url.Values{"name": {name}}, list, page)
		return
	}
	counts := map[string]int{}
	for _, b := range books {
		for _, n := range of(b) {
			counts[n]++
		}
	}
	names := make([]string, 0, len(counts))
	for n := range counts {
		names = append(names, n)
	}
	slices.SortFunc(names, func(a, b string) int { return strings.Compare(strings.ToLower(a), strings.ToLower(b)) })
	title := map[string]string{"authors": "By author", "series": "By series", "tags": "By tag"}[kind]
	f := s.feed(kind, title, s.prefix()+"/"+kind, navType)
	for _, n := range names {
		f.Entries = append(f.Entries, outEntry{ID: "urn:dalibre:" + kind + ":" + url.QueryEscape(n), Title: n, Updated: f.Updated,
			Content: &outContent{Type: "text", Body: fmt.Sprintf("%d %s", counts[n], map[bool]string{true: "book", false: "books"}[counts[n] == 1])},
			Links:   []outLink{{Rel: "subsection", Href: s.prefix() + "/" + kind + "?" + url.Values{"name": {n}}.Encode(), Type: acqType}}})
	}
	write(w, navType, f)
}

func bookOf(books []*calibre.Book, id string) *calibre.Book {
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return nil
	}
	for _, b := range books {
		if b.ID == n {
			return b
		}
	}
	return nil
}

func (s *Server) cover(w http.ResponseWriter, r *http.Request, lib *calibre.Library, books []*calibre.Book, id string) {
	b := bookOf(books, id)
	if b == nil || !b.HasCover {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "max-age=3600")
	http.ServeFile(w, r, lib.CoverPath(b))
}

func (s *Server) file(w http.ResponseWriter, r *http.Request, lib *calibre.Library, books []*calibre.Book, rest string) {
	id, format, _ := strings.Cut(rest, "/")
	b := bookOf(books, id)
	if b == nil {
		http.NotFound(w, r)
		return
	}
	d, err := lib.Details(b)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	for _, f := range d.Files {
		if strings.EqualFold(f.Format, format) {
			fh, err := os.Open(f.Path)
			if err != nil {
				http.NotFound(w, r)
				return
			}
			defer fh.Close()
			st, err := fh.Stat()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if mt := formatMIME[f.Format]; mt != "" {
				w.Header().Set("Content-Type", mt)
			}
			name := b.Title
			if len(b.Authors) > 0 {
				name += " - " + b.Authors[0]
			}
			w.Header().Set("Content-Disposition", mimeAttachment(name+"."+strings.ToLower(f.Format)))
			http.ServeContent(w, r, "", st.ModTime(), fh)
			return
		}
	}
	http.NotFound(w, r)
}

func mimeAttachment(name string) string {
	ascii := strings.Map(func(r rune) rune {
		if r < 32 || r > 126 || r == '"' || r == '\\' {
			return '_'
		}
		return r
	}, name)
	return fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`, ascii, url.PathEscape(name))
}
