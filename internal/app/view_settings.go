package app

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"

	"dalibre/internal/app/icons"
	"dalibre/internal/app/theme"
	"dalibre/internal/app/widgets"
	"dalibre/internal/plugin"
	"dalibre/internal/settings"
)

// The Settings window (Figma "08 · OPDS & Settings"): one window for all
// of Dalibre's settings, in sections. ⌘, or Settings at the sidebar's
// foot opens it; other places open it at the section they are about.

type settingsSection int

const (
	sectionGeneral settingsSection = iota
	sectionLibrary
	sectionOPDS
	sectionPlugins
)

var sections = []struct {
	section settingsSection
	title   string
	icon    *ui.SVG
}{
	{sectionGeneral, "General", icons.Sliders},
	{sectionLibrary, "Library", icons.Library},
	{sectionOPDS, "OPDS", icons.Rss},
	{sectionPlugins, "Plugins", icons.Puzzle},
}

// settingsWidth is the Settings window's width: Plugins' list and plugin
// beside the sections.
const settingsWidth = 920

// openSettings opens the Settings window at section.
func (a *App) openSettings(s settingsSection) {
	a.settingsOpen, a.section = true, s
	if s == sectionPlugins {
		a.checkUpdates(false)
	}
}

func (a *App) settingsDialog(c *ui.Context) {
	if !a.settingsOpen {
		return
	}
	// One width for every section, the widest's: the window doesn't jump
	// as the user moves between them.
	widgets.CompactDialogFrame(c, &a.settingsOpen, settingsWidth, func() {
		ui.Text(c, "Settings").FontSize(14).FontWeight(600).Grow(1)
	}, func() {
		ui.Row(c).Height(540).AlignItems(ui.Stretch).Children(func() {
			ui.Column(c).Width(168).Padding(theme.Space8).Gap(theme.Space2).Background(pal().BgApp).BorderWidth(0, 1, 0, 0).BorderColor(pal().BorderSubtle).AlignItems(ui.Stretch).Children(func() {
				for _, s := range sections {
					a.navItem(c, "settings:"+s.title, s.icon, s.title, 0, a.section == s.section, nil, func() { a.openSettings(s.section) })
				}
			})
			switch a.section {
			case sectionGeneral:
				a.generalPane(c)
			case sectionLibrary:
				a.libraryPane(c)
			case sectionOPDS:
				a.opdsPane(c)
			case sectionPlugins:
				a.pluginsPane(c)
			}
		})
	}, func() {
		note := "Changes apply right away."
		if a.section == sectionPlugins {
			note = fmt.Sprintf("Plugins are sandboxed: only the sites they list · %d MB · %d s per call", plugin.MaxMemory>>20, int(plugin.CallTimeout.Seconds()))
		}
		ui.Text(c, note).FontSize(12).TextColor(pal().TextTertiary).Grow(1)
		if widgets.ButtonSized(c, widgets.Primary, nil, "Done", theme.ControlS).Clicked() {
			a.settingsOpen = false
		}
	})
	a.catalogSheet(c)
	a.githubSheet(c)
}

// pane is a section's scrolling column.
func pane(c *ui.Context, fn func()) {
	ui.Scroll(c).Grow(1).MinWidth(0).Padding(theme.Space20).Gap(theme.Space16).AlignItems(ui.Stretch).Children(fn)
}

// labeled is a control with its label above, in a column; widgets.Field
// is for a row of fields.
func labeled(c *ui.Context, label string, control func()) {
	ui.Column(c.Key(label)).Gap(theme.Space6).AlignItems(ui.Stretch).Children(func() {
		ui.Text(c, label).FontSize(12).FontWeight(500).TextColor(pal().TextSecondary)
		control()
	})
}

// heading is a group's title and what it is for.
func heading(c *ui.Context, title, body string) {
	ui.Column(c).Grow(1).MinWidth(0).Gap(theme.Space4).Children(func() {
		ui.Text(c, title).FontSize(13).FontWeight(600)
		if body != "" {
			ui.Text(c, body).FontSize(12).TextColor(pal().TextSecondary)
		}
	})
}

// ---- General

func (a *App) generalPane(c *ui.Context) {
	pane(c, func() {
		ui.Row(c).Gap(theme.Space16).AlignItems(ui.Center).Children(func() {
			heading(c, "Show books as a list", "A table of titles, authors and sizes, in place of covers.")
			on := a.settings.ListView
			ui.Switch(c, &on).Label("Show books as a list")
			if on != a.settings.ListView {
				a.settings.ListView = on
				a.saveSettings()
			}
		})
		widgets.Hairline(c)
		ui.Row(c).Gap(theme.Space16).AlignItems(ui.Center).Children(func() {
			heading(c, "Show the details panel", "The book or result chosen, at the window’s right. ⌘⌥I turns it on and off.")
			on := !a.settings.HideDetail
			ui.Switch(c, &on).Label("Show the details panel")
			if on == a.settings.HideDetail {
				a.settings.HideDetail = !on
				a.saveSettings()
			}
		})
	})
}

// ---- Library

func (a *App) libraryPane(c *ui.Context) {
	pane(c, func() {
		heading(c, "Library", "Dalibre reads and writes calibre libraries: calibre can open the same folder.")
		ui.Row(c).Padding(theme.Space6, theme.Space6, theme.Space6, theme.Space12).Gap(theme.Space10).Radius(theme.RadiusMD).Background(pal().Subtle).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, icons.Library).FontSize(16).TextColor(pal().Icon)
			ui.Column(c).Grow(1).MinWidth(0).Gap(theme.Space2).Children(func() {
				if a.lib == nil {
					ui.Text(c, "No library is open").FontSize(13).FontWeight(500)
					return
				}
				ui.Text(c, a.lib.Root).Font(theme.FontMono).FontSize(12).SingleLine().Selectable()
				ui.Textf(c, "%s %s", groupDigits(a.counts.All), plural(a.counts.All, "book", "books")).FontSize(12).TextColor(pal().TextTertiary)
			})
			if a.lib != nil {
				if widgets.ButtonSized(c, widgets.Ghost, icons.Folder, revealLabel, theme.ControlS).Clicked() {
					revealPath(a.lib.Root)
				}
			}
		})
		ui.Row(c).Gap(theme.Space8).Children(func() {
			if widgets.Button(c, widgets.Secondary, icons.Folder, "Open another library…").Clicked() {
				a.chooseLibrary()
			}
		})
	})
}

// ---- OPDS

// catalogEdit is the Add or Edit catalog sheet.
type catalogEdit struct {
	open     bool
	id       string // the catalog edited; "" adds one
	url      string
	name     string
	user     string
	password string
	probe    *catalogProbe
	named    bool // the user typed the name
}

// shareEdit is Settings › OPDS's sharing fields, as edited.
type shareEdit struct {
	loaded         bool
	port, user, pw string
}

func (a *App) opdsPane(c *ui.Context) {
	pane(c, func() {
		ui.Column(c).Gap(theme.Space10).AlignItems(ui.Stretch).Children(func() {
			ui.Row(c).Gap(theme.Space16).AlignItems(ui.Start).Children(func() {
				heading(c, "Catalogs", "Browse and download from OPDS catalogs: a calibre, Calibre-Web or Kavita server at home, or public ones like Standard Ebooks. Each shows up in the sidebar.")
				if widgets.ButtonSized(c, widgets.Secondary, icons.Plus, "Add catalog…", theme.ControlS).Clicked() {
					a.openCatalogSheet("")
				}
			})
			for _, cat := range a.settings.OPDS.Catalogs {
				a.catalogRow(c.Key(cat.ID), cat)
			}
			if len(a.settings.OPDS.Catalogs) == 0 {
				ui.Text(c, "No catalogs yet.").FontSize(13).TextColor(pal().TextTertiary)
			}
			in := !a.settings.OPDS.NotInGetBooks
			ui.Checkbox(c, &in, "Search catalogs from Get books, as sources")
			if in == a.settings.OPDS.NotInGetBooks {
				a.settings.OPDS.NotInGetBooks = !in
				a.saveSettings()
				a.setPlugins(a.plugins)
			}
		})
		widgets.Hairline(c)
		a.sharePane(c)
	})
}

// catalogRow is the CatalogRow component.
func (a *App) catalogRow(c *ui.Context, cat *settings.Catalog) {
	st := a.catalogStatus[cat.ID]
	label, tone := st.text()
	col := pal().TextTertiary
	switch tone {
	case "success":
		col = pal().Success.FG
	case "warning":
		col = pal().Warning.FG
	case "danger":
		col = pal().Danger.FG
	}
	row := ui.ButtonBase(c).Padding(theme.Space6, theme.Space6, theme.Space6, theme.Space12).Gap(theme.Space10).Radius(theme.RadiusMD).
		Background(pal().Surface).Border(1, pal().BorderSubtle).AlignItems(ui.Center).Label(cat.Name)
	if st != nil && st.err != nil {
		row.Tooltip(errText(st.err))
	}
	row.OnClick(func() { a.openCatalogSheet(cat.ID) }).Children(func() {
		ui.Box(c).Size(28, 28).Radius(theme.RadiusSM).Background(pal().AccentSubtle).Center().Children(func() {
			ui.Icon(c, icons.Rss).FontSize(16).TextColor(pal().TextAccent)
		})
		ui.Column(c).Grow(1).MinWidth(0).Gap(theme.Space2).Children(func() {
			ui.Text(c, cat.Name).FontSize(13).FontWeight(500).SingleLine()
			ui.Text(c, cat.URL).Font(theme.FontMono).FontSize(12).TextColor(pal().TextTertiary).SingleLine()
		})
		ui.Row(c).Gap(theme.Space6).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Size(6, 6).Radius(theme.RadiusFull).Background(col)
			ui.Text(c, label).FontSize(12).TextColor(col)
		})
		more := ui.ButtonBase(c).Size(theme.ControlM, theme.ControlM).Radius(theme.RadiusSM).Center().Label("More: " + cat.Name)
		if more.Hovered() {
			more.Background(pal().Hover)
		}
		more.PopoverMenuButton(func(m *ui.Menu) {
			if m.Item("Open").Icon(icons.Menu(icons.BookOpen)).Chosen() {
				a.settingsOpen = false
				a.openCatalog(cat.ID)
			}
			if m.Item("Edit…").Icon(icons.Menu(icons.Pencil)).Chosen() {
				a.openCatalogSheet(cat.ID)
			}
			if m.Item("Check again").Icon(icons.Menu(icons.Refresh)).Chosen() {
				if p := a.catalogByID(cat.ID); p != nil {
					a.checkCatalog(p)
				}
			}
			m.Separator()
			if m.Item("Remove").Icon(icons.Menu(icons.Trash)).Danger().Chosen() {
				a.removeCatalog(cat.ID)
			}
		})
		more.Children(func() { ui.Icon(c, icons.More).FontSize(16).TextColor(pal().Icon) })
	})
}

// openCatalogSheet opens the sheet adding a catalog (id "") or editing
// one.
func (a *App) openCatalogSheet(id string) {
	a.catEdit = catalogEdit{open: true, id: id}
	if cat := a.settings.OPDS.Catalog(id); cat != nil {
		a.catEdit.url, a.catEdit.name, a.catEdit.user, a.catEdit.named = cat.URL, cat.Name, cat.Username, true
		if cat.Username != "" && a.secrets != nil {
			secrets := a.secrets
			a.run(func() {
				pw, _ := secrets.Get(catalogSecret(id))
				a.post(func() {
					if a.catEdit.id == id && a.catEdit.password == "" {
						a.catEdit.password = pw
					}
				})
			})
		}
	}
	if !a.settingsOpen {
		a.openSettings(sectionOPDS)
	}
}

// checkCatalogSheet reads the address the sheet holds.
func (a *App) checkCatalogSheet() {
	e := &a.catEdit
	u := strings.TrimSpace(e.url)
	if u == "" {
		return
	}
	if !strings.Contains(u, "://") {
		u = "https://" + u
		e.url = u
	}
	e.probe = a.probeCatalog(u, strings.TrimSpace(e.user), e.password)
}

func (a *App) catalogSheet(c *ui.Context) {
	e := &a.catEdit
	if !e.open {
		return
	}
	if pr := e.probe; pr != nil && !pr.checking && pr.err == nil && !e.named && pr.title != "" {
		e.name = pr.title
	}
	title := "Add OPDS catalog"
	if e.id != "" {
		title = "Edit catalog"
	}
	widgets.DialogFrame(c, &e.open, 480, func() {
		ui.Column(c).Grow(1).Gap(theme.Space4).Children(func() {
			ui.Text(c, title).FontSize(16).FontWeight(600)
			ui.Text(c, "Paste the address of a catalog. Most servers show it on their OPDS or “e-reader” page.").FontSize(13).TextColor(pal().TextSecondary)
		})
	}, func() {
		ui.Column(c).Padding(theme.Space16, theme.Space24, theme.Space20, theme.Space24).Gap(theme.Space12).AlignItems(ui.Stretch).Children(func() {
			labeled(c, "Address", func() {
				in := ui.TextInput(c, &e.url).Label("Address").Placeholder("https://example.com/opds").AutoFocus()
				if in.Submitted() {
					a.checkCatalogSheet()
				}
				if in.Changed() {
					e.probe = nil
				}
			})
			a.probeResult(c)
			labeled(c, "Name", func() {
				before := e.name
				ui.TextInput(c, &e.name).Label("Name").Placeholder("Home server")
				if e.name != before {
					e.named = true
				}
				ui.Text(c, "Shown in the sidebar. Taken from the catalog; change it if you like.").FontSize(12).TextColor(pal().TextTertiary)
			})
			ui.Row(c).Gap(theme.Space12).Children(func() {
				widgets.Field(c, "Username", "If it asks", func() {
					ui.TextInput(c, &e.user).Label("Username")
				})
				widgets.Field(c, "Password", "", func() {
					ui.TextInput(c, &e.password).Label("Password").Password()
				})
			})
			if pr := e.probe; pr != nil && pr.local {
				ui.Row(c).Padding(theme.Space10, theme.Space12).Gap(theme.Space10).Radius(theme.RadiusMD).Background(pal().Subtle).AlignItems(ui.Start).Children(func() {
					ui.Icon(c, icons.Network).FontSize(14).TextColor(pal().Icon)
					ui.Column(c).Grow(1).Gap(theme.Space2).Children(func() {
						ui.Text(c, "On your local network").FontSize(13).FontWeight(500)
						ui.Text(c, "This address is on your network, so Dalibre will reach it only for this catalog. Passwords are kept in the system’s keychain.").FontSize(12).TextColor(pal().TextSecondary)
					})
				})
			}
		})
	}, func() {
		checking := e.probe != nil && e.probe.checking
		if widgets.Button(c, widgets.Ghost, icons.Refresh, map[bool]string{true: "Check again", false: "Check"}[e.probe != nil]).Disabled(checking || strings.TrimSpace(e.url) == "").Clicked() {
			a.checkCatalogSheet()
		}
		ui.Spacer(c)
		if widgets.Button(c, widgets.Secondary, nil, "Cancel").Clicked() {
			e.open = false
		}
		ok := e.probe != nil && !e.probe.checking && e.probe.url == strings.TrimSpace(e.url) && (e.probe.err == nil || errKind(e.probe.err) == plugin.KindAuth && e.user != "")
		label := "Add catalog"
		if e.id != "" {
			label = "Save"
			ok = ok || e.probe == nil && strings.TrimSpace(e.url) != ""
		}
		if widgets.Button(c, widgets.Primary, nil, label).Disabled(!ok || strings.TrimSpace(e.name) == "").Clicked() {
			local := e.probe != nil && e.probe.local
			if e.probe == nil {
				if old := a.settings.OPDS.Catalog(e.id); old != nil {
					local = old.LocalNetwork
				}
			}
			a.saveCatalog(settings.Catalog{ID: e.id, Name: strings.TrimSpace(e.name), URL: strings.TrimSpace(e.url),
				Username: strings.TrimSpace(e.user), LocalNetwork: local}, e.password)
			e.open = false
		}
	})
}

// probeResult says what checking the address found.
func (a *App) probeResult(c *ui.Context) {
	pr := a.catEdit.probe
	if pr == nil {
		return
	}
	bg, fg, ic := pal().Success.BG, pal().Success.FG, icons.CheckCircle
	title, body := "", ""
	switch {
	case pr.checking:
		ui.Row(c).Padding(theme.Space10, theme.Space12).Gap(theme.Space10).Radius(theme.RadiusMD).Background(pal().Subtle).AlignItems(ui.Center).Children(func() {
			ui.Spinner(c).Size(14, 14).Label("Checking")
			ui.Text(c, "Checking the catalog…").FontSize(13).TextColor(pal().TextSecondary)
		})
		return
	case pr.err == nil:
		title = "Found “" + pr.title + "”"
		parts := []string{fmt.Sprintf("%d %s", pr.entries, plural(pr.entries, "entry", "entries"))}
		if pr.searchable {
			parts = append(parts, "search supported")
		} else {
			parts = append(parts, "no search")
		}
		body = strings.Join(parts, " · ")
	case errKind(pr.err) == plugin.KindAuth:
		bg, fg, ic = pal().Warning.BG, pal().Warning.FG, icons.Key
		title, body = "It asks you to sign in", "Fill in the username and password, then check again."
	default:
		bg, fg, ic = pal().Danger.BG, pal().Danger.FG, icons.Alert
		title, body = "That isn’t a catalog Dalibre can read", errText(pr.err)
	}
	ui.Row(c).Padding(theme.Space10, theme.Space12).Gap(theme.Space10).Radius(theme.RadiusMD).Background(bg).AlignItems(ui.Start).Children(func() {
		ui.Icon(c, ic).FontSize(14).TextColor(fg)
		ui.Column(c).Grow(1).MinWidth(0).Gap(theme.Space2).Children(func() {
			ui.Text(c, title).FontSize(13).FontWeight(500).TextColor(fg)
			ui.Text(c, body).FontSize(12).TextColor(pal().TextSecondary).MaxLines(3)
		})
	})
}

// sharePane is Share this library over OPDS.
func (a *App) sharePane(c *ui.Context) {
	sh := &a.settings.OPDS.Share
	ed := &a.shareEdit
	if !ed.loaded {
		ed.loaded = true
		ed.port, ed.user = strconv.Itoa(sh.SharePort()), sh.Username
		if sh.Password && a.secrets != nil {
			secrets := a.secrets
			a.run(func() {
				pw, _ := secrets.Get(shareSecret)
				a.post(func() { ed.pw = pw })
			})
		}
	}
	ui.Column(c).Gap(theme.Space10).AlignItems(ui.Stretch).Children(func() {
		ui.Row(c).Gap(theme.Space16).AlignItems(ui.Start).Children(func() {
			heading(c, "Share this library over OPDS", "Reading apps on your network (KOReader, Moon+ Reader, Thorium, KyBook) can browse this library and download books from it while Dalibre is open.")
			on := sh.On
			ui.Switch(c, &on).Label("Share this library over OPDS")
			if on != sh.On {
				sh.On = on
				a.saveSettings()
				a.applyShare()
			}
		})
		if !sh.On {
			return
		}
		st := a.share
		ui.Row(c).Padding(theme.Space6, theme.Space6, theme.Space6, theme.Space12).Gap(theme.Space10).Radius(theme.RadiusMD).Background(pal().Subtle).AlignItems(ui.Center).Children(func() {
			switch {
			case st != nil && st.err != nil:
				ui.Icon(c, icons.Alert).FontSize(16).TextColor(pal().Danger.FG)
				ui.Text(c, st.err.Error()).FontSize(12).TextColor(pal().Danger.FG).Grow(1)
			case st == nil || st.server == nil:
				ui.Spinner(c).Size(14, 14).Label("Starting")
				ui.Text(c, "Starting…").FontSize(12).TextColor(pal().TextSecondary).Grow(1)
			default:
				addr := a.shareAddress()
				ui.Icon(c, icons.Wifi).FontSize(16).TextColor(pal().Success.FG)
				ui.Column(c).Grow(1).MinWidth(0).Gap(theme.Space2).Children(func() {
					ui.Text(c, addr).Font(theme.FontMono).FontSize(12).Selectable()
					status := "Sharing"
					if a.lib == nil {
						status = "Sharing · open a library to share it"
					} else if n := st.opds.Readers(); n > 0 {
						status += fmt.Sprintf(" · %d %s connected today", n, plural(n, "reader", "readers"))
					}
					ui.Text(c, status).FontSize(12).TextColor(pal().Success.FG)
				})
				if widgets.IconButton(c, icons.Copy, "Copy address", false).Clicked() {
					mygo.Clipboard.Write(transfer.TextData(addr))
					a.toast("Copied %s", addr)
				}
			}
		})
		req := sh.Password
		ui.Checkbox(c, &req, "Require a username and password")
		ui.Row(c).Gap(theme.Space12).AlignItems(ui.End).Children(func() {
			ui.Column(c).Width(120).AlignItems(ui.Stretch).Children(func() {
				labeled(c, "Port", func() { ui.TextInput(c, &ed.port).Label("Port") })
			})
			if req {
				widgets.Field(c, "Username", "", func() { ui.TextInput(c, &ed.user).Label("Username") })
				widgets.Field(c, "Password", "", func() { ui.TextInput(c, &ed.pw).Label("Password").Password() })
			}
		})
		port, perr := strconv.Atoi(strings.TrimSpace(ed.port))
		changed := req != sh.Password || perr == nil && port != sh.SharePort() || req && strings.TrimSpace(ed.user) != sh.Username
		ui.Row(c).Gap(theme.Space8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "Only devices on this network can connect. Books are read from your library as they are; nothing is uploaded anywhere.").
				FontSize(12).TextColor(pal().TextTertiary).Grow(1)
			bad := perr != nil || port < 1 || port > 65535 || req && (strings.TrimSpace(ed.user) == "" || ed.pw == "")
			if widgets.ButtonSized(c, widgets.Primary, nil, "Apply", theme.ControlS).Disabled(bad || !changed && !req).Clicked() {
				sh.Port, sh.Password, sh.Username = port, req, strings.TrimSpace(ed.user)
				a.saveSettings()
				pw, secrets := ed.pw, a.secrets
				a.run(func() {
					if secrets != nil && req {
						secrets.Set(shareSecret, pw)
					}
					a.post(func() {
						a.share = nil // restart with the new password
						a.applyShare()
					})
				})
			}
		})
	})
}

// ---- Plugins from GitHub

// updateBanner offers a plugin's update.
func (a *App) updateBanner(c *ui.Context, m *plugin.Manifest, u *pluginUpdate) {
	ui.Row(c).Padding(theme.Space12, theme.Space12).Gap(theme.Space12).Radius(theme.RadiusMD).Background(pal().AccentSubtle).AlignItems(ui.Start).Children(func() {
		ui.Icon(c, icons.ArrowUpCircle).FontSize(16).TextColor(pal().TextAccent)
		ui.Column(c).Grow(1).MinWidth(0).Gap(theme.Space8).Children(func() {
			ui.Column(c).Gap(theme.Space2).Children(func() {
				ui.Text(c, u.label+" is available").FontSize(13).FontWeight(500).TextColor(pal().TextAccent)
				body := "Published " + ago(u.published) + " on GitHub."
				if n := strings.TrimSpace(u.notes); n != "" {
					body += " " + firstLines(n, 2)
				}
				ui.Text(c, body).FontSize(12).TextColor(pal().TextSecondary).MaxLines(3)
			})
			ui.Row(c).Gap(theme.Space8).AlignItems(ui.Center).Children(func() {
				if u.busy {
					ui.Spinner(c).Size(14, 14).Label("Updating")
					ui.Text(c, "Updating…").FontSize(12).TextColor(pal().TextSecondary)
					return
				}
				if widgets.ButtonSized(c, widgets.Primary, nil, "Update", theme.ControlS).Clicked() {
					a.updatePlugin(m.ID, false)
				}
				if st := a.settings.Plugins[m.ID]; st != nil && st.Source != nil {
					if repo, err := plugin.ParseRepo(st.Source.Repo); err == nil {
						link := repo.URL() + "/releases"
						if st.Source.Branch != "" {
							link = repo.URL() + "/commits/" + url.PathEscape(st.Source.Branch)
						}
						if widgets.ButtonSized(c, widgets.Ghost, icons.External, "Release notes", theme.ControlS).Clicked() {
							openURL(link)
						}
					}
				}
			})
		})
	})
}

func firstLines(s string, n int) string {
	var out []string
	for line := range strings.Lines(s) {
		line = strings.TrimSpace(strings.TrimLeft(line, "#*- "))
		if line != "" {
			out = append(out, line)
		}
		if len(out) == n {
			break
		}
	}
	return strings.Join(out, " ")
}

// pluginSource is SOURCE: the repository a plugin came from, what of it
// it follows, and updating it.
func (a *App) pluginSource(c *ui.Context, m *plugin.Manifest, src *settings.PluginSource) {
	ui.Column(c).Gap(theme.Space8).AlignItems(ui.Stretch).Children(func() {
		widgets.Overline(c, "SOURCE")
		ui.Row(c).Padding(theme.Space8, theme.Space6, theme.Space8, theme.Space12).Gap(theme.Space10).Radius(theme.RadiusMD).Background(pal().Subtle).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, icons.GitBranch).FontSize(16).TextColor(pal().Icon)
			ui.Column(c).Grow(1).MinWidth(0).Gap(theme.Space2).Children(func() {
				ui.Text(c, "github.com/"+src.Repo).Font(theme.FontMono).FontSize(12).SingleLine().Selectable()
				what := "Tracking releases · on " + src.Ref
				if src.Branch != "" {
					what = "Tracking " + src.Branch + " · on " + plugin.ShortSHA(src.Ref)
				}
				if !a.settings.UpdatesChecked.IsZero() {
					what += " · checked " + ago(a.settings.UpdatesChecked)
				}
				ui.Text(c, what).FontSize(12).TextColor(pal().TextTertiary).SingleLine()
			})
			if a.updatesChecking {
				ui.Spinner(c).Size(14, 14).Label("Checking for updates")
			} else if widgets.ButtonSized(c, widgets.Ghost, nil, "Check now", theme.ControlS).Clicked() {
				a.checkUpdates(true)
			}
		})
		auto := src.Auto
		ui.Checkbox(c, &auto, "Update automatically when an update asks for nothing new")
		if auto != src.Auto {
			src.Auto = auto
			a.saveSettings()
		}
	})
}

// githubSheet is Install from GitHub.
func (a *App) githubSheet(c *ui.Context) {
	g := &a.ghi
	if !g.open {
		return
	}
	widgets.DialogFrame(c, &g.open, 520, func() {
		ui.Column(c).Grow(1).Gap(theme.Space4).Children(func() {
			ui.Text(c, "Install from GitHub").FontSize(16).FontWeight(600)
			ui.Text(c, "Install from a release or a branch. Dalibre can keep it up to date from the same place.").FontSize(13).TextColor(pal().TextSecondary)
		})
	}, func() {
		ui.Column(c).Padding(theme.Space16, theme.Space24, theme.Space20, theme.Space24).Gap(theme.Space12).AlignItems(ui.Stretch).Children(func() {
			labeled(c, "Repository", func() {
				ui.Row(c).Gap(theme.Space8).Children(func() {
					in := ui.TextInput(c, &g.text).Label("Repository").Placeholder("owner/repo").AutoFocus().Grow(1)
					if in.Submitted() {
						a.ghFind()
					}
					if widgets.Button(c, widgets.Secondary, nil, "Find").Disabled(strings.TrimSpace(g.text) == "" || g.finding).Clicked() {
						a.ghFind()
					}
				})
				ui.Text(c, "owner/repo or a github.com link.").FontSize(12).TextColor(pal().TextTertiary)
			})
			switch {
			case g.finding:
				ui.Row(c).Gap(theme.Space8).AlignItems(ui.Center).Children(func() {
					ui.Spinner(c).Size(14, 14).Label("Finding")
					ui.Text(c, "Looking at "+g.repo.String()+"…").FontSize(13).TextColor(pal().TextSecondary)
				})
			case g.err != "":
				ui.Row(c).Padding(theme.Space10, theme.Space12).Gap(theme.Space10).Radius(theme.RadiusMD).Background(pal().Danger.BG).AlignItems(ui.Start).Children(func() {
					ui.Icon(c, icons.Alert).FontSize(14).TextColor(pal().Danger.FG)
					ui.Text(c, g.err).FontSize(12).TextColor(pal().Danger.FG).Grow(1)
				})
			}
			if len(g.releases) > 0 || len(g.branches) > 0 {
				a.ghFound(c)
			}
		})
	}, func() {
		ui.Spacer(c)
		if widgets.Button(c, widgets.Secondary, nil, "Cancel").Clicked() {
			g.open = false
		}
		if widgets.Button(c, widgets.Primary, nil, "Install").Disabled(g.manifest == nil || g.preparing).Clicked() {
			a.ghInstallChosen()
		}
	})
	if !g.open {
		g.discard()
	}
}

// ghFound is what the repository holds: the version chosen, the plugin
// it is and what it may reach.
func (a *App) ghFound(c *ui.Context) {
	g := &a.ghi
	ui.Row(c).Padding(theme.Space12).Gap(theme.Space12).Radius(theme.RadiusMD).Background(pal().Subtle).AlignItems(ui.Center).Children(func() {
		switch {
		case g.preparing:
			ui.Spinner(c).Size(16, 16).Label("Downloading")
			ui.Text(c, "Downloading…").FontSize(13).TextColor(pal().TextSecondary).Grow(1)
		case g.manifest != nil:
			pluginTile(c, g.manifest, 32)
			ui.Column(c).Grow(1).MinWidth(0).Gap(theme.Space2).Children(func() {
				ui.Text(c, g.manifest.Name+" "+g.manifest.Version).FontSize(13).FontWeight(500)
				by := g.detail
				if g.manifest.Author != "" {
					by = "by " + g.manifest.Author + " · " + by
				}
				ui.Text(c, by).FontSize(12).TextColor(pal().TextSecondary).MaxLines(2)
			})
		default:
			ui.Spacer(c)
		}
		label := "Latest release"
		if latest, ok := plugin.Latest(g.releases); !ok || g.tag != latest.Tag {
			label = g.tag
		}
		if g.branch != "" {
			label = "Branch: " + g.branch
		}
		b := ui.ButtonBase(c).Width(170).Height(theme.ControlM).Padding(0, theme.Space8, 0, theme.Space10).Gap(theme.Space6).AlignItems(ui.Center).
			Radius(theme.RadiusSM).Background(pal().Surface).Border(1, pal().Border).Label("Version: " + label)
		b.PopoverMenuButton(func(m *ui.Menu) {
			m.MaxWidth(300)
			latest, _ := plugin.Latest(g.releases)
			if len(g.releases) > 0 {
				for i, r := range g.releases {
					if i >= 8 {
						break
					}
					title := r.Tag
					if r.Tag == latest.Tag {
						title = "Latest release"
					}
					sub := r.Tag + " · " + ago(r.Published)
					if r.Prerelease {
						sub += " · pre-release"
					}
					if m.Item(title).ID("tag:" + r.Tag).Subtitle(sub).Icon(icons.Menu(icons.Tag)).Checked(g.branch == "" && g.tag == r.Tag).Chosen() {
						a.ghChoose(r.Tag, "")
					}
				}
			}
			if len(g.branches) > 0 {
				if len(g.releases) > 0 {
					m.Separator()
				}
				for i, br := range g.branches {
					if i >= 8 {
						break
					}
					if m.Item(br.Name).ID("branch:" + br.Name).Trailing(plugin.ShortSHA(br.Commit)).Icon(icons.Menu(icons.GitBranch)).Checked(g.branch == br.Name).Chosen() {
						a.ghChoose("", br.Name)
					}
				}
			}
		})
		b.Children(func() {
			ui.Text(c, label).FontSize(12).FontWeight(500).SingleLine().Grow(1)
			ui.Icon(c, icons.ChevronDown).FontSize(12).TextColor(pal().IconMuted)
		})
	})
	if g.branch != "" {
		ui.Row(c).Padding(theme.Space10, theme.Space12).Gap(theme.Space10).Radius(theme.RadiusMD).Background(pal().Warning.BG).AlignItems(ui.Start).Children(func() {
			ui.Icon(c, icons.Alert).FontSize(14).TextColor(pal().Warning.FG)
			ui.Text(c, "A branch can change at any time and isn’t a published release. Use it to try a fix early or to run a plugin you’re writing.").
				FontSize(12).TextColor(pal().Warning.FG).Grow(1)
		})
	}
	if m := g.manifest; m != nil {
		ui.Column(c).Gap(theme.Space6).AlignItems(ui.Stretch).Children(func() {
			widgets.Overline(c, "IT WILL BE ABLE TO")
			for _, h := range m.Permissions.Hosts {
				permissionItem(c.Key(h), icons.Globe, "Connect to "+h, "", false)
			}
			if m.Permissions.LocalNetwork {
				permissionItem(c, icons.Network, "Connect to this computer and your local network", "Only allow this for a plugin meant for a server you run.", true)
			}
			if len(m.Permissions.Hosts) == 0 && !m.Permissions.LocalNetwork {
				ui.Text(c, "Nothing: it can’t reach the network.").FontSize(12).TextColor(pal().TextTertiary)
			}
		})
		keepLabel := "Keep it up to date from this repository"
		if g.branch != "" {
			keepLabel = "Follow new commits on " + g.branch
		}
		ui.Checkbox(c, &g.keep, keepLabel)
		ui.Text(c, "Plugins from GitHub run in the same sandbox, with only the access above. An update that asks for more waits for you.").
			FontSize(12).TextColor(pal().TextTertiary)
	}
}
