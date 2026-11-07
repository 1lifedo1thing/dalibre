package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"dalibre/internal/calibre"
	"dalibre/internal/catalog"
)

// TestRealLibrary opens the calibre library DALIBRE_REAL_LIBRARY names,
// read-only, and walks through the main views. It never writes to it.
func TestRealLibrary(t *testing.T) {
	root := os.Getenv("DALIBRE_REAL_LIBRARY")
	if root == "" {
		t.Skip("DALIBRE_REAL_LIBRARY not set")
	}
	db := filepath.Join(root, "metadata.db")
	before, err := os.Stat(db)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		after, err := os.Stat(db)
		if err != nil || !after.ModTime().Equal(before.ModTime()) || after.Size() != before.Size() {
			t.Errorf("metadata.db changed")
		}
		for _, j := range []string{"-journal", "-wal", "-shm"} {
			if _, err := os.Stat(db + j); err == nil {
				t.Errorf("left %s behind", filepath.Base(db+j))
			}
		}
	})

	a := New("")
	a.run = func(fn func()) { fn() }
	a.trash = nil
	a.covers.thumbs = nil
	a.open = calibre.OpenReadOnly
	a.openLibrary(root)
	if a.lib == nil {
		t.Fatalf("library not open: %s", a.loadErr)
	}
	t.Cleanup(func() { a.lib.Close() })

	formats := map[string]int{}
	for _, f := range a.counts.Formats {
		formats[f.Value] = f.N
	}
	t.Logf("%d books; formats %v", len(a.books), formats)
	if formats["PDF"] == 0 {
		t.Errorf("no PDF books counted")
	}

	tt := ui.NewTester(a.View, 1440, 900)
	tt.Frame()
	shot(t, tt, "real-grid")

	a.query.Filter = catalog.Filter{Kind: catalog.ByFormat, Value: "PDF"}
	tt.Frame()
	pdfs := a.visible()
	if len(pdfs) != formats["PDF"] {
		t.Errorf("PDF filter shows %d of %d", len(pdfs), formats["PDF"])
	}
	shot(t, tt, "real-pdf")

	// A PDF book's details list its files on disk.
	if len(pdfs) > 0 {
		b := pdfs[0]
		if err := tt.Click(b.Title); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		tt.Frame()
		d := a.details[b.ID]
		if d == nil {
			t.Fatalf("no details for %q", b.Title)
		}
		for _, f := range d.Files {
			if _, err := os.Stat(f.Path); err != nil {
				t.Errorf("%s: %v", f.Format, err)
			}
		}
		shot(t, tt, "real-pdf-detail")
	}

	a.query.Filter = catalog.Filter{}
	a.settings.ListView = true
	tt.Frame()
	// The book chosen under the filter stays chosen, in its new row.
	if b := a.current(); b != nil && (a.cur < 0 || a.visible()[a.cur] != b) {
		t.Errorf("table highlights row %d, panel shows %q", a.cur, b.Title)
	}
	shot(t, tt, "real-list")
}
