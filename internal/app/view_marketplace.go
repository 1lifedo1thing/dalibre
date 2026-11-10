package app

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"dalibre/cmd/dalibre/locales"
	"dalibre/internal/app/icons"
	"dalibre/internal/app/theme"
	"dalibre/internal/app/widgets"
	"dalibre/internal/plugin"
)

// The Marketplace tab of Settings › Plugins (Figma "18 · Plugin
// marketplace"): search, categories and the plugins listed, and the one
// chosen, with what it adds and may reach.

// pluginsTabs are Settings › Plugins' tabs, atop its list.
func (a *App) pluginsTabs(c *ui.Context) {
	tab := a.plTab
	ui.Tabs(c, &tab, locales.PluginsTabInstalled(), locales.PluginsTabMarketplace())
	if tab != a.plTab {
		a.plTab = tab
		if tab == pluginsTabMarketplace {
			a.loadMarketplace(false)
		}
	}
}

// marketplacePane is the Marketplace tab: its list, and the plugin chosen.
func (a *App) marketplacePane(c *ui.Context) {
	mk := &a.mk
	if mk.index == nil && !mk.loading && mk.err == "" {
		a.loadMarketplace(false)
	}
	list := a.marketListings()
	if (mk.sel == "" || mk.index.Find(mk.sel) == nil) && len(list) > 0 {
		mk.sel = list[0].ID
	}
	ui.Column(c).Width(300).Padding(theme.Space8).Gap(theme.Space8).Background(pal().Chrome).BorderWidth(0, 1, 0, 0).BorderColor(pal().BorderSubtle).AlignItems(ui.Stretch).Children(func() {
		a.pluginsTabs(c)
		ui.SearchField(c, &mk.query).Placeholder(locales.MarketSearch()).Label(locales.MarketSearch())
		if mk.index != nil && len(mk.index.Categories) > 0 {
			ui.Row(c).Wrap().Gap(theme.Space6).Children(func() {
				if widgets.Tag(c.Key("all"), locales.MarketAll(), mk.cat == "", nil).Clicked() {
					mk.cat = ""
				}
				for _, cat := range mk.index.Categories {
					if widgets.Tag(c.Key(cat.ID), cat.Title, mk.cat == cat.ID, nil).Clicked() {
						mk.cat = cat.ID
					}
				}
			})
		}
		ui.Scroll(c).Grow(1).MinHeight(0).Gap(theme.Space2).AlignItems(ui.Stretch).Children(func() {
			switch {
			case mk.index == nil && mk.loading:
				ui.Row(c).Padding(theme.Space12, theme.Space10).Gap(theme.Space8).AlignItems(ui.Center).Children(func() {
					ui.Spinner(c).Size(14, 14)
					ui.Text(c, locales.MarketLoading()).FontSize(12).TextColor(pal().TextTertiary)
				})
			case mk.index == nil && mk.err != "":
				a.marketError(c, locales.MarketFailed(mk.err))
			case len(list) == 0:
				ui.Text(c, locales.MarketNoMatch()).FontSize(12).TextColor(pal().TextTertiary).Padding(theme.Space12, theme.Space10)
			}
			for _, l := range list {
				a.marketRow(c.Key(l.ID), l)
			}
		})
		if mk.index != nil && mk.err != "" {
			a.marketError(c, locales.MarketStale())
		}
		ui.Text(c, locales.MarketFrom(plugin.MarketplaceRepo)).FontSize(11).TextColor(pal().TextTertiary).SingleLine().Padding(0, theme.Space4)
	})
	ui.Scroll(c).Grow(1).MinWidth(0).Padding(theme.Space20).Gap(theme.Space16).AlignItems(ui.Stretch).Children(func() {
		if l := mk.index.Find(mk.sel); l != nil {
			a.listingPane(c, l)
		}
	})
}

// marketError says why the index couldn't be read, with Try again.
func (a *App) marketError(c *ui.Context, msg string) {
	ui.Column(c).Padding(theme.Space8, theme.Space10).Gap(theme.Space6).AlignItems(ui.Start).Children(func() {
		ui.Text(c, msg).FontSize(12).TextColor(pal().TextSecondary).MaxLines(4)
		if widgets.ButtonSized(c, widgets.Secondary, nil, locales.MarketTryAgain(), theme.ControlS).Disabled(a.mk.loading).Clicked() {
			a.loadMarketplace(true)
		}
	})
}

// marketRow is the MarketplaceRow component.
func (a *App) marketRow(c *ui.Context, l *plugin.Listing) {
	sel := a.mk.sel == l.ID
	state := a.listingState(l)
	row := ui.ButtonBase(c).Padding(theme.Space8, theme.Space10).Gap(theme.Space10).Radius(theme.RadiusMD).AlignItems(ui.Center).Label(l.Name)
	switch {
	case sel:
		row.Background(pal().SurfaceRaised).Shadow(0, 1, 2, 0, ui.RGBA(26, 23, 18, 0.06)).Shadow(0, 0, 0, 1, pal().BorderSubtle.Alpha(0.6))
	case row.Hovered():
		row.Background(pal().Hover)
	}
	row.OnClick(func() { a.mk.sel = l.ID }).Children(func() {
		pluginTile(c, l.Manifest(), 28)
		ui.Column(c).Grow(1).MinWidth(0).Children(func() {
			ui.Text(c, l.Name).FontSize(13).FontWeight(500).SingleLine()
			detail, tone := l.Description, pal().TextTertiary
			if state == listingUpdate {
				detail, tone = locales.MarketUpdateTo(l.Version), pal().TextAccent
			}
			ui.Text(c, detail).FontSize(12).TextColor(tone).SingleLine()
		})
		a.listingButton(c, l, state, false)
	})
}

// listingButton is what a listing offers: Install, Update, or that it is
// installed. primary is the detail pane's larger button.
func (a *App) listingButton(c *ui.Context, l *plugin.Listing, state listingState, primary bool) {
	variant, height := widgets.Secondary, float32(theme.ControlS)
	if primary {
		variant, height = widgets.Primary, theme.ControlM
	}
	switch {
	case a.mk.installing == l.ID || state == listingUpdate && a.updates[l.ID] != nil && a.updates[l.ID].busy:
		ui.Spinner(c).Size(16, 16).Label(locales.MarketInstallingLabel(l.Name))
	case state == listingNeedsDalibre:
		ui.Text(c, locales.MarketNeedsDalibre(l.Manifest().NeedsDalibre())).FontSize(12).TextColor(pal().TextTertiary).NoWrap()
	case state == listingInstalled:
		ui.Row(c).Gap(theme.Space4).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, icons.CheckCircle).FontSize(14).TextColor(pal().Success.FG)
			ui.Text(c, locales.MarketInstalled()).FontSize(12).FontWeight(500).TextColor(pal().Success.FG).NoWrap()
		})
	case state == listingUpdate:
		var icon *ui.SVG
		if primary {
			icon = icons.Refresh
		}
		if widgets.ButtonSized(c, variant, icon, locales.CommonUpdate(), height).Clicked() {
			a.updatePlugin(l.ID, false)
		}
	default:
		var icon *ui.SVG
		if primary {
			icon = icons.Download
		}
		if widgets.ButtonSized(c, variant, icon, locales.MarketInstall(), height).Disabled(a.mk.installing != "" || a.pluginDir == "").Clicked() {
			a.installListing(l)
		}
	}
}

// listingPane is the plugin chosen in the Marketplace.
func (a *App) listingPane(c *ui.Context, l *plugin.Listing) {
	m := l.Manifest()
	state := a.listingState(l)
	ui.Row(c).Gap(theme.Space12).AlignItems(ui.Center).Children(func() {
		pluginTile(c, m, 40)
		ui.Column(c).Grow(1).MinWidth(0).Gap(theme.Space2).Children(func() {
			ui.Text(c, l.Name).FontSize(16).FontWeight(600)
			var parts []string
			if state == listingUpdate {
				parts = append(parts, locales.MarketVersionInstalled(a.installedVersion(l.ID, nil), l.Version))
			} else {
				parts = append(parts, locales.PluginsVersion(l.Version))
				if l.Author != "" {
					parts = append(parts, locales.PluginsByAuthor(l.Author))
				}
				if l.License != "" {
					parts = append(parts, l.License)
				}
			}
			ui.Text(c, strings.Join(parts, " · ")).FontSize(12).TextColor(pal().TextTertiary)
		})
		a.listingButton(c, l, state, true)
	})
	if l.Description != "" {
		ui.Text(c, l.Description).FontSize(13).TextColor(pal().TextSecondary)
	}
	pluginAddsTo(c, m, false)
	if l.Calibre != "" {
		ui.Column(c).Gap(theme.Space6).AlignItems(ui.Stretch).Children(func() {
			widgets.Overline(c, locales.MarketFollows())
			ui.Text(c, locales.MarketFollowsCalibre(l.Calibre)).FontSize(13)
			note := locales.MarketFollowsNote()
			if l.Downloads > 0 {
				note = locales.MarketCalibreDownloads(l.Downloads) + " " + note
			}
			ui.Text(c, note).FontSize(12).TextColor(pal().TextTertiary)
		})
	}
	ui.Column(c).Gap(theme.Space8).AlignItems(ui.Stretch).Children(func() {
		widgets.Overline(c, locales.PluginsCanAccess())
		for _, h := range m.Permissions.Hosts {
			permissionItem(c.Key(h), icons.Globe, h, "", false)
		}
		if len(m.Permissions.Hosts) == 0 && !m.Permissions.LocalNetwork {
			permissionItem(c, icons.ShieldCheck, locales.MarketNoInternet(), "", false)
		}
		if m.Permissions.LocalNetwork {
			permissionItem(c, icons.Network, locales.PluginsLocalNetwork(), locales.PluginsLocalNetworkDetail(), true)
		}
		if m.IsFileType() {
			permissionItem(c, icons.File, locales.PluginsBookFilesAccess(), locales.PluginsBookFilesAccessDetail(), false)
		}
		libraryPermission(c, m, false, nil)
		note := locales.MarketNoteAllow()
		if state == listingNeedsDalibre {
			note = locales.MarketNeedsDalibreNote(l.Manifest().NeedsDalibre())
		} else if state != listingAvailable {
			note = locales.PluginsNoteAllowed()
		}
		ui.Text(c, note).FontSize(12).TextColor(pal().TextTertiary)
	})
	ui.Row(c).Gap(theme.Space8).AlignItems(ui.Center).Children(func() {
		// Its folder for one of the Marketplace's own, else its repository.
		src := l.Homepage
		if l.Community || !strings.HasPrefix(src, "https://github.com/") {
			src = "https://github.com/" + l.Repo
		}
		if l.Repo != "" {
			if widgets.Button(c, widgets.Ghost, icons.External, locales.PluginsViewOnGithub()).Tooltip(strings.TrimPrefix(src, "https://")).Clicked() {
				openURL(src)
			}
		}
		ui.Spacer(c)
		if l.Community {
			widgets.Tag(c, locales.MarketCommunity(), false, nil).Tooltip(locales.MarketCommunityNote(l.Repo))
		}
		if state == listingInstalled || state == listingUpdate {
			if widgets.Button(c, widgets.Secondary, nil, locales.MarketShowInstalled()).Clicked() {
				a.plTab, a.pl.sel = pluginsTabInstalled, l.ID
			}
		}
	})
}

// pluginAddsTo is ADDS TO DALIBRE: where the plugin shows up, and, for
// one with actions, their titles. installed tells the notes of an
// installed plugin's pane, which say more.
func pluginAddsTo(c *ui.Context, m *plugin.Manifest, installed bool) {
	acts := m.Capabilities.Actions
	var tools, menu []string
	for _, act := range acts {
		if act.Scope == plugin.ScopeLibrary {
			tools = append(tools, act.Title)
		} else {
			menu = append(menu, act.Title)
		}
	}
	if !m.IsBookSource() && !m.IsMetadataSource() && !m.IsFileType() && len(acts) == 0 {
		return
	}
	ui.Column(c).Gap(theme.Space8).AlignItems(ui.Stretch).Children(func() {
		widgets.Overline(c, locales.PluginsAddsToTitle())
		ui.Row(c).Wrap().Gap(theme.Space6).Children(func() {
			if m.IsBookSource() {
				widgets.Tag(c, locales.CommonGetBooks(), false, nil)
			}
			if m.IsMetadataSource() {
				widgets.Tag(c, locales.CommonFetchMetadata(), false, nil)
			}
			if m.IsFileType() {
				widgets.Tag(c, locales.PluginsCapBookFiles(), false, nil)
			}
			if len(tools) > 0 {
				widgets.Tag(c, locales.MainNavTools(), false, nil)
			}
			if len(menu) > 0 {
				widgets.Tag(c, locales.PluginsCapBookMenu(), false, nil)
			}
		})
		if installed {
			return
		}
		if len(tools) > 0 {
			ui.Text(c, locales.PluginsToolsNote(strings.Join(tools, ", "))).FontSize(12).TextColor(pal().TextTertiary)
		}
		if len(menu) > 0 {
			ui.Text(c, locales.PluginsBookMenuNote(strings.Join(menu, ", "))).FontSize(12).TextColor(pal().TextTertiary)
		}
		if m.IsBookSource() {
			ui.Text(c, locales.PluginsGetBooksNote(m.Name)).FontSize(12).TextColor(pal().TextTertiary)
		}
		if m.IsMetadataSource() {
			ui.Text(c, locales.PluginsFetchMetadataNote(m.Name)).FontSize(12).TextColor(pal().TextTertiary)
		}
	})
}
