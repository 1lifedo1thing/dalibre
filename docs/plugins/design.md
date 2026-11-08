# Plugin system: design

For Dalibre's developers. Plugin authors read [README.md](README.md),
which is the contract.

Status: plugin API 1, source plugins. Engine, consent, packaging, author
CLI and library import are done and tested; the app's UI for plugins is
next, designed in Figma first.

## Goals

1. **Anyone can write one.** A text editor and one CLI; no toolchain,
   no build step, the same on Windows, macOS and Linux.
2. **Installing one is safe.** Plugins are untrusted code. A plugin
   reaches only the hosts the user granted, never the disk, other
   processes or the local network (unless granted), and can't hang or
   crash the app.
3. **A stable contract.** Plugins written today keep working; mistakes
   in them are reported precisely to their author and degrade gracefully
   for users.
4. Dalibre stays one binary with no cgo.

## Runtime

JavaScript (ES2023, ES modules) in
[modernc.org/quickjs](https://pkg.go.dev/modernc.org/quickjs): Fabrice
Bellard's QuickJS transpiled to pure Go by the author of the
`modernc.org/sqlite` we already use. One interpreter (VM) per plugin.

| Option | Why not |
|---|---|
| goja (pure-Go JS) | The first version used it. No memory limit: a one-line plugin grew the heap to 1.2 GB in 3 s, stoppable only by timeout. Incomplete ES2015+, no ES modules. |
| QuickJS via cgo (`buke/quickjs-go`) | cgo breaks the one-binary, no-cgo build. |
| QuickJS via wasm (`fastschema/qjs`) | v0.0.6, untouched since 2025. |
| Go `plugin` | No Windows; same toolchain and cgo required. |
| Subprocess + JSON-RPC | Users must install a runtime or authors ship binaries per OS; a process has the whole disk and network. |
| WebAssembly (any language) | Every author needs a compiler and a build; too much for "call an API, map JSON". |

Measured against goja on Windows: a memory bomb is stopped at the 64 MB
limit in 55 ms; a new VM takes 0.15 ms; the app binary grows by 3.4 MB
(43.4 → 46.9 MB). Speed is close (modernc's benchmark puts goja slightly
ahead); plugins wait on the network anyway.

### The host boundary

The Go side and the script share four host functions that take and
return strings, and nothing else:

| | |
|---|---|
| `__host_info()` | the API version, plugin id/version and settings, as JSON |
| `__host_fetch(url, init)` | makes the request (all checks in Go), returns the response or an error as JSON |
| `__host_log(level, text)` | logs, masking secrets |
| `__host_settle(json)` | the outcome of the call in progress |

`internal/plugin/prelude.js` runs first in every VM: it takes those
functions off the global object, builds the frozen `dalibre` object and
`console` on them, and defines `__dalibre_load` and `__dalibre_call`
(non-writable), which import the plugin and call its exports, reporting
each outcome (a value, or a thrown error with its `kind`) through
`__host_settle`. Go calls one of the two, drains the promise job queue,
and reads what was settled. Everything crosses as JSON, so neither side
ever holds the other's objects.

Modules load through a Go loader: only relative `.js`/`.mjs` paths
inside the plugin's folder, 4 MB in all.

## Trust model

The plugin author is untrusted; the user decides what a plugin may do,
once, at install.

| Threat | Defence |
|---|---|
| Read or write the user's files | The interpreter has no file, process or module APIs. Downloads are written by the host, to a temp folder, under a sanitized name. |
| Reach hosts it didn't declare | Every request, redirect, download and cover URL is checked against the granted `permissions.hosts`. Only http(s); no credentials in URLs. |
| Reach the user's machine or LAN (router admin, local services) | Without `localNetwork`, the dialer refuses loopback, private, link-local, CGNAT, multicast and unspecified addresses **after** DNS resolution, so a listed host resolving to 127.0.0.1 (DNS rebinding) fails too. |
| Exfiltrate other plugins' secrets | Each plugin has its own runtime and only its own settings. |
| Hang or overload the app | Per-call deadline (60 s; interrupts the script), 64 MB of memory per plugin, 50 requests per call, 16 MB per fetch body, stack depth cap, 4 MB of scripts, results capped at 100. Calls into one plugin are serialized. A VM that ran out of time or memory is discarded and the plugin loaded afresh on its next call. |
| Huge or hostile files | Downloads capped at 2 GB and streamed; covers 10 MB; packages 32 MB / 1000 files, zip-slip and non-regular entries refused. |
| Gain permissions silently in an update | Granted permissions are stored per plugin; a version asking for more stays unloaded until the user grants again. |
| Deface the host API | `dalibre`, its settings and info are frozen. |
| Leak secrets through logs | Values of `secret` settings are masked in plugin logs. |

### Known gaps

- **Memory safety of the engine.** QuickJS is C transpiled to Go: a
  memory bug in it (QuickJS has had CVEs) corrupts memory in our process,
  where a bug in a pure-Go engine couldn't. Accepted: it bought the
  memory limit, which stops an attack anyone can write in one line, while
  this one needs an unknown engine bug. Keep the dependency current.
- **VMs are not collected by Go.** Each holds memory outside Go's heap
  until `Plugin.Close`; the app closes plugins it replaces.
- **CPU in native code.** Interrupts are checked between JS
  instructions; a single huge native call (a giant regex, `JSON.parse` of
  16 MB) can overrun the deadline briefly.
- **Proxies.** When the user has an HTTP(S) proxy in the environment, the
  proxy is dialed as is and resolves names itself, so the local-network
  check is up to the proxy. Common for users in China (local proxy on
  127.0.0.1); not worth breaking.
- **Secrets at rest** live in `settings.json` in plain text. Should move
  to the OS keychain.
- **No signing.** A package's integrity is whatever its download channel
  gives. Signing makes sense together with a plugin index.

## Lifecycle

```
               Inspect(package)          user grants
 .dalibre-plugin ───────────▶ consent? ───────────────▶ Install → <plugins>/<id>/
                                 │ already covered                  │
                                 └──────────────────────────────────┤
 startup / settings change                                          ▼
 Discover(<plugins>) ── for each: disabled? granted ⊇ asked? ── Load → ready
                                       │ no
                                       └▶ pluginsPending (UI asks; askConsent → grantConsent)
```

State per plugin id, in `settings.json`:

```json
"plugins": {
  "gutenberg": { "disabled": false, "granted": { "hosts": ["gutendex.com", "…"] }, "values": { "languages": "en" } }
}
```

The plugins folder is `<config>/Dalibre/plugins` (`%AppData%` on
Windows), or `$DALIBRE_PLUGINS`. Install unpacks into a hidden temp
folder inside it, then swaps it in by rename, so a failed install leaves
the old version.

## Calls

Arguments and results cross between Go and JS as JSON only. `call` runs
the export under the deadline and drains the job queue; `fetch` does its
I/O synchronously and returns a settled promise, so an async export has
settled by then. A promise still pending means the script awaited
something that can never settle (there are no timers): an error.

What a plugin returns is decoded strictly and checked field by field
(`source.go`):

- the page shape wrong → the call fails (`invalid`), naming what was
  returned and what was expected;
- one result wrong → it's dropped with a warning naming the item and
  field (`items[3].formats[0].format: "EPUBB" isn't one of …`);
- an optional field wrong or unknown → ignored, with a warning.

Warnings go to the log and `dalibre-plugin`; users never see them.

### Errors

Every failure is a `*plugin.Error` with a `Kind` the UI acts on:

| Kind | From | UI |
|---|---|---|
| `auth` | thrown; HTTP 401/403 on download | offer the plugin's settings |
| `settings` | a `required` setting is empty (checked before calling) | offer the plugin's settings |
| `rateLimited` | thrown; 429 | try later |
| `unavailable` | thrown; network failure; 5xx | try later |
| `notFound` | thrown; 404/410; format not offered | say so |
| `invalid` | thrown; malformed return | "the plugin failed", details in log |
| `timeout` | deadline or cancel | say so |
| `permission` | a refused request | "the plugin failed", details in log |
| `script` | any other exception | "the plugin failed", stack in log/CLI |

## Download into the library

`App.downloadBook(plugin, result, format)`:

1. `Resolve`: the format's URL, or the plugin's `resolve`.
2. Host downloads into a temp folder (progress to the UI, cancel).
3. Host fetches the cover, if any (failure ignored).
4. `Library.ImportWith(file, metadata, cover)`: the plugin's metadata
   wins, the file fills gaps; identifiers merged; the file's cover first.
5. Temp folder removed, library reloaded, new book selected.

## Code

| | |
|---|---|
| `internal/plugin/manifest.go` | manifest, validation, permissions, consent check |
| `internal/plugin/plugin.go` | the interpreter: load, discover, modules, calls, recovery, error mapping |
| `internal/plugin/prelude.js` | the script side of the host boundary: `dalibre`, `console`, calling exports |
| `internal/plugin/host.go` | host functions, `fetch`, network sandbox, error kinds |
| `internal/plugin/source.go` | the source contract: search, checking, resolve, download, cover |
| `internal/plugin/install.go` | inspect, install, uninstall, pack |
| `internal/app/plugins.go` | loading, consent, enable/disable, settings, `downloadBook` |
| `internal/settings` | per-plugin state |
| `internal/calibre/importer.go` | `ImportWith` |
| `cmd/dalibre-plugin` | the author CLI |
| `sdk/` | `dalibre.d.ts`, `plugin.schema.json`, the `init` template (embedded) |
| `plugins/gutenberg` | reference plugin |

## Next: UI (Figma first)

Get books as one fixed page with a source picker, and plugins with
several capabilities: see [capabilities.md](capabilities.md) and Figma
Screens › 07 · Get books.

1. **Get books**: source picker, search, results (cover, title, authors,
   language, formats with sizes), load more, download per format.
2. **Downloads** in the jobs popover beside conversions: progress,
   cancel, retry; error kinds turned into actions ("Open settings").
3. **Plugins** in settings: installed list (version, author, state:
   on / off / waiting for permission / failed to load), settings forms
   with masked secrets, enable/disable, uninstall, "Install from file…",
   "Open plugins folder", "Reload".
4. **Consent dialog**: name, author, homepage, description, and the
   permissions in plain words (`Permissions.Describe`), with the local
   network called out; for an update, only what's new.

## Later

- Install from a URL; check for updates (manifest `updateUrl`).
- A plugin index (a JSON file in a repo) to browse and install from,
  with signed packages.
- Secrets in the OS keychain.
- `dalibre.storage` (small per-plugin key–value store) for login tokens
  and caches that survive restarts.
- `dalibre.html` (parse + CSS selectors) for sites without an API;
  `URL`/`URLSearchParams`.
- Login flows (open the browser, receive a callback) for OAuth sources.
- Duplicate check before downloading, by identifier and title/author.
- More capabilities: `metadata` (fetch metadata and covers for existing
  books, as calibre's metadata sources), `opds` (expose a source as OPDS),
  event hooks.
- A subprocess or wasm runtime behind `"runtime"` in the manifest, for
  plugins needing native code or a memory limit.
- A `dalibre-plugin test` harness with recorded HTTP fixtures, so
  authors can run their plugin in CI.

## Open questions

- **Distribution and liability.** Do we run a plugin index? If so, what
  do we accept (public-domain and licensed sources only?), and who
  reviews? Without one, users find packages on their own and Dalibre
  stays out of what they download.
- **Deprecation policy** for API 1 once API 2 exists: how long, and how
  do we tell users.
- **Metadata precedence**: the plugin's metadata wins over the file's.
  Right for clean catalogues; should a plugin be able to say otherwise?
- **Fan-out search**: search one source at a time, or all at once with
  results grouped by source?
- **Where `dalibre-plugin` ships**: in the app bundle, as a separate
  download, or as `dalibre plugin …` subcommands of the app binary.
