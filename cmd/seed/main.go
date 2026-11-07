// Command seed creates a sample calibre library for development:
//
//	go run ./cmd/seed ./testdata/Sample\ Library
package main

import (
	"archive/zip"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dalibre/internal/calibre"
)

type sample struct {
	title, author, series string
	index                 float64
	tags                  []string
	formats               []string
	rating                int
	year                  int
	lang, publisher       string
}

var samples = []sample{
	{"The Three-Body Problem", "Liu Cixin", "Remembrance of Earth’s Past", 1, []string{"Science Fiction", "Hugo Award"}, []string{"epub", "mobi"}, 8, 2008, "eng", "Tor Books"},
	{"The Dark Forest", "Liu Cixin", "Remembrance of Earth’s Past", 2, []string{"Science Fiction"}, []string{"epub"}, 10, 2008, "eng", "Tor Books"},
	{"Death’s End", "Liu Cixin", "Remembrance of Earth’s Past", 3, []string{"Science Fiction"}, []string{"epub", "azw3"}, 8, 2010, "eng", "Tor Books"},
	{"Foundation", "Isaac Asimov", "Foundation", 1, []string{"Science Fiction", "Classics"}, []string{"epub", "mobi"}, 8, 1951, "eng", "Gnome Press"},
	{"Dune", "Frank Herbert", "Dune", 1, []string{"Science Fiction", "Classics"}, []string{"epub", "pdf"}, 10, 1965, "eng", "Chilton Books"},
	{"The Left Hand of Darkness", "Ursula K. Le Guin", "", 0, []string{"Science Fiction", "Literary"}, []string{"epub"}, 8, 1969, "eng", "Ace Books"},
	{"红楼梦", "曹雪芹", "", 0, []string{"Classics", "Chinese Literature"}, []string{"epub"}, 10, 1791, "zho", ""},
	{"三体", "刘慈欣", "地球往事", 1, []string{"科幻"}, []string{"epub"}, 10, 2008, "zho", "重庆出版社"},
	{"Piranesi", "Susanna Clarke", "", 0, []string{"Literary", "Fantasy"}, []string{"epub", "mobi"}, 8, 2020, "eng", "Bloomsbury"},
	{"Invisible Cities", "Italo Calvino", "", 0, []string{"Literary", "Classics"}, []string{"pdf"}, 10, 1972, "eng", "Einaudi"},
	{"Thinking, Fast and Slow", "Daniel Kahneman", "", 0, []string{"Non-fiction", "Psychology"}, []string{"pdf"}, 6, 2011, "eng", "Farrar, Straus and Giroux"},
	{"The Name of the Rose", "Umberto Eco", "", 0, []string{"Literary", "Mystery"}, []string{"epub"}, 8, 1980, "eng", "Bompiani"},
	{"Klara and the Sun", "Kazuo Ishiguro", "", 0, []string{"Literary", "Science Fiction"}, []string{"epub", "mobi"}, 6, 2021, "eng", "Faber and Faber"},
	{"Neuromancer", "William Gibson", "Sprawl", 1, []string{"Science Fiction", "Cyberpunk"}, []string{"mobi"}, 8, 1984, "eng", "Ace Books"},
	{"Structure and Interpretation of Computer Programs", "Harold Abelson", "", 0, []string{"Non-fiction", "Programming"}, []string{"pdf"}, 10, 1985, "eng", "MIT Press"},
	{"Snow Crash", "Neal Stephenson", "", 0, []string{"Science Fiction", "Cyberpunk"}, []string{"epub"}, 6, 1992, "eng", "Bantam Books"},
	{"Meditations", "Marcus Aurelius", "", 0, []string{"Philosophy", "Classics"}, []string{"epub", "pdf"}, 8, 180, "eng", ""},
	{"The Wandering Earth", "Liu Cixin", "", 0, []string{"Science Fiction", "Short Stories"}, []string{"epub"}, 8, 2000, "eng", "Head of Zeus"},
	{"Ball Lightning", "Liu Cixin", "", 0, []string{"Science Fiction"}, []string{"epub", "pdf"}, 6, 2004, "eng", "Tor Books"},
	{"Gödel, Escher, Bach", "Douglas Hofstadter", "", 0, []string{"Non-fiction", "Philosophy"}, []string{"pdf"}, 10, 1979, "eng", "Basic Books"},
}

func main() {
	if len(os.Args) != 2 {
		log.Fatal("usage: seed <library folder>")
	}
	root := os.Args[1]
	lib, err := calibre.Create(root)
	if err != nil {
		log.Fatal(err)
	}
	defer lib.Close()
	tmp, err := os.MkdirTemp("", "dalibre-seed")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(tmp)
	for i, s := range samples {
		first := filepath.Join(tmp, fmt.Sprintf("%d.%s", i, s.formats[0]))
		if err := writeBook(first, s); err != nil {
			log.Fatal(err)
		}
		pub := time.Date(max(s.year, 102), 1, 1, 0, 0, 0, 0, time.UTC)
		id, err := lib.AddBook(calibre.NewBook{File: first, Metadata: calibre.Metadata{
			Title: s.title, Authors: []string{s.author}, Series: s.series, SeriesIndex: s.index, Tags: s.tags,
			Rating: s.rating, Pubdate: pub, Languages: []string{s.lang}, Publisher: s.publisher,
			Comments: fmt.Sprintf("<p>%s by %s. A sample book created by Dalibre's seed command.</p>", s.title, s.author),
		}})
		if err != nil {
			log.Fatal(err)
		}
		for _, f := range s.formats[1:] {
			p := filepath.Join(tmp, fmt.Sprintf("%d.%s", i, f))
			if err := writeBook(p, s); err != nil {
				log.Fatal(err)
			}
			if err := lib.AddFormat(id, p); err != nil {
				log.Fatal(err)
			}
		}
	}
	fmt.Printf("created %s with %d books\n", root, len(samples))
}

// writeBook writes a small but valid EPUB, or placeholder bytes for the
// other formats (enough for the library; not readable books).
func writeBook(path string, s sample) error {
	if !strings.HasSuffix(path, ".epub") {
		return os.WriteFile(path, []byte(strings.Repeat(s.title+"\n", 4000)), 0o644)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	zw := zip.NewWriter(f)
	w, _ := zw.CreateHeader(&zip.FileHeader{Name: "mimetype", Method: zip.Store})
	w.Write([]byte("application/epub+zip"))
	files := map[string]string{
		"META-INF/container.xml": `<?xml version="1.0"?><container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`,
		"OEBPS/content.opf":      fmt.Sprintf(`<?xml version="1.0"?><package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="id"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:identifier id="id">urn:uuid:%d</dc:identifier><dc:title>%s</dc:title><dc:creator>%s</dc:creator><dc:language>%s</dc:language></metadata><manifest><item id="c1" href="c1.xhtml" media-type="application/xhtml+xml"/></manifest><spine><itemref idref="c1"/></spine></package>`, len(s.title), s.title, s.author, s.lang),
		"OEBPS/c1.xhtml":         fmt.Sprintf(`<?xml version="1.0"?><html xmlns="http://www.w3.org/1999/xhtml"><head><title>%s</title></head><body><h1>%s</h1><p>%s</p></body></html>`, s.title, s.title, strings.Repeat("Lorem ipsum dolor sit amet. ", 400)),
	}
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			return err
		}
		w.Write([]byte(content))
	}
	if err := zw.Close(); err != nil {
		return err
	}
	return f.Close()
}
