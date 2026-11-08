package plugins

import (
	"context"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"dalibre/internal/plugin"
)

// serve answers every host's requests with h, as if the plugin reached
// the real ones.
type serve struct{ h http.Handler }

func (s serve) RoundTrip(r *http.Request) (*http.Response, error) {
	rec := &recorder{header: http.Header{}, code: 200}
	s.h.ServeHTTP(rec, r)
	return &http.Response{StatusCode: rec.code, Status: http.StatusText(rec.code), Header: rec.header,
		Body: nopCloser{strings.NewReader(rec.body.String())}, Request: r, ContentLength: int64(rec.body.Len())}, nil
}

type recorder struct {
	header http.Header
	body   strings.Builder
	code   int
}

func (r *recorder) Header() http.Header         { return r.header }
func (r *recorder) Write(b []byte) (int, error) { return r.body.Write(b) }
func (r *recorder) WriteHeader(code int)        { r.code = code }

type nopCloser struct{ *strings.Reader }

func (nopCloser) Close() error { return nil }

const searchFeed = `<?xml version="1.0" encoding="utf-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
<link rel="self" href="/ebooks/search.opds/?query=frankenstein"/>
<link rel="next" title="Next Page" href="/ebooks/search.opds/?query=frankenstein&amp;start_index=26"/>
<entry><id>https://www.gutenberg.org/ebooks/authors/search.opds/?query=frankenstein</id><title>Authors</title>
<content type="text">2 author names match your search.</content></entry>
<entry><id>https://www.gutenberg.org/ebooks/84.opds</id><title>Frankenstein; Or, The Modern Prometheus</title>
<content type="text">Mary Wollstonecraft Shelley</content></entry>
<entry><id>https://www.gutenberg.org/ebooks/67976.opds</id><title>故事新讀本 (Chinese)</title>
<content type="text">May 2, 2022</content></entry>
</feed>`

const bookFeed = `<?xml version="1.0" encoding="utf-8"?>
<feed xmlns="http://www.w3.org/2005/Atom" xmlns:dcterms="http://purl.org/dc/terms/">
<entry><title>Frankenstein</title>
<link type="application/epub+zip" rel="http://opds-spec.org/acquisition" length="294175" href="https://www.gutenberg.org/ebooks/84.epub.noimages"/>
</entry>
<entry><title>Frankenstein; or, The modern Prometheus</title>
<content type="xhtml"><div xmlns="http://www.w3.org/1999/xhtml">
<p>Summary:
A scientist builds a creature &amp; abandons it. (This is an automatically generated summary.)
</p>
<p>Downloads: 204693</p>
</div></content>
<published>1993-10-01T00:00:00+00:00</published>
<rights>Public domain in the USA.</rights>
<author><name>Shelley, Mary Wollstonecraft</name></author>
<category scheme="http://purl.org/dc/terms/LCSH" term="Monsters -- Fiction"/>
<category scheme="http://purl.org/dc/terms/LCSH" term="Gothic fiction"/>
<dcterms:language>en</dcterms:language>
<link type="application/epub+zip" rel="http://opds-spec.org/acquisition" length="440131" href="https://www.gutenberg.org/ebooks/84.epub3.images"/>
<link type="application/x-mobipocket-ebook" rel="http://opds-spec.org/acquisition" length="547590" href="https://www.gutenberg.org/ebooks/84.kf8.images"/>
<link type="application/x-mobipocket-ebook" rel="http://opds-spec.org/acquisition" length="519544" href="https://www.gutenberg.org/ebooks/84.kindle.images"/>
</entry>
</feed>`

func TestGutenberg(t *testing.T) {
	var gotQuery string
	mux := http.NewServeMux()
	mux.HandleFunc("www.gutenberg.org/ebooks/search.opds/", func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Write([]byte(searchFeed))
	})
	mux.HandleFunc("www.gutenberg.org/ebooks/84.opds", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(bookFeed)) })
	mux.HandleFunc("www.gutenberg.org/ebooks/84.epub3.images", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/epub+zip")
		w.Write([]byte("PK epub"))
	})
	all := Builtin()
	if len(all) != 1 {
		t.Fatalf("%d built-in plugins", len(all))
	}
	p, err := plugin.LoadBuiltin(all[0], plugin.Options{Transport: serve{mux}})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if p.Manifest.ID != "dalibre.gutenberg" || !p.Manifest.HasCollections() {
		t.Fatalf("manifest %+v", p.Manifest)
	}
	ctx := context.Background()

	page, err := p.Search(ctx, plugin.Query{Text: "frankenstein", Page: 2, Filters: map[string]string{"language": "zh", "sort": "new"}})
	if err != nil {
		t.Fatal(err)
	}
	if gotQuery != "sort_order=release_date&start_index=26&query=frankenstein%20l.zh" {
		t.Errorf("query %s", gotQuery)
	}
	if len(page.Items) != 2 || !page.HasMore || len(page.Warnings) != 0 {
		t.Fatalf("page %+v", page)
	}
	r := page.Items[0]
	if r.ID != "84" || r.Authors[0] != "Mary Wollstonecraft Shelley" || len(r.Formats) != 3 || r.Identifiers["gutenberg"] != "84" ||
		r.URL != "https://www.gutenberg.org/ebooks/84" || !strings.HasSuffix(r.Cover, "/pg84.cover.medium.jpg") {
		t.Fatalf("result %+v", r)
	}
	if other := page.Items[1]; other.Title != "故事新讀本" || len(other.Authors) != 0 {
		t.Errorf("titled %q by %q", other.Title, other.Authors)
	}

	if _, err := p.List(ctx, plugin.Query{Collection: "popular"}); err != nil || gotQuery != "sort_order=downloads&start_index=1" {
		t.Errorf("list: %v, query %s", err, gotQuery)
	}

	full, err := p.Get(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	if full.Title != "Frankenstein; or, The modern Prometheus" || full.Language != "en" || full.Description != "A scientist builds a creature & abandons it." ||
		len(full.Tags) != 2 || full.Tags[0] != "Monsters — Fiction" || full.Authors[0] != "Mary Wollstonecraft Shelley" {
		t.Fatalf("get %+v", full)
	}
	if len(full.Formats) != 3 || full.Formats[0].Format != "EPUB" || full.Formats[0].Size != 440131 || full.Formats[1].Format != "AZW3" {
		t.Fatalf("formats %+v", full.Formats)
	}
	if len(full.Fields) != 3 || full.Fields[1].Value != "204,693" || full.Fields[2].Value != "Public domain in the USA" {
		t.Fatalf("fields %+v", full.Fields)
	}

	path, err := p.Download(ctx, full, "EPUB", t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Ext(path) != ".epub" {
		t.Fatalf("path %s", path)
	}
}
