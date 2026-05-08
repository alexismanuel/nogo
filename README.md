# nogo

Zero-config Mac Notion cache reader to Markdown.

## Install

From source:

```bash
go install github.com/alexismanuel/nogo@latest
```

This puts `nogo` in `$(go env GOPATH)/bin` (usually `~/go/bin`). Make sure that's on your `$PATH`.

Or clone and build:

```bash
git clone https://github.com/alexismanuel/nogo.git
cd nogo
make install
```

## Quick start

```bash
# Discover what's in your cache
nogo list
nogo list --time                    # with last-edited timestamps

# Fetch a page as Markdown
nogo get abc123def456 --stdout
nogo get https://www.notion.so/My-Page-abc123def456 -o page.md

# Check if cached content is fresh
nogo info abc123def456

# Refresh a single page — opens Notion, syncs that page, quits
nogo refresh abc123def456

# Refresh the whole cache
nogo refresh
```

## Documentation

- **[CLI reference](docs/CLI.md)** — all commands, flags, and output formats
- **[Skill definition](docs/SKILL.md)** — agent-facing integration guide
- **[Architecture decisions](docs/adr/)** — why we chose SQLite-only, launch-and-quit, etc.

## Acknowledgements

nogo was inspired by [notcrawl](https://github.com/openclaw/notcrawl), which mirrors Notion workspaces into SQLite with FTS search, a TUI browser, git-share sync, and API ingestion. nogo takes a narrower path: read-only cache access with no database layer, no API, and no archival features — just fetch and convert on demand. The trade-off is less power for zero config and instant runs.

## Requirements

- macOS with the Notion desktop app installed