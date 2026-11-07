package calibre

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

func esc(s string) string {
	var b strings.Builder
	xml.EscapeText(&b, []byte(s))
	return b.String()
}

// OPF returns the book's metadata.opf as calibre writes it, which calibre
// reads to rebuild a library's database.
func (l *Library) OPF(b *Book, d *Details) []byte {
	var w strings.Builder
	w.WriteString(`<?xml version='1.0' encoding='utf-8'?>
<package xmlns="http://www.idpf.org/2007/opf" unique-identifier="uuid_id" version="2.0">
    <metadata xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:opf="http://www.idpf.org/2007/opf">
`)
	line := func(format string, args ...any) { fmt.Fprintf(&w, "        "+format+"\n", args...) }
	line(`<dc:identifier opf:scheme="calibre" id="calibre_id">%d</dc:identifier>`, b.ID)
	line(`<dc:identifier opf:scheme="uuid" id="uuid_id">%s</dc:identifier>`, esc(b.UUID))
	line(`<dc:title>%s</dc:title>`, esc(b.Title))
	for _, a := range b.Authors {
		line(`<dc:creator opf:file-as="%s" opf:role="aut">%s</dc:creator>`, esc(AuthorSort(a)), esc(a))
	}
	line(`<dc:contributor opf:file-as="calibre" opf:role="bkp">Dalibre</dc:contributor>`)
	if !b.Pubdate.IsZero() {
		line(`<dc:date>%s</dc:date>`, b.Pubdate.UTC().Format("2006-01-02T15:04:05-07:00"))
	}
	if d != nil && d.Comments != "" {
		line(`<dc:description>%s</dc:description>`, esc(d.Comments))
	}
	if b.Publisher != "" {
		line(`<dc:publisher>%s</dc:publisher>`, esc(b.Publisher))
	}
	if d != nil {
		keys := make([]string, 0, len(d.Identifiers))
		for k := range d.Identifiers {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			line(`<dc:identifier opf:scheme="%s">%s</dc:identifier>`, esc(strings.ToUpper(k)), esc(d.Identifiers[k]))
		}
	}
	for _, lang := range b.Languages {
		line(`<dc:language>%s</dc:language>`, esc(lang))
	}
	for _, t := range b.Tags {
		line(`<dc:subject>%s</dc:subject>`, esc(t))
	}
	line(`<meta name="calibre:author_link_map" content="{}"/>`)
	if b.Series != "" {
		line(`<meta name="calibre:series" content="%s"/>`, esc(b.Series))
		line(`<meta name="calibre:series_index" content="%s"/>`, strconv.FormatFloat(b.SeriesIndex, 'f', -1, 64))
	}
	if b.Rating > 0 {
		line(`<meta name="calibre:rating" content="%d"/>`, b.Rating)
	}
	line(`<meta name="calibre:timestamp" content="%s"/>`, b.Timestamp.UTC().Format("2006-01-02T15:04:05-07:00"))
	line(`<meta name="calibre:title_sort" content="%s"/>`, esc(b.Sort))
	w.WriteString("    </metadata>\n    <guide>\n")
	if b.HasCover {
		w.WriteString(`        <reference type="cover" title="Cover" href="cover.jpg"/>` + "\n")
	}
	w.WriteString("    </guide>\n</package>\n")
	return []byte(w.String())
}

// writeOPFBestEffort refreshes metadata.opf; the database stays the
// source of truth, and calibre rewrites dirtied books anyway, so a failure
// here is not an error of the edit.
func (l *Library) writeOPFBestEffort(id int64) {
	b, err := l.Book(id)
	if err != nil {
		return
	}
	d, err := l.Details(b)
	if err != nil {
		return
	}
	dir := l.Dir(b)
	if _, err := os.Stat(dir); err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(dir, "metadata.opf"), l.OPF(b, d), 0o644)
}
