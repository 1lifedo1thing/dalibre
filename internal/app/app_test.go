package app

import (
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"dalibre/internal/app/theme"
	"dalibre/internal/calibre"
	"dalibre/internal/catalog"
)

func TestMain(m *testing.M) {
	if err := theme.RegisterFonts(); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

type sample struct {
	title, author string
	tags          []string
	rating        int
}

var samples = []sample{
	{"The Three-Body Problem", "Liu Cixin", []string{"Science Fiction"}, 8},
	{"The Dark Forest", "Liu Cixin", []string{"Science Fiction"}, 10},
	{"Dune", "Frank Herbert", []string{"Classics", "Science Fiction"}, 10},
	{"Piranesi", "Susanna Clarke", []string{"Fantasy"}, 8},
	{"红楼梦", "曹雪芹", []string{"Classics"}, 10},
	{"Invisible Cities", "Italo Calvino", []string{"Literary"}, 6},
}

// newTestApp opens a library of sample books, running background work
// synchronously so the view sees its results at once.
func newTestApp(t *testing.T) *App {
	t.Helper()
	root := filepath.Join(t.TempDir(), "Library")
	lib, err := calibre.Create(root)
	if err != nil {
		t.Fatal(err)
	}
	for i, s := range samples {
		f := filepath.Join(t.TempDir(), "book.epub")
		os.WriteFile(f, []byte("x"), 0o644)
		if _, err := lib.AddBook(calibre.NewBook{File: f, Metadata: calibre.Metadata{
			Title: s.title, Authors: []string{s.author}, Tags: s.tags, Rating: s.rating,
			Pubdate: time.Date(2000+i, 1, 1, 0, 0, 0, 0, time.UTC), Languages: []string{"eng"},
		}}); err != nil {
			t.Fatal(err)
		}
	}
	lib.Close()
	a := New("")
	a.run = func(fn func()) { fn() }
	a.trash = nil
	a.covers.thumbs = nil
	a.openLibrary(root)
	t.Cleanup(func() {
		if a.lib != nil {
			a.lib.Close()
		}
	})
	if a.lib == nil {
		t.Fatalf("library not open: %s", a.loadErr)
	}
	return a
}

// shot saves the tester's frame as a PNG when DALIBRE_SHOTS names a folder.
func shot(t *testing.T, tt *ui.Tester, name string) {
	dir := os.Getenv("DALIBRE_SHOTS")
	if dir == "" {
		return
	}
	f, err := os.Create(filepath.Join(dir, name+".png"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	png.Encode(f, tt.Image())
}

func TestLibraryShowsBooks(t *testing.T) {
	a := newTestApp(t)
	tt := ui.NewTester(a.View, 1440, 900)
	tt.Frame()
	for _, s := range samples {
		if !tt.HasText(s.title) {
			t.Errorf("grid misses %q; texts %q", s.title, tt.Texts())
		}
	}
	if !tt.HasText("All books") || !tt.HasText("Add books") {
		t.Error("toolbar missing")
	}
	shot(t, tt, "grid-light")

	tt.SetDark(true)
	tt.Frame()
	shot(t, tt, "grid-dark")
}

func TestSelectShowsDetails(t *testing.T) {
	a := newTestApp(t)
	tt := ui.NewTester(a.View, 1440, 900)
	tt.Frame()
	if err := tt.Click("Dune"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if b := a.current(); b == nil || b.Title != "Dune" {
		t.Fatalf("current = %v", b)
	}
	if !tt.HasText("Frank Herbert") || !tt.HasText("DETAILS") {
		t.Errorf("detail panel: %q", tt.Texts())
	}
	shot(t, tt, "detail")
}

func TestSearchAndFilter(t *testing.T) {
	a := newTestApp(t)
	tt := ui.NewTester(a.View, 1440, 900)
	tt.Frame()
	if err := tt.Click("Search library"); err != nil {
		t.Fatal(err)
	}
	tt.Type("liu")
	tt.Frame()
	if len(a.visible()) != 2 || tt.HasText("Piranesi") {
		t.Errorf("search shows %d books", len(a.visible()))
	}
	if !tt.HasText("“liu”") || !tt.HasText("Clear all") {
		t.Error("search chip missing")
	}
	a.query.Search = ""
	tt.Frame()
	// The sidebar's format filter.
	if err := tt.Click("EPUB"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.query.Filter.Value != "EPUB" || len(a.visible()) != len(samples) {
		t.Errorf("format filter %+v shows %d", a.query.Filter, len(a.visible()))
	}
	// The author chip's picker.
	if err := tt.Click("Author"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	shot(t, tt, "picker")
	if err := tt.Click("Liu Cixin, 2 books"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.query.Filter != (catalog.Filter{Kind: catalog.ByAuthor, Value: "Liu Cixin"}) || len(a.visible()) != 2 {
		t.Errorf("author filter %+v shows %d", a.query.Filter, len(a.visible()))
	}
	if !tt.HasText("Author: Liu Cixin") {
		t.Errorf("active chip missing: %q", tt.Texts())
	}
	shot(t, tt, "filters")
}

func TestListView(t *testing.T) {
	a := newTestApp(t)
	a.settings.ListView = true
	tt := ui.NewTester(a.View, 1440, 900)
	tt.Frame()
	if !tt.HasText("Author") || !tt.HasText("Rating") || !tt.HasText("Liu Cixin") {
		t.Errorf("table: %q", tt.Texts())
	}
	if err := tt.Click("Dune"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	shot(t, tt, "list")
}

func TestEditMetadata(t *testing.T) {
	a := newTestApp(t)
	tt := ui.NewTester(a.View, 1440, 900)
	tt.Frame()
	tt.Click("Piranesi")
	tt.Frame()
	a.openEditor(a.current())
	tt.Frame()
	if !tt.HasText("Edit metadata") {
		t.Fatalf("editor not open: %q", tt.Texts())
	}
	shot(t, tt, "edit")
	a.edit.title = "Piranesi (Revised)"
	a.edit.tags = append(a.edit.tags, "Favourite")
	if err := tt.Click("Save"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	b := a.byID[a.edit.id]
	if b == nil || b.Title != "Piranesi (Revised)" || len(b.Tags) != 2 {
		t.Fatalf("saved book = %+v", b)
	}
	if _, err := os.Stat(a.lib.Dir(b)); err != nil {
		t.Errorf("book folder not moved: %v", err)
	}
}

func TestConvertDialog(t *testing.T) {
	a := newTestApp(t)
	tt := ui.NewTester(a.View, 1440, 900)
	tt.Frame()
	tt.Click("Dune")
	tt.Frame()
	a.openConvert(a.selectedIDs())
	tt.Frame()
	if !tt.HasText("Convert 1 book") || !tt.HasText("Kindle (legacy)") {
		t.Fatalf("convert dialog: %q", tt.Texts())
	}
	if err := tt.Click("PDF, Fixed layout"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.convert.opts.Output != "PDF" {
		t.Errorf("output = %s", a.convert.opts.Output)
	}
	shot(t, tt, "convert")
	// The button's label is the dialog's title too; start as it does.
	a.startConversions(a.convert.ids, a.convert.opts)
	tt.Frame()
	if len(a.jobs) != 1 || a.jobs[0].Message == "" {
		t.Fatalf("jobs = %+v", a.jobs)
	}
	shot(t, tt, "jobs")
}

func TestRemoveAsksFirst(t *testing.T) {
	a := newTestApp(t)
	tt := ui.NewTester(a.View, 1440, 900)
	tt.Frame()
	if err := tt.RightClick("Invisible Cities"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Remove from library"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !a.deleteOpen {
		t.Fatalf("no confirmation: %q", tt.Texts())
	}
	shot(t, tt, "delete")
	if err := tt.Click("Remove"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(a.books) != len(samples)-1 {
		t.Errorf("books = %d", len(a.books))
	}
}

func TestWelcomeWithoutLibrary(t *testing.T) {
	a := New("")
	tt := ui.NewTester(a.View, 1200, 800)
	tt.Frame()
	if !tt.HasText("Open library…") {
		t.Errorf("welcome: %q", tt.Texts())
	}
	shot(t, tt, "welcome")
}

func TestSelectionFollowsBook(t *testing.T) {
	a := newTestApp(t)
	a.settings.ListView = true
	tt := ui.NewTester(a.View, 1440, 900)
	tt.Frame()
	if err := tt.Click("Dune"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	// Reordering moves Dune to another row; the table and panel follow it.
	a.query.Descending = !a.query.Descending
	tt.Frame()
	if b := a.current(); b == nil || b.Title != "Dune" {
		t.Fatalf("current = %v", b)
	}
	if shown := a.visible(); a.cur < 0 || shown[a.cur].Title != "Dune" {
		t.Errorf("row %d is not Dune", a.cur)
	}
}

func TestLibrarySwitcherMenu(t *testing.T) {
	a := newTestApp(t)
	other := t.TempDir()
	a.settings.Recent = []string{a.lib.Root, other}
	tt := ui.NewTester(a.View, 1440, 900)
	tt.Frame()
	if err := tt.Click("Library"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	for _, r := range a.settings.Recent {
		if !tt.HasText(r) {
			t.Errorf("menu lacks %s: %q", r, tt.Texts())
		}
	}
	// Hovering the other library shows its reveal button.
	r, ok := tt.Find(other)
	if !ok {
		t.Fatal("no row for", other)
	}
	tt.Move(r.X+10, r.Y+r.H/2)
	tt.Frame()
	tt.Frame()
	shot(t, tt, "switcher")
	if _, ok := tt.Find(revealLabel + ": " + filepath.Base(other)); !ok {
		t.Error("no reveal button")
	}
}

func TestShareMenu(t *testing.T) {
	if !shareSupported {
		t.Skip("no share sheet on this platform")
	}
	a := newTestApp(t)
	var shared []string
	saved := shareFiles
	t.Cleanup(func() { shareFiles = saved })
	shareFiles = func(_ *mygo.Window, paths []string) { shared = paths }
	tt := ui.NewTester(a.View, 1440, 900)
	tt.Frame()
	// One format: a plain item.
	if err := tt.RightClick("Dune"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Share…"); err != nil {
		t.Fatal(err)
	}
	if len(shared) != 1 || !strings.HasSuffix(shared[0], ".epub") {
		t.Fatalf("shared = %q", shared)
	}
	// More formats: a submenu of them.
	var dune *calibre.Book
	for _, b := range a.books {
		if b.Title == "Dune" {
			dune = b
		}
	}
	f := filepath.Join(t.TempDir(), "dune.pdf")
	os.WriteFile(f, []byte("x"), 0o644)
	if err := a.lib.AddFormat(dune.ID, f); err != nil {
		t.Fatal(err)
	}
	dune.Formats = append(dune.Formats, "PDF")
	tt.Frame()
	if err := tt.RightClick("Dune"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Share", "PDF"); err != nil {
		t.Fatal(err)
	}
	if len(shared) != 1 || !strings.HasSuffix(shared[0], ".pdf") {
		t.Fatalf("shared = %q", shared)
	}
}
