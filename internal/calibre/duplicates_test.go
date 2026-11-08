package calibre

import (
	"path/filepath"
	"testing"
)

func TestDuplicates(t *testing.T) {
	l := newTestLibrary(t)
	epub := writeTemp(t, "a.epub", "first epub")
	id, err := l.AddBook(NewBook{File: epub, Metadata: Metadata{
		Title: "The Three-Body Problem", Authors: []string{"Liu Cixin", "Ken Liu"},
		Identifiers: map[string]string{"isbn": "978-0-7653-8203-0"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	d, err := l.Duplicates()
	if err != nil {
		t.Fatal(err)
	}
	prep := func(file, title, author, isbn string) Prepared {
		p := Prepared{File: file, Format: "EPUB", Metadata: Metadata{Title: title, Authors: []string{author}, Identifiers: map[string]string{}}}
		if filepath.Ext(file) == ".pdf" {
			p.Format = "PDF"
		}
		if isbn != "" {
			p.Metadata.Identifiers["isbn"] = isbn
		}
		return p
	}
	other := writeTemp(t, "b.epub", "another epub")
	pdf := writeTemp(t, "c.pdf", "a pdf")

	for _, tc := range []struct {
		name             string
		p                Prepared
		match, has, same bool
	}{
		{"same file", prep(epub, "The Three-Body Problem", "Liu Cixin", ""), true, true, true},
		{"loose title, second author", prep(other, "the three body  problem!", "KEN LIU", ""), true, true, false},
		{"new format", prep(pdf, "The Three-Body Problem", "Liu Cixin", ""), true, false, false},
		{"by ISBN", prep(pdf, "三体", "刘慈欣", "9780765382030"), true, false, false},
		{"other book", prep(pdf, "The Dark Forest", "Liu Cixin", ""), false, false, false},
	} {
		dup, err := d.Find(tc.p)
		if err != nil {
			t.Fatal(tc.name, err)
		}
		if (dup != nil) != tc.match {
			t.Errorf("%s: match = %v", tc.name, dup != nil)
			continue
		}
		if dup != nil && (dup.Book.ID != id || dup.HasFormat != tc.has || dup.SameFile != tc.same) {
			t.Errorf("%s: got book %d has %v same %v", tc.name, dup.Book.ID, dup.HasFormat, dup.SameFile)
		}
	}

	// A book added during the import is found for a later copy.
	p := prep(pdf, "The Dark Forest", "Liu Cixin", "")
	id2, err := l.Add(p)
	if err != nil {
		t.Fatal(err)
	}
	d.Added(id2, p.Metadata)
	if dup, _ := d.Find(p); dup == nil || dup.Book.ID != id2 || !dup.SameFile {
		t.Errorf("added book not found: %+v", dup)
	}
}
