---
name: nogo
description: Read and convert Notion pages and databases from the local desktop cache to Markdown. Use when you need to fetch Notion content, check if cached data is fresh, or force a cache sync.
---

# nogo — Notion local cache skill

Reads Notion content from the desktop app's local SQLite cache. No API key or network needed.

## Commands

### Fetch content

```bash
nogo get <url-or-id> --stdout           # Get page as Markdown (pipe-friendly)
nogo get <url-or-id> -o path/to/file.md # Save to file
nogo get <url-or-id> --frontmatter      # Include YAML metadata
```

### Discover pages

```bash
nogo list                 # Top-level pages and databases
nogo list --all           # Everything in the cache
nogo list --time          # With last-edited timestamps
```

### Check freshness

```bash
nogo info <url-or-id>    # Page metadata + cache sync timestamps
nogo sync                 # Cache-wide sync timestamps
```

### Force a cache sync

```bash
nogo refresh              # Launch Notion, sync, quit (~3-5s)
nogo refresh <url-or-id>  # Sync a specific page
nogo refresh --keep       # Don't quit Notion after sync
```

## Freshness workflow

Before reading content, check if the cache is recent enough:

1. `nogo info <id>` — compare `Last edited` (remote) vs `Last auto-sync` (local)
2. If stale: `nogo refresh <id>` to force a sync
3. Then `nogo get <id> --stdout` to read the content

## Output formats

- **`get --stdout`**: Markdown to stdout, suitable for piping
- **`get --frontmatter`**: YAML frontmatter with `title`, `notion_id`, `notion_url`, `last_edited`
- **`list`**: Tab-separated: `TYPE  ID  TITLE`
- **`list --time`**: Tab-separated: `TYPE  ID  LAST_EDITED  TITLE`
- **`info`**: Human-readable key-value pairs
- **`sync`**: Human-readable key-value pairs

## Limitations

- **macOS only** — reads from `~/Library/Application Support/Notion/notion.db`
- **Cache is only as fresh as the last Notion sync** — use `nogo refresh` to update
- No write capability — nogo is read-only
- Nested child pages render as stubs (`📄 **Title** *(child page)*`)