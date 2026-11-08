package calibre

import (
	"bytes"
	"crypto/sha256"
	"io"
	"os"
	"strings"
	"unicode"
)

// Duplicate is a book in the library that a file being added matches.
type Duplicate struct {
	Book      *Book
	HasFormat bool // the book has a file in the new file's format
	SameFile  bool // and that file is byte for byte the new one
}

// Duplicates finds the books a library has already, by title and author
// or by ISBN, for files about to be added.
type Duplicates struct {
	l      *Library
	byName map[string]int64 // dupKey(title, author) → book
	byISBN map[string]int64
}

// Duplicates indexes the library's books for Find.
func (l *Library) Duplicates() (*Duplicates, error) {
	books, err := l.Books()
	if err != nil {
		return nil, err
	}
	ids, err := l.Identifiers()
	if err != nil {
		return nil, err
	}
	d := &Duplicates{l: l, byName: map[string]int64{}, byISBN: map[string]int64{}}
	for _, b := range books {
		d.addNames(b.ID, b.Title, b.Authors)
	}
	for k, id := range ids {
		if t, v, _ := strings.Cut(k, ":"); t == "isbn" {
			if isbn := digits(v); isbn != "" {
				d.byISBN[isbn] = id
			}
		}
	}
	return d, nil
}

// Added records a book added since the index was made, so that a second
// copy in the same import is found too.
func (d *Duplicates) Added(id int64, m Metadata) {
	m.normalize()
	d.addNames(id, m.Title, m.Authors)
	if isbn := digits(m.Identifiers["isbn"]); isbn != "" {
		d.byISBN[isbn] = id
	}
}

func (d *Duplicates) addNames(id int64, title string, authors []string) {
	if len(authors) == 0 {
		authors = []string{"Unknown"}
	}
	for _, a := range authors {
		d.byName[dupKey(title, a)] = id
	}
}

// Find returns the book p duplicates, or nil.
func (d *Duplicates) Find(p Prepared) (*Duplicate, error) {
	m := p.Metadata
	m.normalize()
	id := int64(0)
	if isbn := digits(m.Identifiers["isbn"]); isbn != "" {
		id = d.byISBN[isbn]
	}
	for _, a := range m.Authors {
		if id != 0 {
			break
		}
		id = d.byName[dupKey(m.Title, a)]
	}
	if id == 0 {
		return nil, nil
	}
	b, err := d.l.Book(id)
	if err != nil {
		return nil, err
	}
	dup := &Duplicate{Book: b}
	det, err := d.l.Details(b)
	if err != nil {
		return nil, err
	}
	for _, f := range det.Files {
		if f.Format == p.Format {
			dup.HasFormat = true
			dup.SameFile = sameFile(f.Path, p.File)
		}
	}
	return dup, nil
}

// dupKey is a title and an author with case, punctuation and spacing
// left out, so "The Three-Body Problem" by "Liu  Cixin" matches
// "the three body problem" by "liu cixin".
func dupKey(title, author string) string {
	return fold(title) + "\x00" + fold(author)
}

func fold(s string) string {
	var b strings.Builder
	space := false
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			if space && b.Len() > 0 {
				b.WriteByte(' ')
			}
			b.WriteRune(r)
			space = false
		} else {
			space = true
		}
	}
	return b.String()
}

func digits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' || r == 'x' || r == 'X' {
			b.WriteRune(unicode.ToUpper(r))
		}
	}
	return b.String()
}

// sameFile reports whether two files hold the same bytes.
func sameFile(a, b string) bool {
	sa, err1 := os.Stat(a)
	sb, err2 := os.Stat(b)
	if err1 != nil || err2 != nil || sa.Size() != sb.Size() {
		return false
	}
	ha, hb := fileHash(a), fileHash(b)
	return ha != nil && bytes.Equal(ha, hb)
}

func fileHash(path string) []byte {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return nil
	}
	return h.Sum(nil)
}
