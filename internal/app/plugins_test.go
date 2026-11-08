package app

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"dalibre/internal/plugin"
)

// fakeWeb answers the plugin's requests without a network.
type fakeWeb struct{ h http.Handler }

func (f fakeWeb) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.Host == "" {
		r.Host = r.URL.Host
	}
	rec := httptest.NewRecorder()
	f.h.ServeHTTP(rec, r)
	resp := rec.Result()
	resp.Request = r
	return resp, nil
}

func TestDownloadBookFromPlugin(t *testing.T) {
	a := newTestApp(t)
	mux := http.NewServeMux()
	mux.HandleFunc("gutendex.com/books/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"next":null,"results":[{"id":84,"title":"Frankenstein","authors":[{"name":"Shelley, Mary Wollstonecraft"}],
			"subjects":["Horror tales"],"languages":["en"],
			"formats":{"application/epub+zip":"https://www.gutenberg.org/ebooks/84.epub"}}]}`))
	})
	mux.HandleFunc("www.gutenberg.org/ebooks/84.epub", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not really an epub"))
	})
	p, err := plugin.Load(filepath.Join("..", "..", "plugins", "gutenberg"), plugin.Options{Transport: fakeWeb{mux}})
	if err != nil {
		t.Fatal(err)
	}
	a.plugins = []*plugin.Plugin{p}
	if len(a.sources()) != 1 {
		t.Fatal("the plugin isn’t a source")
	}
	page, err := p.Search(t.Context(), plugin.Query{Text: "frankenstein"})
	if err != nil {
		t.Fatal(err)
	}
	before := len(a.books)
	a.downloadBook(p, page.Items[0], "EPUB")
	if d := a.downloads[0]; !d.finished || d.err != nil {
		t.Fatalf("download: %+v", d)
	}
	if len(a.books) != before+1 {
		t.Fatalf("books %d, want %d", len(a.books), before+1)
	}
	b := a.byID[a.curID]
	if b == nil || b.Title != "Frankenstein" || b.Authors[0] != "Mary Wollstonecraft Shelley" || b.Tags[0] != "Horror tales" {
		t.Fatalf("added %+v", b)
	}
	if d, err := a.lib.Details(b); err != nil || d.Identifiers["gutenberg"] != "84" {
		t.Fatalf("details %+v, %v", d, err)
	}
	if _, err := os.Stat(filepath.Join(a.lib.Root, b.Path)); err != nil {
		t.Fatal(err)
	}
}

// A plugin loads only once the user grants its permissions, and an
// update asking for more waits for the user again.
func TestPluginConsent(t *testing.T) {
	a := newTestApp(t)
	a.pluginDir = t.TempDir()
	pack := func(version, hosts string) string {
		src := filepath.Join(t.TempDir(), "src")
		os.MkdirAll(src, 0o755)
		os.WriteFile(filepath.Join(src, "plugin.json"), []byte(`{"id":"com.example.books","name":"Example","version":"`+version+
			`","api":1,"capabilities":["source"],"permissions":{"hosts":[`+hosts+`]}}`), 0o644)
		os.WriteFile(filepath.Join(src, "main.js"), []byte(`exports.search = () => ({items: []})`), 0o644)
		pkg, err := plugin.Pack(src, t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		return pkg
	}

	a.installPlugin(pack("1.0.0", `"a.example"`))
	if a.consent == nil || len(a.plugins) != 0 {
		t.Fatal("installed without asking")
	}
	a.grantConsent()
	if len(a.plugins) != 1 || a.plugins[0].Manifest.Version != "1.0.0" {
		t.Fatalf("plugins %d after granting", len(a.plugins))
	}

	// An update within what was granted installs at once.
	a.installPlugin(pack("1.1.0", `"a.example"`))
	if a.consent != nil || a.plugins[0].Manifest.Version != "1.1.0" {
		t.Fatal("an update needing nothing new asked")
	}

	// One asking for a new host is asked about; installed by hand, it
	// waits unloaded.
	if _, err := plugin.Install(pack("2.0.0", `"a.example","b.example"`), a.pluginDir); err != nil {
		t.Fatal(err)
	}
	a.loadPlugins()
	if len(a.plugins) != 0 || len(a.pluginsPending) != 1 {
		t.Fatalf("plugins %d, pending %d", len(a.plugins), len(a.pluginsPending))
	}
	a.askConsent("com.example.books")
	a.grantConsent()
	if len(a.plugins) != 1 || len(a.pluginsPending) != 0 {
		t.Fatal("not loaded after granting")
	}

	a.setPluginEnabled("com.example.books", false)
	if len(a.plugins) != 0 {
		t.Fatal("a disabled plugin loaded")
	}
	a.uninstallPlugin("com.example.books")
	if _, ok := a.settings.Plugins["com.example.books"]; ok {
		t.Fatal("uninstalling kept the plugin's state")
	}
}
