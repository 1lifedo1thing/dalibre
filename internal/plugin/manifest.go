// Package plugin runs Dalibre's plugins, which anyone may write: folders
// holding a plugin.json manifest and a JavaScript file, run in an
// embedded interpreter (no cgo, nothing for the user to install).
//
// Plugins are untrusted. A script has no file, process or network access
// of its own: it reaches the network only through the host's fetch, only
// to the hosts its manifest lists and the user granted, and never to the
// user's own machine or network unless granted that too. Every call has a
// deadline and a request budget, and what a plugin returns is checked
// field by field before the host uses it.
//
// API version 1 has one capability, "source": a plugin that searches a
// catalogue and says where a book's file is; the host downloads it.
package plugin

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// APIVersion is the plugin API this Dalibre implements. Within a version
// the API only grows: new optional fields and functions, never a changed
// or removed one. A manifest asking for a newer version is refused.
const APIVersion = 1

// Capabilities a plugin can declare.
const (
	CapSource = "source"
)

// Limits on what a plugin may be.
const (
	MaxScriptSize = 4 << 20
	MaxSettings   = 32
)

// Manifest is a plugin's plugin.json.
type Manifest struct {
	Schema       string      `json:"$schema,omitempty"`
	ID           string      `json:"id"`
	Name         string      `json:"name"`
	Version      string      `json:"version"` // semver, as "1.2.0"
	API          int         `json:"api"`
	Description  string      `json:"description,omitempty"`
	Author       string      `json:"author,omitempty"`
	Homepage     string      `json:"homepage,omitempty"`
	License      string      `json:"license,omitempty"`
	Main         string      `json:"main,omitempty"` // main.js when empty
	Capabilities []string    `json:"capabilities"`
	Permissions  Permissions `json:"permissions"`
	Settings     []Setting   `json:"settings,omitempty"`
}

// Permissions are what a plugin may reach. The user grants them when the
// plugin is installed, and again when an update asks for more.
type Permissions struct {
	// Hosts the plugin may fetch and download from, redirects included,
	// as "example.com" or "*.example.com" (any subdomain, not the domain).
	Hosts []string `json:"hosts"`
	// LocalNetwork lets the plugin reach this computer and the local
	// network (loopback, private and link-local addresses), as an OPDS
	// server at home. Without it those addresses are refused, whatever
	// a listed host resolves to.
	LocalNetwork bool `json:"localNetwork,omitempty"`
}

// Setting is a value the user gives a plugin, as an API key.
type Setting struct {
	Key         string `json:"key"`
	Title       string `json:"title"`
	Type        string `json:"type,omitempty"` // "text" (default) or "secret"
	Description string `json:"description,omitempty"`
	Default     string `json:"default,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

// Has reports whether the plugin declares capability c.
func (m *Manifest) Has(c string) bool { return slices.Contains(m.Capabilities, c) }

var (
	idPattern      = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)*$`)
	semverPattern  = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$`)
	keyPattern     = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,63}$`)
	hostPattern    = regexp.MustCompile(`^(\*\.)?[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)*$`)
	reservedPrefix = []string{"dalibre"}
)

// ReadManifest reads and checks dir/plugin.json.
func ReadManifest(dir string) (*Manifest, error) {
	data, err := os.ReadFile(filepath.Join(dir, "plugin.json"))
	if err != nil {
		return nil, err
	}
	return ParseManifest(data)
}

// ParseManifest parses and checks a plugin.json. Unknown fields are
// errors, so a misspelt one is caught rather than ignored.
func ParseManifest(data []byte) (*Manifest, error) {
	var m Manifest
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("plugin.json: %w", err)
	}
	if err := m.check(); err != nil {
		return nil, fmt.Errorf("plugin.json: %w", err)
	}
	return &m, nil
}

func (m *Manifest) check() error {
	switch {
	case len(m.ID) > 64 || !idPattern.MatchString(m.ID):
		return fmt.Errorf("id %q must be lowercase letters, digits and '-', in dot-separated parts, as \"gutenberg\" or \"com.example.books\"", m.ID)
	case slices.ContainsFunc(reservedPrefix, func(p string) bool { return m.ID == p || strings.HasPrefix(m.ID, p+".") }):
		return fmt.Errorf("id %q is reserved", m.ID)
	case strings.TrimSpace(m.Name) == "" || len(m.Name) > 64:
		return errors.New("name must be 1 to 64 characters")
	case !semverPattern.MatchString(m.Version):
		return fmt.Errorf("version %q must be a semantic version, as \"1.0.0\"", m.Version)
	case m.API < 1:
		return errors.New("api is missing; this Dalibre implements api 1")
	case m.API > APIVersion:
		return fmt.Errorf("needs plugin API %d; this Dalibre has %d, update Dalibre", m.API, APIVersion)
	case len(m.Capabilities) == 0:
		return errors.New("capabilities are missing")
	case len(m.Settings) > MaxSettings:
		return fmt.Errorf("at most %d settings", MaxSettings)
	}
	if m.Main == "" {
		m.Main = "main.js"
	}
	if main := filepath.ToSlash(m.Main); filepath.IsAbs(m.Main) || strings.HasPrefix(main, "/") ||
		slices.Contains(strings.Split(main, "/"), "..") || !strings.HasSuffix(main, ".js") {
		return fmt.Errorf("main %q must be a .js file inside the plugin's folder", m.Main)
	}
	for _, c := range m.Capabilities {
		if c != CapSource {
			return fmt.Errorf("unknown capability %q", c)
		}
	}
	if m.Has(CapSource) && len(m.Permissions.Hosts) == 0 {
		return errors.New("a source must list the hosts it reaches in permissions.hosts")
	}
	for _, h := range m.Permissions.Hosts {
		if !hostPattern.MatchString(h) || !strings.Contains(strings.TrimPrefix(h, "*."), ".") && h != "localhost" {
			return fmt.Errorf("host %q must be a lowercase host name, as \"example.com\" or \"*.example.com\"", h)
		}
	}
	keys := map[string]bool{}
	for _, s := range m.Settings {
		switch {
		case !keyPattern.MatchString(s.Key):
			return fmt.Errorf("setting key %q must be letters, digits and '_', starting with a letter", s.Key)
		case keys[s.Key]:
			return fmt.Errorf("setting %q is defined twice", s.Key)
		case strings.TrimSpace(s.Title) == "":
			return fmt.Errorf("setting %q has no title", s.Key)
		case s.Type != "" && s.Type != "text" && s.Type != "secret":
			return fmt.Errorf("setting %q: type must be \"text\" or \"secret\"", s.Key)
		}
		keys[s.Key] = true
	}
	return nil
}

// allows reports whether the permissions let the plugin reach host.
func (p *Permissions) allows(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	for _, h := range p.Hosts {
		if suffix, ok := strings.CutPrefix(h, "*"); ok {
			if strings.HasSuffix(host, suffix) && len(host) > len(suffix) {
				return true
			}
		} else if host == h {
			return true
		}
	}
	return false
}

// Covers reports whether p, as granted, allows everything want asks
// for. An update asking for more than was granted needs the user again.
func (p Permissions) Covers(want Permissions) bool {
	if want.LocalNetwork && !p.LocalNetwork {
		return false
	}
	for _, h := range want.Hosts {
		if !slices.Contains(p.Hosts, h) {
			return false
		}
	}
	return true
}

// Describe lists the permissions as the user is asked to grant them.
func (p Permissions) Describe() []string {
	var out []string
	for _, h := range p.Hosts {
		if d, ok := strings.CutPrefix(h, "*."); ok {
			out = append(out, "Connect to any subdomain of "+d)
		} else {
			out = append(out, "Connect to "+h)
		}
	}
	if p.LocalNetwork {
		out = append(out, "Connect to this computer and your local network")
	}
	return out
}
