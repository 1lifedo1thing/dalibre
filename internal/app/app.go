package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"dalibre/internal/calibre"
	"dalibre/internal/catalog"
	"dalibre/internal/convert"
	"dalibre/internal/settings"
)

// App is the state the window shows. Every widgets.Field is read and written on
// the main thread; work on disk runs in goroutines that hand their results
// back through post.
type App struct {
	win  *mygo.Window
	post func(func()) // runs fn on the main thread and rebuilds
	run  func(func()) // runs fn in the background
	// trash takes removed books' folders; nil keeps them in the library's
	// trash folder, as tests do.
	trash func(path string) error
	// open opens a library; tests open real libraries read-only.
	open func(root string) (*calibre.Library, error)

	settings *settings.Settings

	lib       *calibre.Library
	loading   bool
	loadErr   string
	books     []*calibre.Book
	byID      map[int64]*calibre.Book
	counts    catalog.Counts
	version   int // bumps when books change, to recompute shown
	booksTime time.Time

	query    catalog.Query
	shown    []*calibre.Book
	shownKey shownKey
	// curID is the book the detail panel shows. cur is its row in shown,
	// which the grid and table move; curAt and curList are what cur was last
	// set to and for which rows, so a move is told from a stale index.
	curID      int64
	cur        int
	curAt      int
	curList    []*calibre.Book
	chosen     ui.Selection[int64]
	grid       ui.GridState
	table      ui.ListState
	tableSort  ui.SortOrder
	sectionsOn map[string]bool

	details    map[int64]*calibre.Details
	detailsReq map[int64]bool
	covers     *coverCache

	// dialogs
	editOpen      bool
	edit          editState
	convertOpen   bool
	convert       convertState
	deleteOpen    bool
	deleteIDs     []int64
	createAsk     bool
	createAskPath string
	jobsOpen      bool
	jobs          []*convert.Job

	busy        string // a long action in progress, as an import
	busyDone    int
	busyTotal   int
	toasts      []toastMsg
	searchFocus ui.Handle
}

type toastMsg struct {
	text  string
	error bool
}

type shownKey struct {
	q       catalog.Query
	version int
}

// New returns the app with the settings at settingsPath ("" keeps them
// in memory, as tests do).
func New(settingsPath string) *App {
	a := &App{
		cur:        -1,
		curAt:      -1,
		byID:       map[int64]*calibre.Book{},
		details:    map[int64]*calibre.Details{},
		detailsReq: map[int64]bool{},
		sectionsOn: map[string]bool{"formats": true},
		covers:     newCoverCache(),
		settings:   settings.Load(settingsPath),
	}
	a.post = func(fn func()) { fn() }
	a.run = func(fn func()) { go fn() }
	a.trash = mygo.Shell.TrashItem
	a.open = calibre.Open
	a.query.Sort = catalog.SortKey(a.settings.Sort)
	a.query.Descending = a.settings.Descending
	return a
}

// Attach shows the app in win: work done in the background hands its
// results to the window's main thread. It opens the startup library.
func (a *App) Attach(win *mygo.Window) {
	a.win = win
	a.post = win.Update
	a.openLibrary(a.startupLibrary())
}

// View builds the window from the state, every frame.
func (a *App) View(c *ui.Context) { a.view(c) }

func (a *App) saveSettings() {
	a.settings.Sort, a.settings.Descending = int(a.query.Sort), a.query.Descending
	_ = a.settings.Save()
}

func (a *App) toast(format string, args ...any) {
	a.toasts = append(a.toasts, toastMsg{text: fmt.Sprintf(format, args...)})
}

func (a *App) toastError(err error) {
	a.toasts = append(a.toasts, toastMsg{text: err.Error(), error: true})
}

// startupLibrary picks the library to open: the one used last, then
// DALIBRE_LIBRARY, then calibre's own choice.
func (a *App) startupLibrary() string {
	if p := os.Getenv("DALIBRE_LIBRARY"); p != "" {
		return p
	}
	if a.settings.Library != "" {
		return a.settings.Library
	}
	return calibreLibraryPath()
}

// calibreLibraryPath reads the library calibre last used from its
// global.py.json, if calibre is installed.
func calibreLibraryPath() string {
	var dirs []string
	if d, err := os.UserConfigDir(); err == nil {
		dirs = append(dirs, filepath.Join(d, "calibre"))
	}
	if h, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(h, ".config", "calibre"), filepath.Join(h, "Library", "Preferences", "calibre"))
	}
	for _, d := range dirs {
		data, err := os.ReadFile(filepath.Join(d, "global.py.json"))
		if err != nil {
			continue
		}
		var g struct {
			LibraryPath string `json:"library_path"`
		}
		if json.Unmarshal(data, &g) == nil && g.LibraryPath != "" {
			return g.LibraryPath
		}
	}
	return ""
}

// openLibrary opens root in the background and shows it.
func (a *App) openLibrary(root string) {
	if root == "" {
		return
	}
	a.loading, a.loadErr = true, ""
	a.run(func() {
		lib, err := a.open(root)
		var books []*calibre.Book
		if err == nil {
			books, err = lib.Books()
		}
		a.post(func() {
			a.loading = false
			if err != nil {
				if lib != nil {
					lib.Close()
				}
				if errors.Is(err, calibre.ErrNotLibrary) {
					a.createAsk, a.createAskPath = true, root
					return
				}
				a.loadErr = fmt.Sprintf("Couldn’t open %s: %v", root, err)
				return
			}
			if a.lib != nil {
				a.lib.Close()
			}
			a.lib = lib
			a.details, a.detailsReq = map[int64]*calibre.Details{}, map[int64]bool{}
			a.curID, a.chosen = 0, ui.Selection[int64]{}
			a.grid, a.table = ui.GridState{}, ui.ListState{}
			a.query.Filter, a.query.Search, a.query.Shelf = catalog.Filter{}, "", catalog.AllBooks
			a.setBooks(books)
			a.settings.Opened(root)
			a.saveSettings()
			if a.win != nil {
				a.win.SetTitle("Dalibre — " + filepath.Base(root))
			}
		})
	})
}

func (a *App) createLibrary(root string) {
	a.run(func() {
		lib, err := calibre.Create(root)
		if err == nil {
			lib.Close()
		}
		a.post(func() {
			if err != nil {
				a.toastError(err)
				return
			}
			a.openLibrary(root)
		})
	})
}

func (a *App) setBooks(books []*calibre.Book) {
	a.books = books
	a.byID = make(map[int64]*calibre.Book, len(books))
	for _, b := range books {
		a.byID[b.ID] = b
	}
	a.booksTime = time.Now()
	a.counts = catalog.Tally(books, a.booksTime)
	a.version++
}

// reload reads the books again after a change, keeping the selection.
func (a *App) reload(after func()) {
	lib := a.lib
	if lib == nil {
		return
	}
	a.run(func() {
		books, err := lib.Books()
		a.post(func() {
			if lib != a.lib {
				return
			}
			if err != nil {
				a.toastError(err)
				return
			}
			a.details, a.detailsReq = map[int64]*calibre.Details{}, map[int64]bool{}
			a.setBooks(books)
			if after != nil {
				after()
			}
		})
	})
}

// visible returns the books the query selects, recomputed only when the
// query or the books change.
func (a *App) visible() []*calibre.Book {
	k := shownKey{a.query, a.version}
	if k != a.shownKey || a.shown == nil {
		a.shown = catalog.Apply(a.books, a.query, a.booksTime)
		a.shownKey = k
	}
	return a.shown
}

// current is the book the detail panel shows: the last one chosen.
func (a *App) current() *calibre.Book {
	a.adoptCur()
	return a.byID[a.curID]
}

// adoptCur takes the row the grid or table moved cur to as the current book.
func (a *App) adoptCur() {
	if a.cur == a.curAt {
		return
	}
	a.curID = 0
	if a.cur >= 0 && a.cur < len(a.curList) {
		a.curID = a.curList[a.cur].ID
	}
	a.curAt = a.cur
}

// syncCur points cur at the current book's row in shown, wherever the
// query, the order or an edit moved it.
func (a *App) syncCur(shown []*calibre.Book) {
	a.adoptCur()
	a.cur = slices.IndexFunc(shown, func(b *calibre.Book) bool { return b.ID == a.curID })
	a.curAt, a.curList = a.cur, shown
}

// selectedIDs are the books chosen, or the current one.
func (a *App) selectedIDs() []int64 {
	var ids []int64
	for _, b := range a.visible() {
		if a.chosen.Has(b.ID) {
			ids = append(ids, b.ID)
		}
	}
	if len(ids) == 0 {
		if b := a.current(); b != nil {
			ids = []int64{b.ID}
		}
	}
	return ids
}

// detailsOf returns the book's comments and files, loading them once.
func (a *App) detailsOf(b *calibre.Book) *calibre.Details {
	if d := a.details[b.ID]; d != nil {
		return d
	}
	if a.lib != nil && !a.detailsReq[b.ID] {
		a.detailsReq[b.ID] = true
		lib := a.lib
		a.run(func() {
			d, err := lib.Details(b)
			a.post(func() {
				if err == nil && lib == a.lib {
					a.details[b.ID] = d
				}
			})
		})
	}
	return nil
}

var ebookFilter = []mygo.FileFilter{
	{Name: "E-books", Extensions: []string{"epub", "pdf", "mobi", "azw3", "azw", "fb2", "txt", "cbz", "djvu", "docx", "rtf"}},
	{Name: "All files", Extensions: []string{"*"}},
}

// chooseBooks asks for files and adds them.
func (a *App) chooseBooks() {
	a.run(func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: a.win, Title: "Add books", Multiple: true, Filters: ebookFilter})
		if err != nil || len(paths) == 0 {
			return
		}
		a.post(func() { a.importFiles(paths) })
	})
}

// importFiles adds files, and the e-books inside folders, as new books.
func (a *App) importFiles(paths []string) {
	lib := a.lib
	if lib == nil || a.busy != "" {
		return
	}
	a.busy, a.busyDone, a.busyTotal = "Adding books", 0, 0
	a.run(func() {
		// Folders are walked here, off the main thread: they may be large.
		var files []string
		for _, p := range paths {
			filepath.WalkDir(p, func(path string, d os.DirEntry, err error) error {
				if err == nil && !d.IsDir() && isEbook(path) {
					files = append(files, path)
				}
				return nil
			})
		}
		if len(files) == 0 {
			a.post(func() {
				a.busy = ""
				a.toast("No e-books found to add")
			})
			return
		}
		a.post(func() { a.busyTotal = len(files) })
		var added []int64
		var errs []string
		for i, f := range files {
			id, err := lib.Import(f)
			if err != nil {
				errs = append(errs, fmt.Sprintf("%s: %v", filepath.Base(f), err))
			} else {
				added = append(added, id)
			}
			a.post(func() { a.busyDone = i + 1 })
		}
		a.post(func() {
			a.busy = ""
			a.reload(func() {
				if len(added) > 0 {
					a.query = catalog.Query{Sort: catalog.SortDateAdded, Descending: true}
					a.chosen.Clear()
					a.curID = added[len(added)-1]
					a.toast("Added %d %s", len(added), plural(len(added), "book", "books"))
				}
				for _, e := range errs {
					a.toastError(errors.New(e))
				}
			})
		})
	})
}

func isEbook(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".epub", ".pdf", ".mobi", ".azw3", ".azw", ".fb2", ".txt", ".cbz", ".djvu", ".docx", ".rtf", ".prc":
		return true
	}
	return false
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// saveMetadata writes the editor's changes.
func (a *App) saveMetadata(id int64, m calibre.Metadata, cover []byte) {
	lib := a.lib
	if lib == nil {
		return
	}
	a.run(func() {
		err := lib.UpdateMetadata(id, m)
		if err == nil && len(cover) > 0 {
			err = lib.SetCover(id, cover)
		}
		a.post(func() {
			if err != nil {
				a.toastError(err)
				return
			}
			a.covers.forget(id)
			a.reload(func() { a.toast("Saved “%s”", m.Title) })
		})
	})
}

// removeBooks moves the books to the Recycle Bin.
func (a *App) removeBooks(ids []int64) {
	lib := a.lib
	if lib == nil || len(ids) == 0 {
		return
	}
	a.run(func() {
		err := lib.RemoveBooks(ids, a.trash)
		a.post(func() {
			a.chosen.Clear()
			a.curID = 0
			a.reload(func() {
				if err != nil {
					a.toastError(err)
				}
				a.toast("Moved %d %s to the Recycle Bin", len(ids), plural(len(ids), "book", "books"))
			})
		})
	})
}

// addFormat asks for a file and adds it to the book.
func (a *App) addFormat(id int64) {
	lib := a.lib
	a.run(func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: a.win, Title: "Add a format", Filters: ebookFilter})
		if err != nil || len(paths) == 0 || lib == nil {
			return
		}
		err = lib.AddFormat(id, paths[0])
		a.post(func() {
			if err != nil {
				a.toastError(err)
				return
			}
			a.reload(func() { a.toast("Added %s", strings.ToUpper(strings.TrimPrefix(filepath.Ext(paths[0]), "."))) })
		})
	})
}

func (a *App) removeFormat(id int64, format string) {
	lib := a.lib
	a.run(func() {
		err := lib.RemoveFormat(id, format)
		a.post(func() {
			if err != nil {
				a.toastError(err)
				return
			}
			a.reload(func() { a.toast("Removed %s", format) })
		})
	})
}

// openBook opens the book's preferred format in the default reader.
func (a *App) openBook(b *calibre.Book, format string) {
	if b == nil || a.lib == nil {
		return
	}
	if format == "" {
		for _, f := range []string{"EPUB", "AZW3", "MOBI", "PDF"} {
			if slices.Contains(b.Formats, f) {
				format = f
				break
			}
		}
		if format == "" && len(b.Formats) > 0 {
			format = b.Formats[0]
		}
	}
	if format == "" {
		a.toast("“%s” has no files", b.Title)
		return
	}
	lib := a.lib
	a.run(func() {
		d, err := lib.Details(b)
		if err == nil {
			for _, f := range d.Files {
				if f.Format == format {
					err = mygo.Shell.OpenPath(f.Path)
					break
				}
			}
		}
		if err != nil {
			a.post(func() { a.toastError(err) })
		}
	})
}

func (a *App) revealBook(b *calibre.Book) {
	if b != nil && a.lib != nil {
		mygo.Shell.ShowItemInFolder(a.lib.Dir(b))
	}
}

func (a *App) chooseLibrary() {
	a.run(func() {
		dirs, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: a.win, Title: "Open a calibre library", Directory: true, CreateDirectories: true})
		if err != nil || len(dirs) == 0 {
			return
		}
		a.post(func() { a.openLibrary(dirs[0]) })
	})
}
