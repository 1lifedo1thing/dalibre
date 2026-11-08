package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"dalibre/internal/calibre"
	"dalibre/internal/catalog"
	"dalibre/internal/plugin"
	"dalibre/internal/settings"
)

// download is a book coming from a source plugin into the library.
type download struct {
	plugin      *plugin.Plugin
	result      plugin.Result
	format      string
	done, total int64 // bytes; total is -1 when unknown
	err         error
	finished    bool
	cancel      context.CancelFunc
}

// consentAsk is a plugin waiting for the user to grant its permissions:
// one being installed (archive set) or an installed one whose update asks
// for more.
type consentAsk struct {
	manifest *plugin.Manifest
	archive  string
}

// granted reports whether the user granted m everything it asks for.
func granted(st *settings.PluginState, m *plugin.Manifest) bool {
	if st == nil || st.Granted == nil {
		return false
	}
	return plugin.Permissions{Hosts: st.Granted.Hosts, LocalNetwork: st.Granted.LocalNetwork}.Covers(m.Permissions)
}

// loadPlugins loads the enabled plugins in the plugins folder whose
// permissions the user granted, in the background. The others wait for
// the user in pluginsPending.
func (a *App) loadPlugins() {
	dir := a.pluginDir
	if dir == "" {
		return
	}
	states := map[string]settings.PluginState{}
	for id, st := range a.settings.Plugins {
		states[id] = *st
	}
	a.run(func() {
		var pending []*plugin.Manifest
		plugins, errs := plugin.Discover(dir, func(m *plugin.Manifest) (plugin.Options, bool) {
			st := states[m.ID]
			if st.Disabled {
				return plugin.Options{}, false
			}
			if !granted(&st, m) {
				pending = append(pending, m)
				return plugin.Options{}, false
			}
			return plugin.Options{Settings: st.Values}, true
		})
		a.post(func() {
			old := a.plugins
			for _, d := range a.downloads {
				if slices.Contains(old, d.plugin) && !d.finished {
					d.cancel()
				}
			}
			// Each plugin holds an interpreter whose memory Go doesn't
			// collect. Close waits for a call in progress.
			a.run(func() {
				for _, p := range old {
					p.Close()
				}
			})
			a.plugins, a.pluginErrs, a.pluginsPending = plugins, errs, pending
			for _, err := range errs {
				a.toastError(fmt.Errorf("Plugin %w", err))
			}
		})
	})
}

// sources are the loaded plugins that search for books.
func (a *App) sources() []*plugin.Plugin {
	var out []*plugin.Plugin
	for _, p := range a.plugins {
		if p.Manifest.Has(plugin.CapSource) {
			out = append(out, p)
		}
	}
	return out
}

// installPlugin installs a plugin package. When it asks for more than
// the user granted it, the user is asked first (a.consent).
func (a *App) installPlugin(archive string) {
	a.run(func() {
		m, err := plugin.Inspect(archive)
		a.post(func() {
			if err != nil {
				a.toastError(fmt.Errorf("Couldn’t install the plugin: %w", err))
				return
			}
			if !granted(a.settings.Plugins[m.ID], m) {
				a.consent = &consentAsk{manifest: m, archive: archive}
				return
			}
			a.finishInstall(archive)
		})
	})
}

// grantConsent grants the plugin the user is asked about its
// permissions, and installs or loads it.
func (a *App) grantConsent() {
	c := a.consent
	if c == nil {
		return
	}
	a.consent = nil
	st := a.settings.Plugin(c.manifest.ID)
	st.Granted = &settings.Grant{Hosts: slices.Clone(c.manifest.Permissions.Hosts), LocalNetwork: c.manifest.Permissions.LocalNetwork}
	a.saveSettings()
	if c.archive != "" {
		a.finishInstall(c.archive)
		return
	}
	a.loadPlugins()
}

// askConsent asks the user to grant an installed plugin waiting in
// pluginsPending.
func (a *App) askConsent(id string) {
	if i := slices.IndexFunc(a.pluginsPending, func(m *plugin.Manifest) bool { return m.ID == id }); i >= 0 {
		a.consent = &consentAsk{manifest: a.pluginsPending[i]}
	}
}

func (a *App) finishInstall(archive string) {
	dir := a.pluginDir
	a.run(func() {
		m, err := plugin.Install(archive, dir)
		a.post(func() {
			if err != nil {
				a.toastError(fmt.Errorf("Couldn’t install the plugin: %w", err))
				return
			}
			a.toast("Installed %s %s", m.Name, m.Version)
			a.loadPlugins()
		})
	})
}

// setPluginEnabled turns plugin id on or off.
func (a *App) setPluginEnabled(id string, on bool) {
	a.settings.Plugin(id).Disabled = !on
	a.saveSettings()
	a.loadPlugins()
}

// setPluginValues saves a plugin's settings and reloads it with them.
func (a *App) setPluginValues(id string, values map[string]string) {
	a.settings.Plugin(id).Values = values
	a.saveSettings()
	a.loadPlugins()
}

// uninstallPlugin removes plugin id and what the user decided about it.
func (a *App) uninstallPlugin(id string) {
	dir := a.pluginDir
	a.run(func() {
		err := plugin.Uninstall(dir, id)
		a.post(func() {
			if err != nil {
				a.toastError(err)
				return
			}
			delete(a.settings.Plugins, id)
			a.saveSettings()
			a.loadPlugins()
		})
	})
}

// downloadBook downloads r as format with its plugin and adds it to the
// library, with the metadata the plugin gave.
func (a *App) downloadBook(p *plugin.Plugin, r plugin.Result, format string) {
	lib := a.lib
	if lib == nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	d := &download{plugin: p, result: r, format: strings.ToUpper(format), total: -1, cancel: cancel}
	a.downloads = append(a.downloads, d)
	a.run(func() {
		defer cancel()
		id, err := func() (int64, error) {
			dir, err := os.MkdirTemp("", "dalibre-download-")
			if err != nil {
				return 0, err
			}
			defer os.RemoveAll(dir)
			var last time.Time
			file, err := p.Download(ctx, r, format, dir, func(done, total int64) {
				if now := time.Now(); now.Sub(last) > 100*time.Millisecond || done == total {
					last = now
					a.post(func() { d.done, d.total = done, total })
				}
			})
			if err != nil {
				return 0, err
			}
			cover, _ := p.Cover(ctx, r) // the file's own cover comes first; a missing one is no failure
			return lib.ImportWith(file, resultMetadata(r), cover)
		}()
		a.post(func() {
			d.finished, d.err = true, err
			if err != nil {
				a.toastError(errors.New(pluginErrorText(r.Title, err)))
				return
			}
			a.reload(func() {
				a.query = catalog.Query{Sort: catalog.SortDateAdded, Descending: true}
				a.chosen.Clear()
				a.curID = id
				a.toast("Added %s", r.Title)
			})
		})
	})
}

// pluginErrorText says what went wrong with a download, and what to do.
func pluginErrorText(title string, err error) string {
	var pe *plugin.Error
	if !errors.As(err, &pe) {
		return fmt.Sprintf("Couldn’t add %s: %v", title, err)
	}
	switch pe.Kind {
	case plugin.KindAuth, plugin.KindSettings:
		return fmt.Sprintf("%s needs you to sign in or check its settings: %s", pe.Plugin, pe.Msg)
	case plugin.KindRateLimited:
		return fmt.Sprintf("%s is busy; try again in a while", pe.Plugin)
	case plugin.KindUnavailable:
		return fmt.Sprintf("%s isn’t answering; try again later (%s)", pe.Plugin, pe.Msg)
	case plugin.KindNotFound:
		return fmt.Sprintf("%s is no longer available from %s", title, pe.Plugin)
	case plugin.KindTimeout:
		return fmt.Sprintf("Downloading %s %s", title, pe.Msg)
	}
	return fmt.Sprintf("Couldn’t download %s: the %s plugin failed (%s)", title, pe.Plugin, pe.Msg)
}

// resultMetadata is the library metadata for a plugin's result.
func resultMetadata(r plugin.Result) calibre.Metadata {
	m := calibre.Metadata{
		Title: r.Title, Authors: r.Authors, Publisher: r.Publisher,
		Series: r.Series, SeriesIndex: r.SeriesIndex, Tags: r.Tags, Comments: r.Description,
		Pubdate: plugin.ParseDate(r.Pubdate), Identifiers: map[string]string{},
	}
	if code := calibre.LanguageCode(r.Language); code != "" {
		m.Languages = []string{code}
	}
	for k, v := range r.Identifiers {
		if k = strings.ToLower(strings.TrimSpace(k)); k != "" && v != "" {
			m.Identifiers[k] = v
		}
	}
	return m
}
