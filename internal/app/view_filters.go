package app

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"dalibre/internal/app/icons"
	"dalibre/internal/app/theme"
	"dalibre/internal/app/widgets"
	"dalibre/internal/catalog"
)

// pickerLimit caps the rows a picker lists; typing narrows the rest.
const pickerLimit = 200

type filterPicker struct {
	kind   catalog.FilterKind
	name   string // as on the chip
	plural string // as in the picker's search placeholder
	icon   *ui.SVG
	items  []catalog.Count
}

func (a *App) filterPickers() []filterPicker {
	return []filterPicker{
		{catalog.ByAuthor, "Author", "authors", icons.User, a.counts.Authors},
		{catalog.BySeries, "Series", "series", icons.Layers, a.counts.Series},
		{catalog.ByTag, "Tag", "tags", icons.Tag, a.counts.Tags},
		{catalog.ByPublisher, "Publisher", "publishers", icons.Building, a.counts.Publishers},
		{catalog.ByLanguage, "Language", "languages", icons.Globe, a.counts.Languages},
	}
}

// filterBar is the row of filter chips over the books: each opens a
// searchable picker of its values, so long lists stay out of the sidebar.
func (a *App) filterBar(c *ui.Context) {
	q := a.query
	ui.Row(c).Padding(0, theme.Space32, theme.Space12, theme.Space32).Gap(theme.Space8).AlignItems(ui.Center).Wrap().Children(func() {
		for _, p := range a.filterPickers() {
			if len(p.items) == 0 {
				continue
			}
			a.filterChip(c.Key(int(p.kind)), p)
		}
		// Filters with no chip, as a format chosen in the sidebar.
		if k := q.Filter.Kind; k == catalog.ByFormat {
			widgets.Tag(c.Key("filter"), filterKindName(k)+": "+q.Filter.Value, true, func() { a.setQuery(a.query.Shelf, catalog.Filter{}) })
		}
		if q.Search != "" {
			widgets.Tag(c.Key("search"), "“"+q.Search+"”", true, func() { a.query.Search = "" })
		}
		if q.Filter.Kind == catalog.NoFilter && q.Search == "" {
			return
		}
		clear := ui.ButtonBase(c).Padding(theme.Space2, theme.Space4).Radius(theme.RadiusXS).Label("Clear all")
		clear.OnClick(func() {
			a.query.Search = ""
			a.setQuery(a.query.Shelf, catalog.Filter{})
		}).Children(func() {
			t := ui.Text(c, "Clear all").FontSize(12).FontWeight(500).TextColor(pal().TextAccent)
			if clear.Hovered() {
				t.Underline()
			}
		})
	})
}

func (a *App) filterChip(c *ui.Context, p filterPicker) {
	f := a.query.Filter
	active := f.Kind == p.kind
	open := &a.pickerOpen[p.kind]
	label := p.name
	if active {
		label = p.name + ": " + valueLabel(p.kind, f.Value)
	}
	chip := ui.ButtonBase(c).Height(28).MaxWidth(280).Radius(theme.RadiusFull).Padding(0, theme.Space6, 0, theme.Space10).Gap(theme.Space4).AlignItems(ui.Center).Label(label)
	bg, fg, border := ui.Transparent, pal().TextSecondary, pal().BorderSubtle
	switch {
	case active:
		bg, fg, border = pal().AccentSubtle, pal().TextAccent, pal().AccentSubtle
	case *open || chip.Hovered():
		bg = pal().Hover
	}
	chip.Background(bg).Border(1, border).TextColor(fg)
	if chip.Clicked() {
		*open = !*open
		a.pickerQuery = ""
	}
	chip.Children(func() {
		ui.Icon(c, p.icon).FontSize(14)
		ui.Text(c, label).FontSize(12).FontWeight(500).SingleLine().Shrink(1)
		if active {
			x := ui.ButtonBase(c).Size(18, 18).Radius(theme.RadiusFull).Center().Label("Clear " + p.name)
			if x.Hovered() {
				x.Background(fg.Alpha(0.15))
			}
			x.OnClick(func() { a.setQuery(a.query.Shelf, catalog.Filter{}) }).Children(func() { ui.Icon(c, icons.X).FontSize(11) })
		} else {
			ui.Icon(c, icons.ChevronDown).FontSize(12).TextColor(pal().IconMuted)
		}
	})
	ui.PopoverBase(c, chip, open, func(panel ui.Element) {
		panel.Width(300).Padding(theme.Space6).Gap(theme.Space4).Radius(theme.RadiusLG).Background(pal().SurfaceRaised).Border(1, pal().Border).
			Shadow(0, 2, 6, 0, ui.RGBA(26, 23, 18, 0.08)).Shadow(0, 24, 48, -8, ui.RGBA(26, 23, 18, 0.18)).AlignItems(ui.Stretch).Children(func() {
			if len(p.items) > 8 {
				ui.SearchField(c, &a.pickerQuery).Placeholder("Search " + p.plural + "…").Label("Search " + p.plural).AutoFocus()
			}
			needle := strings.ToLower(strings.TrimSpace(a.pickerQuery))
			ui.Scroll(c).MaxHeight(340).Gap(theme.Space2).AlignItems(ui.Stretch).Children(func() {
				n := 0
				for _, it := range p.items {
					label := valueLabel(p.kind, it.Value)
					if needle != "" && !strings.Contains(strings.ToLower(label), needle) && !strings.Contains(strings.ToLower(it.Value), needle) {
						continue
					}
					if n == pickerLimit {
						ui.Text(c, "Type to find more…").FontSize(12).TextColor(pal().TextTertiary).Padding(theme.Space6, theme.Space8)
						break
					}
					n++
					value := it.Value
					sel := f == catalog.Filter{Kind: p.kind, Value: value}
					a.pickerRow(c.Key(value), label, it.N, sel, func() {
						*open = false
						a.setQuery(a.query.Shelf, catalog.Filter{Kind: p.kind, Value: value})
					})
				}
				if n == 0 {
					ui.Text(c, "No matches").FontSize(12).TextColor(pal().TextTertiary).Padding(theme.Space12, theme.Space8)
				}
			})
		})
	})
}

func (a *App) pickerRow(c *ui.Context, label string, count int, selected bool, onClick func()) {
	row := ui.ButtonBase(c).Height(30).Radius(theme.RadiusSM).Padding(0, theme.Space8).Gap(theme.Space8).AlignItems(ui.Center).Label(fmt.Sprintf("%s, %d %s", label, count, plural(count, "book", "books")))
	if selected {
		row.Background(pal().AccentSubtle)
	} else if row.Hovered() {
		row.Background(pal().Hover)
	}
	row.OnClick(onClick).Children(func() {
		t := ui.Text(c, label).Grow(1).Shrink(1).SingleLine().FontSize(13).TextColor(pal().Text)
		if selected {
			t.FontWeight(500).TextColor(pal().TextAccent)
		}
		ui.Textf(c, "%s", groupDigits(count)).Font(theme.FontMono).FontSize(12).TextColor(pal().TextTertiary)
	})
}

func valueLabel(k catalog.FilterKind, v string) string {
	if k == catalog.ByLanguage {
		return languageName(v)
	}
	return v
}
