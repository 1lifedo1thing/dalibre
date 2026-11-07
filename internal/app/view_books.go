package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"dalibre/internal/app/icons"
	"dalibre/internal/app/theme"
	"dalibre/internal/app/widgets"
	"dalibre/internal/calibre"
	"dalibre/internal/catalog"
)

// The grid is a wall of floating shelves (Figma "01b · Library — Shelf"):
// each cell is a slot of a shelf, its book standing on a thin board under
// which its title sits.
const (
	shelfBayH   = 228 // the tallest cover, lifted, with air above it
	shelfPlankH = 12  // a board: lit top, front face, bottom lip
	shelfLabelH = 76
	// shelfReach is how far a slot's board and shadows reach past its cell,
	// over the cell's border and half the gap, so the boards of a row join.
	shelfReach = 6
	gridCellH  = shelfBayH + shelfPlankH + shelfLabelH + 4 // + the cell's border

	coverMinH, coverMaxH = 160, 210
)

func (a *App) gridView(c *ui.Context, shown []*calibre.Book) {
	a.grid.Key = func(i int) any { return shown[i].ID }
	a.grid.Selection = &a.chosen
	a.grid.Selected = &a.cur
	a.grid.Label = func(i int) string { return shown[i].Title }
	// The grid tints and outlines its chosen cells; on the shelf a book
	// shows it by being pulled out instead, so its cells stay clear.
	base := c.Theme()
	tt := *base
	tt.Accent, tt.InactiveAccent, tt.SurfaceHover, tt.SurfacePressed, tt.Border = ui.Transparent, ui.Transparent, ui.Transparent, ui.Transparent, ui.Transparent
	tt.Radius = 0
	var g ui.Element
	ui.Themed(c, &tt).Grow(1).MinHeight(0).Background(ui.Transparent).AlignItems(ui.Stretch).Children(func() {
		g = ui.GridView(c, &a.grid, len(shown), theme.CardW, gridCellH, func(i int) {
			a.shelfSlot(c, base, shown[i], i == len(shown)-1)
		}).Grow(1).Margin(theme.Space8, theme.Space32, 0, theme.Space32).Label("Books")
	})
	if g.Submitted() {
		a.openBook(a.current(), "")
	}
}

// shelfSlot builds a book standing on its board, with its title below; the
// last one's board runs on to the end of the shelf.
func (a *App) shelfSlot(c *ui.Context, base *ui.Theme, b *calibre.Book, last bool) {
	p := pal()
	ink := ui.RGBA(26, 23, 18, 1)
	if p.Dark {
		ink = ui.RGBA(0, 0, 0, 1)
	}
	right := float32(-shelfReach)
	if last {
		right = -4096 // clipped by the grid
	}
	chosen := a.chosen.Has(b.ID)
	slot := ui.Themed(c, base).Fill().Background(ui.Transparent).ContextMenu(a.bookMenu(b))
	var lift float32
	switch {
	case chosen:
		lift = theme.Space12
	case slot.Hovered():
		lift = theme.Space4
	}
	slot.Children(func() {
		ui.Column(c).Height(shelfBayH).Justify(ui.End).AlignItems(ui.Center).Padding(0, 0, lift, 0).Children(func() {
			bmp := a.cover(b, theme.CoverH*2)
			cov := coverOf(c, b, bmp, theme.CoverW, shelfCoverH(bmp), false)
			if lift > 0 {
				cov.Shadow(0, 14, 22, -6, ink.Alpha(0.26))
			}
		})
		ui.Box(c).Height(shelfPlankH).Children(func() {
			ui.Column(c).Absolute().Top(0).Bottom(0).Left(-shelfReach).Right(right).PassThrough().Children(func() {
				ui.Box(c).Height(3).Background(p.PlankTop)
				ui.Box(c).Grow(1).Background(p.PlankFace)
				ui.Box(c).Height(2).Background(p.PlankEdge)
			})
			if chosen {
				ui.Row(c).Absolute().Top(5).Left(0).Right(0).Justify(ui.Center).PassThrough().Children(func() {
					ui.Box(c).Size(24, 2).Radius(1).Background(p.Accent)
				})
			}
		})
		ui.Column(c).Grow(1).Padding(theme.Space16, 0, 0, 0).AlignItems(ui.Center).Children(func() {
			// The soft shadow the board casts on the wall below it.
			ui.Box(c).Absolute().Top(0).Height(18).Left(-shelfReach).Right(right).PassThrough().LinearGradient(ui.LinearGradient{
				Angle: 180, Stops: []ui.GradientStop{{Color: ink.Alpha(0.10), At: 0}, {Color: ink.Alpha(0.03), At: 0.4}, {Color: ink.Alpha(0), At: 1}},
			})
			// As wide as the cover, under it.
			ui.Column(c).Width(theme.CoverW).Gap(theme.Space2).AlignItems(ui.Stretch).Children(func() {
				title := ui.Text(c, b.Title).FontWeight(500).FontSize(13).FixedLineHeight(18).SingleLine()
				if chosen {
					title.TextColor(p.TextAccent)
				}
				ui.Text(c, strings.Join(b.Authors, ", ")).FontSize(12).TextColor(p.TextTertiary).SingleLine()
			})
		})
	})
}

// shelfCoverH is how high a cover stands at the shelf's width: as its
// image is, within bounds, so books differ in height as real ones do.
func shelfCoverH(bmp *ui.Bitmap) float32 {
	if bmp == nil {
		return theme.CoverH
	}
	w, h := bmp.Size()
	if w <= 0 {
		return theme.CoverH
	}
	return min(max(theme.CoverW*float32(h)/float32(w), coverMinH), coverMaxH)
}

var tableColumns = []ui.TableColumn{
	{ID: "cover", Width: 44, Fixed: true},
	{ID: "title", Title: "Title", Sortable: true, MinWidth: 160, RowHeader: true},
	{ID: "author", Title: "Author", Width: 130, Sortable: true},
	{ID: "series", Title: "Series", Width: 110, Sortable: true},
	{ID: "rating", Title: "Rating", Width: 84, Sortable: true},
	{ID: "formats", Title: "Formats", Width: 92},
	{ID: "size", Title: "Size", Width: 76, Align: ui.End, Sortable: true},
	{ID: "added", Title: "Added", Width: 96, Align: ui.End, Sortable: true},
}

var columnSort = map[string]catalog.SortKey{
	"title": catalog.SortTitle, "author": catalog.SortAuthor, "series": catalog.SortSeries, "rating": catalog.SortRating,
	"size": catalog.SortSize, "added": catalog.SortDateAdded,
}

func (a *App) listView(c *ui.Context, shown []*calibre.Book) {
	// The header shows the order the toolbar's menu chose, and sets it.
	a.tableSort = ui.SortOrder{Descending: a.query.Descending}
	for id, k := range columnSort {
		if k == a.query.Sort {
			a.tableSort.Column = id
		}
	}
	before := a.tableSort
	a.table.Key = func(i int) any { return shown[i].ID }
	a.table.Selection = &a.chosen
	a.table.Selected = &a.cur
	a.table.Sort = &a.tableSort
	a.table.RoundRows = true
	a.table.Label = func(i int) string { return shown[i].Title }
	a.table.CellMenu = func(m *ui.Menu, row, col int) { a.bookMenu(shown[row])(m) }
	// Chosen rows in the design's tint (bg/selected) with the usual text,
	// rather than the accent with white text, so every column stays legible.
	tt := *c.Theme()
	tt.Accent, tt.InactiveAccent, tt.AccentText = pal().Selected, pal().Selected, pal().Text
	var t ui.Element
	ui.Themed(c, &tt).Grow(1).MinHeight(0).Background(ui.Transparent).Margin(0, theme.Space24, theme.Space24, theme.Space24).AlignItems(ui.Stretch).Children(func() {
		t = ui.Table(c, &a.table, tableColumns, len(shown), func(row, col int) {
			b := shown[row]
			switch col {
			case 0:
				ui.Box(c).Padding(theme.Space6, 0).Children(func() { a.coverView(c, b, 28, 40, true) })
			case 1:
				ui.Column(c).Gap(theme.Space2).MinWidth(0).Children(func() {
					ui.Text(c, b.Title).FontWeight(500).SingleLine()
					sub := b.Sort
					if !b.Pubdate.IsZero() {
						sub = fmt.Sprint(b.Pubdate.Year())
					} else {
						sub = ""
					}
					if b.Publisher != "" {
						sub = strings.TrimPrefix(sub+" · "+b.Publisher, " · ")
					}
					if sub != "" {
						ui.Text(c, sub).FontSize(12).TextColor(pal().TextTertiary).SingleLine()
					}
				})
			case 2:
				ui.Text(c, strings.Join(b.Authors, ", ")).SingleLine()
			case 3:
				if b.Series != "" {
					ui.Textf(c, "%s #%s", b.Series, seriesIndex(b.SeriesIndex)).TextColor(pal().TextSecondary).SingleLine()
				} else {
					ui.Text(c, "—").TextColor(pal().TextTertiary)
				}
			case 4:
				widgets.Stars(c, b.Rating, 12)
			case 5:
				ui.Row(c).Gap(theme.Space4).Children(func() {
					for _, f := range b.Formats {
						widgets.FormatBadge(c.Key(f), f, false)
					}
				})
			case 6:
				ui.Text(c, humanSize(b.Size)).Font(theme.FontMono).FontSize(12).TextColor(pal().TextSecondary).NoWrap()
			case 7:
				ui.Text(c, isoDate(b.Timestamp)).Font(theme.FontMono).FontSize(12).TextColor(pal().TextSecondary).NoWrap()
			}
		}).Grow(1).Label("Books")
	})
	if t.Submitted() {
		a.openBook(a.current(), "")
	}
	if a.tableSort != before {
		if k, ok := columnSort[a.tableSort.Column]; ok {
			a.query.Sort, a.query.Descending = k, a.tableSort.Descending
			a.saveSettings()
		}
	}
}

// bookMenu is the context menu of a book, in the grid and the list.
func (a *App) bookMenu(b *calibre.Book) func(m *ui.Menu) {
	return func(m *ui.Menu) {
		ids := a.selectedIDs()
		if !containsID(ids, b.ID) {
			ids = []int64{b.ID}
		}
		many := len(ids) > 1
		if !many {
			if m.Item("Open").Icon(icons.Menu(icons.BookOpen)).Trailing("Enter").Chosen() {
				a.openBook(b, "")
			}
			if m.Item("Edit metadata…").Icon(icons.Menu(icons.Pencil)).Trailing("Ctrl+I").Chosen() {
				a.openEditor(b)
			}
		}
		label := "Convert…"
		if many {
			label = fmt.Sprintf("Convert %d books…", len(ids))
		}
		if m.Item(label).Icon(icons.Menu(icons.Convert)).Trailing("Ctrl+Shift+C").Chosen() {
			a.openConvert(ids)
		}
		if !many {
			if m.Item("Show in folder").Icon(icons.Menu(icons.Folder)).Chosen() {
				a.revealBook(b)
			}
		}
		m.Separator()
		del := "Remove from library"
		if many {
			del = fmt.Sprintf("Remove %d books", len(ids))
		}
		if m.Item(del).Icon(icons.Menu(icons.Trash)).Danger().Trailing("Del").Chosen() {
			a.askDelete(ids)
		}
	}
}

func containsID(ids []int64, id int64) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

func seriesIndex(f float64) string {
	if f == float64(int64(f)) {
		return fmt.Sprint(int64(f))
	}
	return fmt.Sprintf("%.1f", f)
}

func humanSize(n int64) string {
	switch {
	case n <= 0:
		return "—"
	case n < 1<<20:
		return fmt.Sprintf("%d KB", (n+1023)/1024)
	case n < 1<<30:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	}
	return fmt.Sprintf("%.1f GB", float64(n)/(1<<30))
}

func isoDate(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return t.Local().Format("2006-01-02")
}
