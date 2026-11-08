package app

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"dalibre/cmd/dalibre/locales"
	"dalibre/internal/calibre"
	"dalibre/internal/plugin"
)

// Fetch metadata (Figma, Screens › 04 · Edit metadata, "Fetch metadata"):
// a step of the Edit metadata dialog that asks the metadata plugins for
// candidates and fills the form with the fields the user picks. Nothing
// is written until the form is saved.

// fetchTimeout bounds one plugin's identify.
const fetchTimeout = 30 * time.Second

// fetchState is the Fetch metadata step of the Edit metadata dialog.
type fetchState struct {
	open bool
	// text is the search box; while it is still initial the book's own
	// title, authors, identifiers and language are looked up.
	text, initial string
	// source is the plugin searched, by id; "" searches all.
	source string
	// gen counts searches, so a late answer to an older one is dropped.
	gen     int
	pending map[string]bool // plugins still answering, by id
	results map[string][]plugin.Result
	errs    map[string]error
	// sel is the candidate shown, as candKey.
	sel string
	// use is which fields the user picked for the candidate shown.
	use    map[string]bool
	covers map[string]*remoteCover // by URL
}

// candidate is a plugin's answer.
type candidate struct {
	plugin *plugin.Plugin
	r      plugin.Result
}

func (c candidate) key() string { return c.plugin.Manifest.ID + "\x00" + c.r.ID }

// metadataSources are the loaded plugins that look up metadata.
func (a *App) metadataSources() []*plugin.Plugin {
	var out []*plugin.Plugin
	for _, p := range a.plugins {
		if p.Manifest.IsMetadataSource() {
			out = append(out, p)
		}
	}
	return out
}

// openFetch shows the Fetch metadata step for the book being edited, and
// searches.
func (a *App) openFetch() {
	e := &a.edit
	text := strings.TrimSpace(e.title)
	if len(e.authors) > 0 {
		text += " " + e.authors[0]
	}
	src := a.fetch.source
	if src != "" && a.pluginByID(src) == nil {
		src = ""
	}
	a.fetch = fetchState{open: true, text: text, initial: text, source: src, covers: map[string]*remoteCover{}}
	a.fetchSearch()
}

// fetchSources are the plugins the step searches.
func (a *App) fetchSources() []*plugin.Plugin {
	all := a.metadataSources()
	if a.fetch.source == "" {
		return all
	}
	return slices.DeleteFunc(all, func(p *plugin.Plugin) bool { return p.Manifest.ID != a.fetch.source })
}

// fetchBook is what the search box asks for: the book itself while the
// box holds what it was given, else what the user typed, as an ISBN when
// it is one.
func (a *App) fetchBook() plugin.Book {
	f, e := &a.fetch, &a.edit
	text := strings.TrimSpace(f.text)
	if text == strings.TrimSpace(f.initial) {
		b := plugin.Book{Title: e.title, Authors: slices.Clone(e.authors), Identifiers: parseIdentifiers(e.identifiers)}
		if len(e.languages) > 0 {
			b.Language = calibre.LanguageCode(e.languages[0])
		}
		return b
	}
	if isbn := isbnOf(text); isbn != "" {
		return plugin.Book{Identifiers: map[string]string{"isbn": isbn}}
	}
	return plugin.Book{Title: text, Identifiers: map[string]string{}}
}

// isbnOf is s as an ISBN, digits only, if it is one.
func isbnOf(s string) string {
	s = strings.Map(func(r rune) rune {
		switch {
		case r >= '0' && r <= '9':
			return r
		case r == 'x' || r == 'X':
			return 'X'
		case r == '-' || r == ' ':
			return -1
		}
		return '!'
	}, strings.TrimPrefix(strings.ToLower(strings.TrimSpace(s)), "isbn"))
	if strings.Contains(s, "!") || len(s) != 10 && len(s) != 13 || strings.Contains(s[:len(s)-1], "X") {
		return ""
	}
	return s
}

// fetchSearch asks every source for candidates, in the background.
func (a *App) fetchSearch() {
	f := &a.fetch
	f.gen++
	gen := f.gen
	f.pending, f.results, f.errs, f.sel, f.use = map[string]bool{}, map[string][]plugin.Result{}, map[string]error{}, "", nil
	book := a.fetchBook()
	if strings.TrimSpace(book.Title) == "" && len(book.Identifiers) == 0 {
		return
	}
	for _, p := range a.fetchSources() {
		id := p.Manifest.ID
		f.pending[id] = true
		a.run(func() {
			ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
			defer cancel()
			rs, err := p.Identify(ctx, book)
			a.post(func() {
				if a.fetch.gen != gen {
					return
				}
				delete(a.fetch.pending, id)
				a.fetch.results[id], a.fetch.errs[id] = rs, err
				if a.fetch.sel == "" {
					if cs := a.fetchCandidates(); len(cs) > 0 {
						a.fetchSelect(cs[0])
					}
				}
			})
		})
	}
}

// fetchCandidates are the candidates found so far, by source then rank.
func (a *App) fetchCandidates() []candidate {
	var out []candidate
	for _, p := range a.fetchSources() {
		for _, r := range a.fetch.results[p.Manifest.ID] {
			out = append(out, candidate{p, r})
		}
	}
	return out
}

// fetchSelected is the candidate shown, if any.
func (a *App) fetchSelected() (candidate, bool) {
	for _, c := range a.fetchCandidates() {
		if c.key() == a.fetch.sel {
			return c, true
		}
	}
	return candidate{}, false
}

// fetchSelect shows c, with the fields that would change picked.
func (a *App) fetchSelect(c candidate) {
	a.fetch.sel = c.key()
	a.fetch.use = map[string]bool{}
	for _, fd := range a.fetchFields(c) {
		a.fetch.use[fd.key] = fd.changes
	}
	a.fetchCover(c)
}

// fetchCover is c's cover, loaded in the background on first ask.
func (a *App) fetchCover(c candidate) *remoteCover {
	url := c.r.Cover
	if url == "" {
		return nil
	}
	if rc := a.fetch.covers[url]; rc != nil {
		return rc
	}
	rc := &remoteCover{loading: true}
	a.fetch.covers[url] = rc
	a.run(func() {
		coverSlots <- struct{}{}
		data, err := c.plugin.Cover(context.Background(), c.r)
		<-coverSlots
		var bmp *ui.Bitmap
		var size image.Point
		if err == nil && len(data) > 0 {
			if img, _, err := image.Decode(bytes.NewReader(data)); err == nil {
				bmp, size = ui.NewBitmap(img), img.Bounds().Size()
			}
		}
		a.post(func() {
			rc.loading, rc.bmp = false, bmp
			if bmp != nil {
				rc.data, rc.size = data, size
			}
		})
	})
	return rc
}

// fetchField is a line of the comparison: what the book has and what the
// candidate would put there.
type fetchField struct {
	key, label string
	current    string
	next       string
	// changes is set when the candidate has a value and it differs.
	changes bool
}

// fetchFields compares the form with c, field by field.
func (a *App) fetchFields(c candidate) []fetchField {
	e, r := &a.edit, c.r
	var out []fetchField
	add := func(key, label, cur, next string) {
		cur, next = strings.TrimSpace(cur), strings.TrimSpace(next)
		out = append(out, fetchField{key: key, label: label, current: cur, next: next, changes: next != "" && next != cur})
	}
	cover := ""
	if r.Cover != "" {
		cover = r.Cover
	}
	out = append(out, fetchField{key: "cover", label: locales.FetchMetaCover(), next: cover, changes: cover != ""})
	add("title", locales.CommonTitle(), e.title, r.Title)
	add("authors", locales.CommonAuthors(), strings.Join(e.authors, ", "), strings.Join(r.Authors, ", "))
	series := r.Series
	if series != "" && r.SeriesIndex > 0 {
		series += " #" + seriesIndex(r.SeriesIndex)
	}
	cur := e.series
	if cur != "" && strings.TrimSpace(e.seriesIndex) != "" && r.SeriesIndex > 0 {
		cur += " #" + strings.TrimSpace(e.seriesIndex)
	}
	add("series", locales.CommonSeries(), cur, series)
	add("tags", locales.CommonTags(), strings.Join(e.tags, ", "), strings.Join(r.Tags, ", "))
	add("publisher", locales.CommonPublisher(), e.publisher, r.Publisher)
	pub := ""
	if t := plugin.ParseDate(r.Pubdate); !t.IsZero() {
		pub = strings.TrimSpace(r.Pubdate)
		if len(pub) > 10 {
			pub = t.Format("2006-01-02")
		}
	}
	curPub := ""
	if !e.pubdate.IsZero() {
		curPub = e.pubdate.Format("2006-01-02")
		if pub != "" && strings.HasPrefix(curPub, pub) {
			pub = curPub // the same date, said less precisely
		}
	}
	add("pubdate", locales.CommonPublished(), curPub, pub)
	lang := ""
	if code := calibre.LanguageCode(r.Language); code != "" {
		lang = languageName(code)
	}
	var langs []string
	for _, l := range e.languages {
		if code := calibre.LanguageCode(l); code != "" {
			langs = append(langs, languageName(code))
		} else {
			langs = append(langs, l)
		}
	}
	add("language", locales.CommonLanguage(), strings.Join(langs, ", "), lang)
	have := parseIdentifiers(e.identifiers)
	var added []string
	for _, k := range slices.Sorted(maps.Keys(r.Identifiers)) {
		if have[k] != r.Identifiers[k] {
			added = append(added, k+":"+r.Identifiers[k])
		}
	}
	out = append(out, fetchField{key: "identifiers", label: locales.CommonIdentifiers(), current: strings.TrimSpace(e.identifiers),
		next: strings.Join(added, ", "), changes: len(added) > 0})
	desc := r.Description
	if strings.Contains(desc, "<") {
		desc = htmlToText(desc)
	}
	add("description", locales.CommonDescription(), e.comments, desc)
	return out
}

// fetchApply fills the form with the picked fields of the candidate shown
// and returns to it. Nothing is saved.
func (a *App) fetchApply() {
	c, ok := a.fetchSelected()
	if !ok {
		return
	}
	e, r, use := &a.edit, c.r, a.fetch.use
	for _, fd := range a.fetchFields(c) {
		if !use[fd.key] || fd.next == "" {
			continue
		}
		switch fd.key {
		case "cover":
			if rc := a.fetch.covers[r.Cover]; rc != nil && rc.bmp != nil {
				e.cover, e.coverBmp = rc.data, rc.bmp
			}
		case "title":
			e.title = r.Title
		case "authors":
			e.authors, e.authorSort = slices.Clone(r.Authors), ""
		case "series":
			e.series = r.Series
			if r.SeriesIndex > 0 {
				e.seriesIndex = seriesIndex(r.SeriesIndex)
			}
		case "tags":
			e.tags = slices.Clone(r.Tags)
		case "publisher":
			e.publisher = r.Publisher
		case "pubdate":
			if t := plugin.ParseDate(r.Pubdate); !t.IsZero() {
				e.pubdate = t
			}
		case "language":
			e.languages = []string{calibre.LanguageCode(r.Language)}
		case "identifiers":
			ids := parseIdentifiers(e.identifiers)
			maps.Copy(ids, r.Identifiers)
			e.identifiers = formatIdentifiers(ids)
		case "description":
			e.comments = fd.next
		}
	}
	a.fetch.open = false
	a.toast("%s", locales.FetchMetaFilledIn(c.plugin.Manifest.Name))
}

// parseIdentifiers reads the form's "type:value, …".
func parseIdentifiers(s string) map[string]string {
	out := map[string]string{}
	for _, part := range strings.Split(s, ",") {
		if k, v, ok := strings.Cut(strings.TrimSpace(part), ":"); ok {
			if k, v = strings.ToLower(strings.TrimSpace(k)), strings.TrimSpace(v); k != "" && v != "" {
				out[k] = v
			}
		}
	}
	return out
}

// formatIdentifiers writes identifiers as the form shows them.
func formatIdentifiers(ids map[string]string) string {
	var parts []string
	for _, k := range slices.Sorted(maps.Keys(ids)) {
		parts = append(parts, k+":"+ids[k])
	}
	return strings.Join(parts, ", ")
}

// candidateLines are the lines under a candidate's title: authors and
// year, and publisher and source.
func candidateLines(c candidate) (string, string) {
	var first, second []string
	if len(c.r.Authors) > 0 {
		first = append(first, strings.Join(c.r.Authors, ", "))
	}
	if t := plugin.ParseDate(c.r.Pubdate); !t.IsZero() {
		first = append(first, fmt.Sprint(t.Year()))
	}
	if c.r.Publisher != "" {
		second = append(second, c.r.Publisher)
	}
	second = append(second, c.plugin.Manifest.Name)
	return strings.Join(first, " · "), strings.Join(second, " · ")
}
