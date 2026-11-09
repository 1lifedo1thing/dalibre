# Changelog

Each version's section is the release notes that Dalibre's update window
shows, so write them for readers of the app. `mygo build` needs a
`## <version>` section for the version in `cmd/dalibre/mygo.json`.

## 0.3.0

- Wireless devices: KOReader, and other reading apps that connect to
  calibre over Wi‑Fi, connect to Dalibre the same way. Turn on Wireless
  devices in the toolbar's Sharing button, then choose Calibre › Connect
  in KOReader. Send books to the reader, or remove them, from the book
  menu; books whose metadata you change reach it too. Settings › Wireless
  devices has its port and an optional password.

## 0.2.0

- Your own AI agents, such as Claude Code, Claude Desktop and Cursor, can
  use your library over MCP: search it, read and edit metadata, add and
  remove books, and convert formats with their own tools. Turn it on from
  the new MCP button in the toolbar; Settings › MCP has the setup for each
  agent and lists every tool.
- While an agent works, the MCP button shows which one, and its popover
  lists what each agent did.
- Sharing the library over OPDS moves to its own button in the toolbar.

## 0.1.1

- Feedback, above Settings in the sidebar, reports a bug or requests a
  feature on GitHub, with Dalibre's version and your system filled in for
  bug reports, or shows the issues already open.

## 0.1.0

The first release of Dalibre, a desktop library for e-books that reads and
writes calibre libraries.

- Browse, search, filter and sort your calibre library, as a grid of covers
  or a list.
- Import books, and fetch their metadata from sources such as NeoDB.
- Get books from Project Gutenberg and from sources that plugins add.
- Switch between libraries, and share books.
- Dalibre updates itself: it checks for new versions in the background, and
  from Settings › General or, on macOS, Dalibre > Check for Updates….
