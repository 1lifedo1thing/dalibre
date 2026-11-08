package plugin

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// Native is a book source written in Go and run in place of a script, as
// Dalibre's own OPDS catalogs. It reaches the network only through the
// Getter it is handed, which makes the requests a script's fetch would:
// to the hosts the manifest allows, through the same sandboxed client.
type Native interface {
	// Search returns a page of books matching q.Text.
	Search(ctx context.Context, get Getter, q Query) (Page, error)
	// Collections are what the source offers to browse.
	Collections(ctx context.Context, get Getter) ([]Collection, error)
	// List returns a page of the collection q.Collection.
	List(ctx context.Context, get Getter, q Query) (Page, error)
}

// Getter makes a GET request the plugin may make.
type Getter func(ctx context.Context, url string) (*http.Response, error)

// NewNative makes a plugin of a book source written in Go. m describes it
// as a manifest would: its id, name and permissions; it isn't checked as
// a plugin.json is, so it may allow any host ("*"). Options.Authorize may
// add credentials to its requests.
func NewNative(m *Manifest, n Native, opts Options) *Plugin {
	p := &Plugin{Manifest: m, Builtin: true, native: n, ctx: context.Background(), logf: opts.Logf, settings: map[string]string{}, authorize: opts.Authorize}
	if p.logf == nil {
		p.logf = func(string, ...any) {}
	}
	p.client = p.httpClient(opts.Transport)
	return p
}

// Native returns the Go source behind a plugin made by NewNative; nil for
// a script.
func (p *Plugin) Native() Native { return p.native }

// getter is what a native source makes its requests with.
func (p *Plugin) getter() Getter {
	return func(ctx context.Context, url string) (*http.Response, error) { return p.get(ctx, url, nil) }
}

// nativePage calls a native source and checks what it returns as a
// script's page is checked.
func (p *Plugin) nativePage(ctx context.Context, call string, q Query) (Page, error) {
	var pg Page
	var err error
	if call == "list" {
		pg, err = p.native.List(ctx, p.getter(), q)
	} else {
		pg, err = p.native.Search(ctx, p.getter(), q)
	}
	if err != nil {
		return Page{}, p.nativeError(call, err)
	}
	out := Page{HasMore: pg.HasMore, Cursor: pg.Cursor}
	seen := map[string]bool{}
	for i, r := range pg.Items {
		if _, err := r.check(); err != nil || seen[r.ID] {
			p.logf("[%s] %s: items[%d] dropped: %v", p.Manifest.ID, call, i, err)
			continue
		}
		seen[r.ID] = true
		out.Items = append(out.Items, r)
	}
	return out, nil
}

// nativeError makes what a native source failed with an *Error, as a
// script's.
func (p *Plugin) nativeError(call string, err error) error {
	if e, ok := err.(*Error); ok {
		if e.Plugin == "" {
			e.Plugin = p.Manifest.ID
		}
		if e.Call == "" {
			e.Call = call
		}
		return e
	}
	return &Error{Plugin: p.Manifest.ID, Call: call, Kind: KindUnavailable, Msg: err.Error(), Err: err}
}

// HTTPError makes a failed response an *Error of the kind its status
// says, for native sources.
func HTTPError(resp *http.Response) *Error {
	kind := KindUnavailable
	switch {
	case resp.StatusCode == 401 || resp.StatusCode == 403:
		kind = KindAuth
	case resp.StatusCode == 404 || resp.StatusCode == 410:
		kind = KindNotFound
	case resp.StatusCode == 429:
		kind = KindRateLimited
	}
	return &Error{Kind: kind, Msg: fmt.Sprintf("%s answered %s", resp.Request.URL.Redacted(), resp.Status)}
}

// Fetcher makes GET requests as the plugin, for a native source's own
// pages beyond search and list: an OPDS catalog's feeds.
func (p *Plugin) Fetcher() Getter { return p.getter() }

// IsLocalHost reports whether host is this computer or on the local
// network, by the addresses it resolves to.
func IsLocalHost(ctx context.Context, host string) bool {
	if a, err := netip.ParseAddr(host); err == nil {
		return isLocal(a)
	}
	if host == "localhost" || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".localhost") {
		return true
	}
	addrs, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return false
	}
	for _, a := range addrs {
		if isLocal(a) {
			return true
		}
	}
	return false
}
