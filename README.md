# Dalibre

A calm, fast desktop library for e-books that reads and writes **calibre
libraries**. Written in Go with [MyGo](https://github.com/egoist/mygo)'s native
UI: no webview, no cgo, one binary.

Design: [Figma — Dalibre](https://www.figma.com/design/tZg6AMrNXoz9VYoa1xZoCo/Dalibre)

## Develop

```sh
go tool mygo dev ./cmd/dalibre        # run with live reload
go test ./...                         # unit + view tests (no window needed)
go run ./cmd/seed "testdata/Sample Library"   # a sample library to play with
```

Point the app at a library with `DALIBRE_LIBRARY` (PowerShell:
`$env:DALIBRE_LIBRARY="C:\path\to\Library"`); otherwise it opens the last one
used, then the one calibre itself uses.

View tests can save screenshots: `DALIBRE_SHOTS=<folder> go test ./internal/app`.

Build a release (exe + installer in `cmd/dalibre/build`):

```sh
go tool mygo build ./cmd/dalibre
```

## Layout

```
cmd/
  dalibre/            the app: main.go, mygo.json, resources/icon.png
  seed/               creates a sample calibre library for development
  dalibre-plugin/     CLI for plugin authors: init, check, search, download, pack, install
plugins/
  gutenberg/          Project Gutenberg, built into the app (dalibre.gutenberg)
sdk/                  for plugin authors: dalibre.d.ts, plugin.schema.json, template
internal/
  calibre/            calibre library access: schema, SQL functions its
                      triggers need, naming rules, CRUD, OPF
  ebookmeta/          title/authors/cover from EPUB, MOBI/AZW3 and file names
  catalog/            search, filters, sorting and sidebar counts (no UI)
  convert/            conversion model and engine detection (UI only in v1)
  plugin/             plugins: manifest, sandboxed JS runtime, consent, packages
  settings/           user settings persisted as JSON
  thumbs/             on-disk cover thumbnails
  app/                state, actions and views of the window
    theme/            design tokens (Radix Colors, spacing, radii) and fonts
    icons/            Lucide icons
    widgets/          the Figma library's components
```

## Plugins

Get books (in the sidebar) searches and browses book sources and adds
what you pick to your library. Project Gutenberg is built in; anyone can
add a source as a plugin: a `plugin.json` and a JavaScript file, run
sandboxed inside Dalibre. See
[Writing Dalibre plugins](docs/plugins/README.md) and the
[design](docs/plugins/design.md).

## Compatibility with calibre

- New libraries use calibre's own `metadata.db` schema (user_version 28).
- Dalibre registers the SQL functions calibre's triggers call (`title_sort`,
  `uuid4`, …), so writes behave as calibre's do.
- Book folders and files follow calibre's naming (`Author/Title (id)/Title -
  Author.ext`, transliterated to ASCII), and move when the title or first
  author changes.
- Edits write `metadata.opf` and mark the book in `metadata_dirtied`, so
  calibre refreshes it next time.
- Removed books go to the system Recycle Bin.

## Licenses

`internal/calibre/metadata_sqlite.sql` comes from calibre (GPL-3.0). Fonts
(Inter, Geist Mono) are under the SIL Open Font License; icons
are Lucide (ISC); colors are Radix Colors (MIT).
