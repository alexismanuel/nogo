# nogo

Zero-config Mac Notion cache reader to Markdown.

## Install

```bash
go install .
```

## Quick start

```bash
nogo list                          # list pages from cache
nogo get <url-or-id> --stdout     # fetch a page as Markdown
nogo info <url-or-id>             # check page metadata + freshness
nogo refresh                       # sync cache (launches Notion, then quits)
```

## Documentation

- **[CLI reference](docs/CLI.md)** — all commands, flags, and output formats
- **[Skill definition](docs/SKILL.md)** — agent-facing integration guide
- **[Architecture decisions](docs/adr/)** — why we chose SQLite-only, launch-and-quit, etc.

## Acknowledgements

nogo was inspired by [notcrawl](https://github.com/openclaw/notcrawl), which mirrors Notion workspaces into SQLite with FTS search, a TUI browser, git-share sync, and API ingestion. nogo takes a narrower path: read-only cache access with no database layer, no API, and no archival features — just fetch and convert on demand. The trade-off is less power for zero config and instant runs.

## Requirements

- macOS with the Notion desktop app installed