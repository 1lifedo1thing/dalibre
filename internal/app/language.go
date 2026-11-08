package app

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"

	"dalibre/cmd/dalibre/locales"
	"dalibre/internal/app/icons"
	"dalibre/internal/app/theme"
)

// The language Dalibre shows: the system's, or one the user chose in
// Settings › General or on the welcome screen. MyGo keeps the choice for
// the next launches (App.SetLocale); settings.Language remembers it too,
// so that the picker can tell "System" from the language it resolves to.

// languages returns the languages Dalibre is written in, as tags: its
// catalogs'.
func languages() []string { return i18n.Languages() }

// autonym is a language's name in itself: "English", "简体中文".
func autonym(tag string) string { return i18n.Autonym(tag) }

// systemLanguageLabel is the picker's "System" item, with the language it
// shows now: "System (English)".
func (a *App) systemLanguageLabel() string {
	// Following the system, the system's language is the one shown. Only
	// ask MyGo otherwise: it waits for the app's main thread to say.
	cur := i18n.Locale()
	if a.settings.Language != "" {
		cur = bestLanguage(mygo.App.PreferredLanguages()...)
	}
	return locales.LanguageSystem(autonym(cur))
}

// bestLanguage returns the language of the catalogs that suits a reader
// of tags best, as the app would show it: zh-Hant for zh-TW.
func bestLanguage(tags ...string) string {
	if l := i18n.Match(tags, languages()); l != "" {
		return l
	}
	return "en"
}

// setLanguage shows tag, or the system's languages again for "".
func (a *App) setLanguage(tag string) {
	if tag == a.settings.Language {
		return
	}
	a.settings.Language = tag
	a.saveSettings()
	mygo.App.SetLocale(tag)
}

// languageSelect is Settings › General's picker.
func (a *App) languageSelect(c *ui.Context) {
	tags := append([]string{""}, languages()...)
	names := make([]string, len(tags))
	cur := names[0]
	for i, t := range tags {
		if t == "" {
			names[i] = a.systemLanguageLabel()
		} else {
			names[i] = autonym(t)
		}
		if t == a.settings.Language {
			cur = names[i]
		}
	}
	if ui.Select(c, &cur, names).Width(200).Label(locales.CommonLanguage()).Changed() {
		for i, n := range names {
			if n == cur {
				a.setLanguage(tags[i])
			}
		}
	}
}

// welcomeLanguage is the welcome screen's picker (Figma: 09 · Onboarding,
// Welcome v2 — First launch, language): the language shown, opening a
// menu of the others.
func (a *App) welcomeLanguage(c *ui.Context) {
	b := ui.ButtonBase(c.Key("welcome:language")).Height(28).Padding(0, theme.Space8, 0, theme.Space10).Gap(theme.Space6).
		Radius(8).AlignItems(ui.Center).Label(locales.CommonLanguage())
	if b.Hovered() {
		b.Background(onDark(0.14)).Border(1, onDark(0.2))
	} else {
		b.Background(onDark(0.08)).Border(1, onDark(0.14))
	}
	b.PopoverMenuButton(func(m *ui.Menu) {
		if m.Item(a.systemLanguageLabel()).ID("lang:").Checked(a.settings.Language == "").Chosen() {
			a.setLanguage("")
		}
		m.Separator()
		for _, t := range languages() {
			if m.Item(autonym(t)).ID("lang:" + t).Checked(a.settings.Language == t).Chosen() {
				a.setLanguage(t)
			}
		}
	})
	b.Children(func() {
		ui.Icon(c, icons.Globe).FontSize(14).TextColor(onDark(0.62))
		ui.Text(c, autonym(i18n.Locale())).FontSize(12).FontWeight(500).TextColor(onDark(0.72))
		ui.Icon(c, icons.ChevronDown).FontSize(12).TextColor(onDark(0.62))
	})
}
