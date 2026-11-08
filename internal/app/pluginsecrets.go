package app

import (
	"errors"
	"maps"
	"slices"

	"dalibre/cmd/dalibre/locales"
	"dalibre/internal/plugin"
)

// A plugin's secret settings (type "secret" in its plugin.json: a
// password, an API key) are kept in the system's credential store, not
// in settings.json with its other values.

// pluginSecret is the store's key for setting key of plugin id.
func pluginSecret(id, key string) string { return plugin.SecretKey(id, key) }

// secretKeys are the keys of m's secret settings.
func secretKeys(m *plugin.Manifest) []string {
	var keys []string
	for _, s := range m.Settings {
		if s.Type == "secret" {
			keys = append(keys, s.Key)
		}
	}
	return keys
}

// pluginValues are what plugin m loads with: saved, its values in
// settings.json, with its secrets from store. A secret still in saved,
// from before secrets were kept in the store, is moved there; moved says
// so, for the caller to drop it from settings.json. With no store, saved
// is used as it is.
func pluginValues(store secretStore, m *plugin.Manifest, saved map[string]string) (values, secrets map[string]string, moved bool) {
	keys := secretKeys(m)
	if store == nil || len(keys) == 0 {
		return saved, nil, false
	}
	values, secrets = map[string]string{}, map[string]string{}
	for k, v := range saved {
		if !slices.Contains(keys, k) {
			values[k] = v
		}
	}
	for _, k := range keys {
		if v := saved[k]; v != "" {
			if store.Set(pluginSecret(m.ID, k), v) == nil {
				moved = true
			}
			secrets[k] = v
		} else if v, err := store.Get(pluginSecret(m.ID, k)); err == nil && v != "" {
			secrets[k] = v
		}
	}
	maps.Copy(values, secrets)
	return values, secrets, moved
}

// dropMovedSecrets removes the secrets pluginValues moved to the store
// from settings.json.
func (a *App) dropMovedSecrets(moved []*plugin.Manifest) {
	if len(moved) == 0 {
		return
	}
	for _, m := range moved {
		if st := a.settings.Plugins[m.ID]; st != nil {
			for _, k := range secretKeys(m) {
				delete(st.Values, k)
			}
		}
	}
	a.saveSettings()
}

// savePluginValues saves values as plugin m's settings: its secrets in
// the store, the others in settings.json. done runs on the main thread
// once they are saved.
func (a *App) savePluginValues(m *plugin.Manifest, values map[string]string, done func()) {
	store := a.secrets
	keys := secretKeys(m)
	if store == nil || len(keys) == 0 {
		a.settings.Plugin(m.ID).Values = values
		a.saveSettings()
		done()
		return
	}
	plain, secrets := map[string]string{}, map[string]string{}
	for k, v := range values {
		if slices.Contains(keys, k) {
			secrets[k] = v
		} else {
			plain[k] = v
		}
	}
	a.run(func() {
		var err error
		for _, k := range keys {
			if v := secrets[k]; v != "" {
				err = store.Set(pluginSecret(m.ID, k), v)
			} else {
				store.Delete(pluginSecret(m.ID, k))
			}
			if err != nil {
				break
			}
		}
		a.post(func() {
			if err != nil {
				a.toastError(errors.New(locales.PluginsKeychainFailed(m.Name, err.Error())))
				return
			}
			a.settings.Plugin(m.ID).Values = plain
			if a.pluginSecrets == nil {
				a.pluginSecrets = map[string]map[string]string{}
			}
			a.pluginSecrets[m.ID] = secrets
			a.saveSettings()
			done()
		})
	})
}

// forgetPluginSecrets removes plugin m's secrets from the store.
func (a *App) forgetPluginSecrets(m *plugin.Manifest) {
	store := a.secrets
	delete(a.pluginSecrets, m.ID)
	if store == nil {
		return
	}
	keys := secretKeys(m)
	a.run(func() {
		for _, k := range keys {
			store.Delete(pluginSecret(m.ID, k))
		}
	})
}
