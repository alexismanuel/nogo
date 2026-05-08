# nogo

notion cli for agents on mac. no config, no auth.

## Requirements

- macOS with the Notion desktop app installed
- Go 1.21+ (for building from source)

## Acknowledgements

nogo was inspired by [notcrawl](https://github.com/openclaw/notcrawl), which mirrors Notion workspaces into SQLite with FTS search, a TUI browser, git-share sync, and API ingestion. nogo takes a narrower path: read-only cache access with no database layer, no API, and no archival features — just fetch and convert on demand. The trade-off is less power for zero config and instant runs.

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

# Search pages by title
nogo search meeting
nogo search --top design            # top-level items only

# Fetch a page as Markdown
nogo get abc123def456 --stdout
nogo get https://www.notion.so/My-Page-abc123def456 -o page.md

# Check if cached content is fresh
nogo info abc123def456

# Refresh a single page — opens Notion, syncs that page, quits
nogo refresh abc123def456
```

## Documentation

- **[CLI reference](docs/CLI.md)** — all commands, flags, and output formats
- **[Skill definition](docs/SKILL.md)** — agent-facing integration guide
- **[Architecture decisions](docs/adr/)** — why we chose SQLite-only, launch-and-quit, etc.

## Caveats

- **Only pages you've opened appear in results.** nogo reads Notion's local SQLite cache, which only contains pages you've loaded at least once in the Notion desktop app. If a page was never visited, it won't be in the cache — `nogo search` and `nogo list` will not find it. Use `nogo refresh <id>` to sync a page into the cache first.
