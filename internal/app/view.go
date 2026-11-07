package app

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui"

	"dalibre/internal/app/icons"
	"dalibre/internal/app/theme"
	"dalibre/internal/app/widgets"
	"dalibre/internal/catalog"
)

// view builds the window from the state, every frame.
func (a *App) view(c *ui.Context) {
	sys := c.Theme()
	c.SetTheme(theme.Use(sys.Dark).Theme(sys))
	c.SetMenuPresentation(ui.PopoverMenus) // menus drawn in the window, in the design's look

	for _, t := range a.toasts {
		typ := "success"
		if t.error {
			typ = "error"
		}
		c.AddToast(ui.Toast{Title: t.text, Type: typ})
	}
	a.toasts = nil

	a.shortcuts(c)

	root := ui.Row(c).Fill().AlignItems(ui.Stretch).Background(pal().BgApp).TextColor(pal().Text)
	if files := root.DroppedFiles(); files != nil && a.lib != nil {
		a.importFiles(files)
	}
	dragOver := root.FileDragOver() && a.lib != nil
	root.Children(func() {
		a.sidebar(c)
		ui.Column(c).Grow(1).MinWidth(0).AlignItems(ui.Stretch).Children(func() {
			a.toolbar(c)
			ui.Row(c).Grow(1).MinHeight(0).AlignItems(ui.Stretch).Children(func() {
				ui.Column(c).Grow(1).MinWidth(0).AlignItems(ui.Stretch).Children(func() {
					a.mainContent(c)
				})
				if a.lib != nil && !a.settings.HideDetail {
					a.detailPanel(c)
				}
			})
		})
		if dragOver {
			a.dropOverlay(c)
		}
	})
	a.dialogs(c)
}

func (a *App) shortcuts(c *ui.Context) {
	c.OnWindowShortcut(ui.Cmd, ui.KeyO, func() {
		if a.lib != nil {
			a.chooseBooks()
		}
	})
	c.OnWindowShortcut(ui.Cmd, ui.KeyF, func() { a.searchFocus.Focus() })
	c.OnWindowShortcut(ui.Cmd, ui.KeyK, func() { a.searchFocus.Focus() })
	if a.anyDialog() {
		return
	}
	c.OnWindowShortcut(ui.Cmd, ui.KeyI, func() { a.openEditor(a.current()) })
	c.OnWindowShortcut(ui.Cmd|ui.Shift, ui.KeyC, func() { a.openConvert(a.selectedIDs()) })
	c.OnWindowShortcut(ui.Cmd|ui.Alt, ui.KeyI, func() {
		a.settings.HideDetail = !a.settings.HideDetail
		a.saveSettings()
	})
	if !c.FocusTakesText() {
		if c.Shortcut(0, ui.KeyDelete) || c.Shortcut(0, ui.KeyBackspace) {
			a.askDelete(a.selectedIDs())
		}
	}
}

func (a *App) anyDialog() bool { return a.editOpen || a.convertOpen || a.deleteOpen || a.createAsk }

// ---- Sidebar

func (a *App) sidebar(c *ui.Context) {
	bar := c.TitleBar()
	ui.Column(c).Width(theme.SidebarWidth).Background(pal().BgSidebar).BorderWidth(0, 1, 0, 0).BorderColor(pal().BorderSubtle).AlignItems(ui.Stretch).Children(func() {
		// Room for macOS's traffic lights; a place to drag the window by.
		top := float32(theme.Space12)
		if bar.Left > 0 {
			top = bar.Height
		}
		ui.Box(c).Height(top).DragWindow()
		ui.Box(c).Padding(0, theme.Space8, theme.Space8, theme.Space8).Children(func() { a.librarySwitcher(c) })
		ui.Scroll(c).Grow(1).Padding(theme.Space8, theme.Space8, theme.Space16, theme.Space8).Gap(theme.Space20).AlignItems(ui.Stretch).Children(func() {
			if a.lib == nil {
				return
			}
			ui.Column(c).Gap(theme.Space2).AlignItems(ui.Stretch).Children(func() {
				a.navItem(c, "all", icons.Library, "All books", a.counts.All, a.query.Shelf == catalog.AllBooks && a.query.Filter.Kind == catalog.NoFilter, nil, func() {
					a.setQuery(catalog.AllBooks, catalog.Filter{})
				})
				a.navItem(c, "recent", icons.Clock, "Recently added", a.counts.Recent, a.query.Shelf == catalog.RecentlyAdded && a.query.Filter.Kind == catalog.NoFilter, nil, func() {
					a.setQuery(catalog.RecentlyAdded, catalog.Filter{})
				})
			})
			a.navSection(c, "formats", "Formats", catalog.ByFormat, a.counts.Formats, icons.File, 0)
			a.navSection(c, "authors", "Authors", catalog.ByAuthor, a.counts.Authors, icons.User, 40)
			a.navSection(c, "series", "Series", catalog.BySeries, a.counts.Series, icons.Layers, 40)
			a.navSection(c, "tags", "Tags", catalog.ByTag, a.counts.Tags, icons.Tag, 60)
			a.navSection(c, "publishers", "Publishers", catalog.ByPublisher, a.counts.Publishers, icons.Building, 40)
			a.navSection(c, "languages", "Languages", catalog.ByLanguage, a.counts.Languages, icons.Globe, 0)
		})
		a.sidebarFooter(c)
	})
}

func (a *App) setQuery(shelf catalog.Shelf, f catalog.Filter) {
	a.query.Shelf, a.query.Filter = shelf, f
	a.curID = 0
	a.chosen.Clear()
}

func (a *App) navItem(c *ui.Context, key string, icon *ui.SVG, label string, count int, selected bool, iconColor *ui.Color, onClick func()) {
	row := ui.ButtonBase(c.Key(key)).Height(30).Radius(theme.RadiusSM).Padding(0, theme.Space8).Gap(theme.Space8).AlignItems(ui.Center).Label(label)
	fg := pal().Icon
	switch {
	case selected:
		row.Background(pal().SurfaceRaised).Shadow(0, 1, 2, 0, ui.RGBA(26, 23, 18, 0.06)).Shadow(0, 0, 0, 1, pal().BorderSubtle.Alpha(0.6))
		fg = pal().TextAccent
	case row.Hovered():
		row.Background(pal().Hover)
	}
	if iconColor != nil {
		fg = *iconColor
	}
	row.OnClick(onClick).Children(func() {
		ui.Icon(c, icon).FontSize(16).TextColor(fg)
		t := ui.Text(c, label).Grow(1).SingleLine().TextColor(pal().Text)
		if selected {
			t.FontWeight(500)
		}
		if count > 0 {
			ui.Textf(c, "%s", groupDigits(count)).Font(theme.FontMono).FontSize(12).TextColor(pal().TextTertiary)
		}
	})
}

// navSection is a collapsible group of filters; limit caps how many rows
// show before "Show all".
func (a *App) navSection(c *ui.Context, key, title string, kind catalog.FilterKind, items []catalog.Count, icon *ui.SVG, limit int) {
	if len(items) == 0 {
		return
	}
	open := a.sectionsOn[key]
	ui.Column(c.Key(key)).Gap(theme.Space2).AlignItems(ui.Stretch).Children(func() {
		h := ui.ButtonBase(c).Height(24).Padding(0, theme.Space8).AlignItems(ui.Center).Radius(theme.RadiusXS).Label(title)
		h.OnClick(func() { a.sectionsOn[key] = !a.sectionsOn[key] }).Children(func() {
			widgets.Overline(c, strings.ToUpper(title)).Grow(1)
			if h.Hovered() || !open {
				ui.Textf(c, "%d", len(items)).Font(theme.FontMono).FontSize(11).TextColor(pal().TextTertiary)
			}
			chev := icons.ChevronDown
			if !open {
				chev = icons.ChevronRight
			}
			ui.Icon(c, chev).FontSize(12).TextColor(pal().IconMuted)
		})
		if !open {
			return
		}
		all := a.sectionsOn[key+":all"]
		for i, it := range items {
			if limit > 0 && i >= limit && !all {
				more := ui.ButtonBase(c).Height(26).Padding(0, theme.Space8, 0, theme.Space32).AlignItems(ui.Center).Radius(theme.RadiusSM).Label("Show all")
				if more.Hovered() {
					more.Background(pal().Hover)
				}
				more.OnClick(func() { a.sectionsOn[key+":all"] = true }).Children(func() {
					ui.Textf(c, "Show all %d", len(items)).FontSize(12).TextColor(pal().TextAccent)
				})
				break
			}
			value := it.Value
			label := value
			var col *ui.Color
			if kind == catalog.ByFormat {
				fg := pal().Format(value).FG
				col = &fg
			}
			if kind == catalog.ByLanguage {
				label = languageName(value)
			}
			sel := a.query.Filter == catalog.Filter{Kind: kind, Value: value}
			a.navItem(c, key+":"+value, icon, label, it.N, sel, col, func() {
				a.setQuery(catalog.AllBooks, catalog.Filter{Kind: kind, Value: value})
			})
		}
	})
}

func (a *App) librarySwitcher(c *ui.Context) {
	name, sub := "No library", "Open or create one"
	if a.lib != nil {
		name = filepath.Base(a.lib.Root)
		sub = fmt.Sprintf("%s %s", groupDigits(a.counts.All), plural(a.counts.All, "book", "books"))
	}
	b := ui.ButtonBase(c).Padding(theme.Space8).Gap(theme.Space10).AlignItems(ui.Center).Radius(theme.RadiusMD).
		Background(pal().SurfaceRaised).Border(1, pal().BorderSubtle).Shadow(0, 1, 2, 0, ui.RGBA(26, 23, 18, 0.05)).Label("Library")
	if b.Hovered() {
		b.Border(1, pal().Border)
	}
	b.PopoverMenuButton(func(m *ui.Menu) {
		if m.Item("Open library…").Chosen() {
			a.chooseLibrary()
		}
		for _, r := range a.settings.Recent {
			if a.lib != nil && r == a.lib.Root {
				continue
			}
			if m.Item(filepath.Base(r)).Subtitle(r).Chosen() {
				a.openLibrary(r)
			}
		}
		if a.lib != nil {
			m.Separator()
			if m.Item("Show in folder").Chosen() {
				revealPath(a.lib.Root)
			}
		}
	})
	b.Children(func() {
		ui.Box(c).Size(28, 28).Radius(theme.RadiusSM).Background(pal().Inverse).Center().Children(func() {
			ui.Text(c, "D").FontWeight(700).FontSize(16).TextColor(pal().TextOnInverse)
		})
		ui.Column(c).Grow(1).MinWidth(0).Children(func() {
			ui.Text(c, name).FontSize(13).FontWeight(500).SingleLine()
			ui.Text(c, sub).FontSize(12).TextColor(pal().TextTertiary).SingleLine()
		})
		ui.Icon(c, icons.ChevronDown).FontSize(14).TextColor(pal().IconMuted)
	})
}

func (a *App) sidebarFooter(c *ui.Context) {
	if a.busy == "" && len(a.jobs) == 0 {
		return
	}
	ui.Column(c).Padding(0, theme.Space8, theme.Space12, theme.Space8).Gap(theme.Space8).AlignItems(ui.Stretch).Children(func() {
		widgets.Hairline(c)
		if a.busy != "" {
			ui.Row(c).Padding(theme.Space4, theme.Space8).Gap(theme.Space10).AlignItems(ui.Center).Children(func() {
				ui.Spinner(c).Size(16, 16).Label(a.busy)
				ui.Column(c).Grow(1).Gap(theme.Space6).AlignItems(ui.Stretch).Children(func() {
					label := a.busy + "…"
					if a.busyTotal > 0 {
						label = fmt.Sprintf("%s %d of %d", a.busy, a.busyDone, a.busyTotal)
					}
					ui.Text(c, label).FontSize(12).FontWeight(500).SingleLine()
					v := -1.0
					if a.busyTotal > 0 {
						v = float64(a.busyDone) / float64(a.busyTotal)
					}
					ui.Progress(c, v).Height(4)
				})
			})
		}
		if len(a.jobs) > 0 {
			a.jobsButton(c)
		}
	})
}

// ---- Toolbar

func (a *App) toolbar(c *ui.Context) {
	bar := c.TitleBar()
	ui.Row(c).Height(theme.ToolbarHeight).Padding(0, theme.Space24+bar.Right, 0, theme.Space32).Gap(theme.Space8).AlignItems(ui.Center).DragWindow().Children(func() {
		title, count := "Dalibre", ""
		if a.lib != nil {
			title, count = a.viewTitle(), groupDigits(len(a.visible()))
		}
		ui.Row(c).Grow(1).MinWidth(0).Gap(theme.Space8).AlignItems(ui.Baseline).Children(func() {
			ui.Text(c, title).FontWeight(600).FontSize(22).LetterSpacing(-0.4).SingleLine().Shrink(1)
			if count != "" {
				ui.Text(c, count).Font(theme.FontMono).FontSize(12).TextColor(pal().TextTertiary)
			}
		})
		if a.lib == nil {
			return
		}
		ui.SearchField(c, &a.query.Search).Placeholder("Search title, author, tag…").Label("Search library").Width(260).Bind(&a.searchFocus).OnChange(func() {
			a.curID = 0
		})
		a.viewToggle(c)
		sort := widgets.IconButton(c, icons.Sort, "Sort: "+a.query.Sort.Label(), false)
		sort.PopoverMenuButton(func(m *ui.Menu) {
			for _, k := range catalog.SortKeys {
				if m.Item(k.Label).Radio(a.query.Sort == k.Key).Chosen() {
					a.query.Sort = k.Key
					a.saveSettings()
				}
			}
			m.Separator()
			if m.Item("Ascending").Radio(!a.query.Descending).Chosen() {
				a.query.Descending = false
				a.saveSettings()
			}
			if m.Item("Descending").Radio(a.query.Descending).Chosen() {
				a.query.Descending = true
				a.saveSettings()
			}
		})
		if widgets.IconButton(c, icons.PanelRight, "Details (Ctrl+Alt+I)", !a.settings.HideDetail).Clicked() {
			a.settings.HideDetail = !a.settings.HideDetail
			a.saveSettings()
		}
		ui.Box(c).Size(1, 20).Background(pal().Border).Margin(0, theme.Space4)
		if widgets.Button(c, widgets.Primary, icons.Plus, "Add books").Tooltip("Add books (Ctrl+O)").Disabled(a.busy != "").Clicked() {
			a.chooseBooks()
		}
	})
}

func (a *App) viewTitle() string {
	f := a.query.Filter
	switch f.Kind {
	case catalog.ByLanguage:
		return languageName(f.Value)
	case catalog.NoFilter:
		if a.query.Shelf == catalog.RecentlyAdded {
			return "Recently added"
		}
		return "All books"
	}
	return f.Value
}

// viewToggle is the ViewToggle component: grid or list.
func (a *App) viewToggle(c *ui.Context) {
	sel := 0
	if a.settings.ListView {
		sel = 1
	}
	seg := ui.SegmentedBase(c, &sel, 2)
	seg.Track.Padding(theme.Space2).Gap(theme.Space2).Radius(theme.RadiusSM).Background(pal().Subtle).Label("View").Children(func() {
		for i, ic := range []*ui.SVG{icons.Grid, icons.List} {
			s := seg.Segment(i).Size(36, 28).Radius(theme.RadiusXS).Center().Label([]string{"Grid", "List"}[i])
			fg := pal().IconMuted
			if i == sel {
				s.Background(pal().SurfaceRaised).Shadow(0, 1, 2, 0, ui.RGBA(26, 23, 18, 0.08))
				fg = pal().Text
			} else if s.Hovered() {
				fg = pal().Text
			}
			s.TextColor(fg).Children(func() { ui.Icon(c, ic).FontSize(16) })
		}
	})
	if (sel == 1) != a.settings.ListView {
		a.settings.ListView = sel == 1
		a.saveSettings()
	}
}

// ---- Main content

func (a *App) mainContent(c *ui.Context) {
	switch {
	case a.loading:
		ui.Column(c).Grow(1).Center().Gap(theme.Space12).Children(func() {
			ui.Spinner(c).Size(24, 24).Label("Loading")
			ui.Text(c, "Opening library…").TextColor(pal().TextSecondary)
		})
		return
	case a.lib == nil:
		a.welcome(c)
		return
	}
	a.filterBar(c)
	shown := a.visible()
	if len(shown) == 0 {
		a.emptyState(c)
		return
	}
	a.syncCur(shown)
	if a.settings.ListView {
		a.listView(c, shown)
	} else {
		a.gridView(c, shown)
	}
	a.adoptCur()
}

func (a *App) filterBar(c *ui.Context) {
	q := a.query
	if q.Filter.Kind == catalog.NoFilter && q.Search == "" {
		return
	}
	ui.Row(c).Padding(0, theme.Space32, theme.Space16, theme.Space32).Gap(theme.Space8).AlignItems(ui.Center).Wrap().Children(func() {
		ui.Text(c, "Filtered by").FontSize(12).FontWeight(500).TextColor(pal().TextTertiary)
		if q.Filter.Kind != catalog.NoFilter {
			label := filterKindName(q.Filter.Kind) + ": " + q.Filter.Value
			if q.Filter.Kind == catalog.ByLanguage {
				label = "Language: " + languageName(q.Filter.Value)
			}
			widgets.Tag(c.Key("filter"), label, true, func() { a.setQuery(a.query.Shelf, catalog.Filter{}) })
		}
		if q.Search != "" {
			widgets.Tag(c.Key("search"), "“"+q.Search+"”", true, func() { a.query.Search = "" })
		}
		clear := ui.ButtonBase(c).Padding(theme.Space2, theme.Space4).Radius(theme.RadiusXS).Label("Clear all")
		clear.OnClick(func() {
			a.query.Search = ""
			a.setQuery(catalog.AllBooks, catalog.Filter{})
		}).Children(func() {
			t := ui.Text(c, "Clear all").FontSize(12).FontWeight(500).TextColor(pal().TextAccent)
			if clear.Hovered() {
				t.Underline()
			}
		})
	})
}

func filterKindName(k catalog.FilterKind) string {
	return map[catalog.FilterKind]string{catalog.ByFormat: "Format", catalog.ByAuthor: "Author", catalog.BySeries: "Series",
		catalog.ByTag: "Tag", catalog.ByPublisher: "Publisher", catalog.ByLanguage: "Language"}[k]
}

func (a *App) emptyState(c *ui.Context) {
	ui.Column(c).Grow(1).Center().Gap(theme.Space12).Padding(theme.Space48).Children(func() {
		ui.Box(c).Size(56, 56).Radius(theme.RadiusFull).Background(pal().Subtle).Center().Children(func() {
			ic := icons.Search
			if len(a.books) == 0 {
				ic = icons.BookOpen
			}
			ui.Icon(c, ic).FontSize(24).TextColor(pal().IconMuted)
		})
		if len(a.books) == 0 {
			ui.Text(c, "Your library is empty").FontWeight(600).FontSize(18)
			ui.Text(c, "Drop EPUB, PDF or MOBI files here, or add them from disk.").TextColor(pal().TextSecondary)
			if widgets.Button(c, widgets.Primary, icons.Plus, "Add books").Margin(theme.Space8, 0, 0, 0).Clicked() {
				a.chooseBooks()
			}
			return
		}
		ui.Text(c, "No books match").FontWeight(600).FontSize(18)
		ui.Text(c, "Try another search or clear the filters.").TextColor(pal().TextSecondary)
		if widgets.Button(c, widgets.Secondary, nil, "Clear filters").Margin(theme.Space8, 0, 0, 0).Clicked() {
			a.query.Search = ""
			a.setQuery(catalog.AllBooks, catalog.Filter{})
		}
	})
}

func (a *App) welcome(c *ui.Context) {
	ui.Column(c).Grow(1).Center().Gap(theme.Space16).Padding(theme.Space48).Children(func() {
		ui.Text(c, "DALIBRE").FontSize(11).FontWeight(600).LetterSpacing(1.2).TextColor(pal().TextAccent)
		ui.Text(c, "A calm library for your books.").FontWeight(600).FontSize(28).LetterSpacing(-0.6).TextAlign(ui.Center)
		ui.Text(c, "Open an existing calibre library, or pick an empty folder to start a new one.").TextColor(pal().TextSecondary).TextAlign(ui.Center).MaxWidth(420)
		if a.loadErr != "" {
			ui.Row(c).Padding(theme.Space8, theme.Space12).Gap(theme.Space8).Radius(theme.RadiusMD).Background(pal().Danger.BG).AlignItems(ui.Center).MaxWidth(520).Children(func() {
				ui.Icon(c, icons.Alert).FontSize(16).TextColor(pal().Danger.FG)
				ui.Text(c, a.loadErr).FontSize(13).TextColor(pal().Danger.FG)
			})
		}
		ui.Row(c).Gap(theme.Space8).Margin(theme.Space8, 0, 0, 0).Children(func() {
			if widgets.Button(c, widgets.Primary, icons.Folder, "Open library…").Clicked() {
				a.chooseLibrary()
			}
		})
		if len(a.settings.Recent) > 0 {
			ui.Column(c).Gap(theme.Space4).Margin(theme.Space24, 0, 0, 0).Width(420).AlignItems(ui.Stretch).Children(func() {
				widgets.Overline(c, "RECENT").Margin(0, 0, theme.Space4, theme.Space8)
				for _, r := range a.settings.Recent {
					row := ui.ButtonBase(c.Key(r)).Padding(theme.Space8).Gap(theme.Space10).Radius(theme.RadiusMD).AlignItems(ui.Center).Label(r)
					if row.Hovered() {
						row.Background(pal().Hover)
					}
					row.OnClick(func() { a.openLibrary(r) }).Children(func() {
						ui.Icon(c, icons.Database).FontSize(16).TextColor(pal().IconMuted)
						ui.Column(c).Grow(1).MinWidth(0).Children(func() {
							ui.Text(c, filepath.Base(r)).FontWeight(500).SingleLine()
							ui.Text(c, r).Font(theme.FontMono).FontSize(11).TextColor(pal().TextTertiary).SingleLine()
						})
					})
				}
			})
		}
	})
}

func (a *App) dropOverlay(c *ui.Context) {
	ui.Box(c).Absolute().Top(theme.Space12).Left(theme.Space12).Right(theme.Space12).Bottom(theme.Space12).PassThrough().
		Radius(theme.RadiusXL).Border(2, pal().Accent).BorderStyle(ui.BorderDashed).Background(pal().Accent.Alpha(0.06)).Center().Children(func() {
		ui.Column(c).Center().Gap(theme.Space8).Padding(theme.Space24).Radius(theme.RadiusLG).Background(pal().SurfaceRaised).Shadow(0, 8, 24, 0, ui.RGBA(0, 0, 0, 0.12)).Children(func() {
			ui.Icon(c, icons.Upload).FontSize(24).TextColor(pal().TextAccent)
			ui.Text(c, "Drop to add to the library").FontWeight(500)
		})
	})
}

func groupDigits(n int) string {
	s := fmt.Sprint(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}

var languageNames = map[string]string{
	"eng": "English", "zho": "Chinese", "jpn": "Japanese", "kor": "Korean", "fra": "French", "deu": "German",
	"spa": "Spanish", "ita": "Italian", "por": "Portuguese", "rus": "Russian", "ara": "Arabic", "nld": "Dutch",
	"swe": "Swedish", "pol": "Polish", "tur": "Turkish", "lat": "Latin", "und": "Undetermined",
}

func languageName(code string) string {
	if n, ok := languageNames[code]; ok {
		return n
	}
	return code
}

// pal is the palette of the frame being built.
func pal() *theme.Palette { return theme.P() }
