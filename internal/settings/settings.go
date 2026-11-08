// Package settings keeps the user's choices between runs, as JSON in the
// user's configuration folder.
package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
)

// Settings are the choices Dalibre remembers.
type Settings struct {
	Library    string   `json:"library"`
	Recent     []string `json:"recent"`
	ListView   bool     `json:"listView"`
	HideDetail bool     `json:"hideDetail"`
	Sort       int      `json:"sort"`
	Descending bool     `json:"descending"`
	// Plugins are what the user decided about each plugin, by id.
	Plugins map[string]*PluginState `json:"plugins,omitempty"`

	path string
}

// PluginState is what the user decided about a plugin.
type PluginState struct {
	Disabled bool `json:"disabled,omitempty"`
	// Granted are the permissions the user granted; nil until they do.
	// A plugin asking for more than was granted isn't loaded until the
	// user grants it again.
	Granted *Grant            `json:"granted,omitempty"`
	Values  map[string]string `json:"values,omitempty"` // the plugin's settings
}

// Grant is the permissions granted to a plugin, as its manifest has them.
type Grant struct {
	Hosts        []string `json:"hosts"`
	LocalNetwork bool     `json:"localNetwork,omitempty"`
}

// Plugin returns the state of plugin id, creating it.
func (s *Settings) Plugin(id string) *PluginState {
	if s.Plugins == nil {
		s.Plugins = map[string]*PluginState{}
	}
	if s.Plugins[id] == nil {
		s.Plugins[id] = &PluginState{}
	}
	return s.Plugins[id]
}

// MaxRecent is how many libraries Recent keeps.
const MaxRecent = 6

// DefaultPath is settings.json in the user's configuration folder.
func DefaultPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "Dalibre", "settings.json")
}

// Load reads the settings at path; missing or unreadable settings give
// the defaults. An empty path keeps the settings in memory only.
func Load(path string) *Settings {
	s := &Settings{Descending: true, path: path}
	if path != "" {
		if data, err := os.ReadFile(path); err == nil {
			_ = json.Unmarshal(data, s)
		}
	}
	return s
}

// Save writes the settings where they were loaded from.
func (s *Settings) Save() error {
	if s.path == "" {
		return nil
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// Opened records root as the library in use, first among the recent ones.
func (s *Settings) Opened(root string) {
	s.Library = root
	s.Recent = append([]string{root}, slices.DeleteFunc(s.Recent, func(r string) bool { return r == root })...)
	if len(s.Recent) > MaxRecent {
		s.Recent = s.Recent[:MaxRecent]
	}
}
