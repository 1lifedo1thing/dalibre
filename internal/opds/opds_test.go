package opds

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dalibre/internal/calibre"
	"dalibre/internal/plugin"
)

func testLibrary(t *testing.T) *calibre.Library {
	t.Helper()
	lib, err := calibre.Create(filepath.Join(t.TempDir(), "Library"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { lib.Close() })
	add := func(title, author, series string, tags ...string) {
		f := filepath.Join(t.TempDir(), "in.epub")
		os.WriteFile(f, []byte("epub of "+title), 0o644)
		if _, err := lib.AddBook(calibre.NewBook{File: f, Metadata: calibre.Metadata{Title: title, Authors: []string{author}, Series: series, Tags: tags,
			Identifiers: map[string]string{"isbn": "9780441478125"}, Comments: "<p>About " + title + "</p>"}}); err != nil {
			t.Fatal(err)
		}
	}
	add("The Left Hand of Darkness", "Ursula K. Le Guin", "Hainish Cycle", "Science fiction")
	add("A Wizard of Earthsea", "Ursula K. Le Guin", "Earthsea", "Fantasy")
	add("Emma", "Jane Austen", "")
	return lib
}

func plainGet(user, pass string) plugin.Getter {
	return func(ctx context.Context, u string) (*http.Response, error) {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if user != "" {
			req.SetBasicAuth(user, pass)
		}
		return http.DefaultClient.Do(req)
	}
}

func TestServerRoundTrip(t *testing.T) {
	lib := testLibrary(t)
	srv := httptest.NewServer(&Server{Library: func() *calibre.Library { return lib }, Title: "Test", Username: "randy", Password: "pw"})
	defer srv.Close()
	ctx := context.Background()
	get := plainGet("randy", "pw")

	if resp, _ := plainGet("", "")(ctx, srv.URL+"/opds"); resp.StatusCode != 401 {
		t.Fatalf("without a password: %d", resp.StatusCode)
	}

	src := &Source{Root: srv.URL + "/opds"}
	cs, err := src.Collections(ctx, get)
	if err != nil {
		t.Fatal(err)
	}
	var titles []string
	for _, c := range cs {
		titles = append(titles, c.Title)
	}
	if got := strings.Join(titles, ","); got != "New books,By author,By series,By tag,All books" {
		t.Fatalf("collections %s", got)
	}

	// By author is a navigation feed; one author's books an acquisition feed.
	f, err := Fetch(ctx, get, cs[1].ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Entries) != 2 || f.Entries[1].Title != "Ursula K. Le Guin" || f.Entries[1].IsBook() {
		t.Fatalf("authors %+v", f.Entries)
	}
	pg, err := src.List(ctx, get, plugin.Query{Collection: f.Entries[1].Navigation().Href})
	if err != nil {
		t.Fatal(err)
	}
	if len(pg.Items) != 2 || pg.Items[0].Title != "The Left Hand of Darkness" { // by title sort
		t.Fatalf("le guin %+v", pg.Items)
	}
	r := pg.Items[0]
	if r.Formats[0].Format != "EPUB" || r.Formats[0].Size == 0 || r.Identifiers["isbn"] != "9780441478125" || !strings.Contains(r.Description, "About") {
		t.Errorf("result %+v", r)
	}

	// Search goes through the OpenSearch description.
	pg, err = src.Search(ctx, get, plugin.Query{Text: "darkness"})
	if err != nil {
		t.Fatal(err)
	}
	if len(pg.Items) != 1 || pg.Items[0].Title != "The Left Hand of Darkness" {
		t.Fatalf("search %+v", pg.Items)
	}

	// The file downloads.
	resp, err := get(ctx, pg.Items[0].Formats[0].URL)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(body) != "epub of The Left Hand of Darkness" {
		t.Errorf("file %q", body)
	}
}

func TestParseThirdParty(t *testing.T) {
	const feed = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom" xmlns:dcterms="http://purl.org/dc/terms/" xmlns:opds="http://opds-spec.org/2010/catalog">
 <id>https://example.org/opds</id><title>Example</title>
 <link rel="search" type="application/atom+xml" href="/search?q={searchTerms}"/>
 <link rel="next" type="application/atom+xml;profile=opds-catalog;kind=acquisition" href="?page=2"/>
 <entry>
  <title>Persuasion</title><id>urn:isbn:978-0-14-143968-8</id>
  <author><name>Jane Austen</name></author>
  <dcterms:issued>1817-12-20T00:00:00Z</dcterms:issued>
  <dcterms:language>en</dcterms:language>
  <summary type="text">Anne Elliot</summary>
  <link rel="http://opds-spec.org/acquisition/open-access" type="application/epub+zip" href="/b/1.epub" length="1024"/>
  <link rel="http://opds-spec.org/acquisition/buy" type="application/pdf" href="/buy"/>
  <link rel="http://opds-spec.org/acquisition" type="application/x-mobipocket-ebook" href="/b/1.mobi"/>
  <link rel="http://opds-spec.org/image" type="image/jpeg" href="/c/1.jpg"/>
 </entry>
 <entry><title>Poetry</title><id>nav:poetry</id><link rel="subsection" type="application/atom+xml;profile=opds-catalog" href="poetry"/></entry>
</feed>`
	f, err := Parse(strings.NewReader(feed), "https://example.org/opds/root")
	if err != nil {
		t.Fatal(err)
	}
	if f.Next() != "https://example.org/opds/root?page=2" || f.Search().Href != "https://example.org/search?q={searchTerms}" {
		t.Errorf("links next %q search %q", f.Next(), f.Search().Href)
	}
	r, ok := EntryResult(f.Entries[0])
	if !ok || len(r.Formats) != 2 || r.Formats[0].URL != "https://example.org/b/1.epub" || r.Formats[1].Format != "MOBI" ||
		r.Pubdate != "1817-12-20" || r.Identifiers["isbn"] != "9780141439688" || r.Cover != "https://example.org/c/1.jpg" {
		t.Errorf("result %+v", r)
	}
	if nav := f.Entries[1].Navigation(); nav == nil || nav.Href != "https://example.org/opds/poetry" {
		t.Errorf("navigation %+v", nav)
	}
	if got := FillTemplate("https://x/s?q={searchTerms}&p={startPage?}&l={language?}", "le guin"); got != "https://x/s?q=le+guin&p=1&l=*" {
		t.Errorf("template %s", got)
	}
	if _, err := Parse(strings.NewReader(`{"metadata":{}}`), "https://x/"); err == nil {
		t.Error("OPDS 2 parsed")
	}
	if _, err := Parse(strings.NewReader(`<html><body>login</body></html>`), "https://x/"); err == nil {
		t.Error("a web page parsed")
	}
}
