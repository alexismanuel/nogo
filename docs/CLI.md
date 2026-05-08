# CLI Reference

## `nogo search <query>`

Search pages and databases in the local Notion cache by title.

Performs case-insensitive exact substring matching. Multi-word queries match as a literal string (`nogo search project meeting` finds titles containing `"project meeting"`).

By default, searches all pages and databases, excluding collection rows and template copies. Use `--top` to restrict to top-level items only.

Exits with code 1 when no results match.

### Flags

| Flag | Default | Description |
|---|---|---|
| `--top` | false | Restrict to top-level items only (pages/databases at the workspace root) |
| `--time` | false | Show last edited time for each result |

### Output

Same tab-separated format as `nogo list`: `TYPE  ID  TITLE` (or `TYPE  ID  LAST_EDITED  TITLE` with `--time`).

### Examples

```bash
# Search all pages and databases
nogo search meeting

# Search top-level items only
nogo search --top design

# Search with timestamps
nogo search --time roadmap
```

---

## `nogo get <url-or-id>`

Fetch a Notion page or database from the local cache and convert it to Markdown.

The page must have been synced by the Notion desktop app — nogo reads the local SQLite cache, not the API. Use `nogo refresh <id>` to force a sync if content is stale.

### Arguments

| Arg | Description |
|---|---|
| `<url-or-id>` | A Notion page/database URL, or a raw UUID (with or without dashes) |

URL formats accepted:
```
https://www.notion.so/My-Page-abc123def456
https://www.notion.so/workspace/abc123def456
abc123def456
abc123de-f456-7890-abcd-ef1234567890
```

### Flags

| Flag | Default | Description |
|---|---|---|
| `-o, --output` | `<title>.md` | Output file path. Creates parent directories if needed. |
| `--stdout` | false | Print Markdown to stdout instead of saving to a file |
| `--frontmatter` | false | Prepend YAML frontmatter with `title`, `notion_id`, `notion_url`, `last_edited` |

### Behavior

- **Pages**: Converts the block tree to GFM Markdown — headings, lists (nested), code blocks, callouts, tables, equations, etc.
- **Databases**: Renders as a Markdown table with columns from the collection schema. Supports title, rich text, select, multi_select, number, checkbox, date, url, email, status, people, relation, files.
- **Fallback**: If the given ID matches both a database and a page, the database is returned.

### Examples

```bash
# Fetch a page by URL
nogo get https://www.notion.so/My-Page-abc123def456

# Fetch by raw ID
nogo get abc123def456

# Save to a specific file
nogo get abc123def456 -o notes/meeting.md

# Pipe to stdout (useful for piping to other tools)
nogo get abc123def456 --stdout

# Include YAML frontmatter
nogo get abc123def456 --frontmatter
```

### Output

On success, prints the output path to stderr:
```
Saved to: /path/to/my-page.md
```

On error, prints a descriptive message to stderr and exits with code 1.

---

## `nogo list`

List pages and databases from the local Notion cache.

By default, lists all pages and databases, excluding collection rows and template copies. Use `--top` to show only top-level items (at the workspace root).

### Flags

| Flag | Default | Description |
|---|---|---|
| `--top` | false | Show top-level items only (pages/databases at the workspace root) |
| `--time` | false | Show last edited time for each item |

### Output

Tab-separated columns: `TYPE`, `ID`, `TITLE` (or `TYPE`, `ID`, `LAST_EDITED`, `TITLE` with `--time`).

```
TYPE      ID                                    TITLE
page      27626ae5-...-c74854b66aa3  Git worktrees 101
database  d3d26ae5-...-014512d331ec  People
```

### Examples

```bash
# All pages and databases
nogo list

# Top-level items only
nogo list --top

# With timestamps
nogo list --time
```

---

## `nogo info <url-or-id>`

Show metadata and cache freshness for a page or database.

### Arguments

| Arg | Description |
|---|---|
| `<url-or-id>` | A Notion page/database URL or UUID (same formats as `get`) |

### Output

```
Type:         page
ID:           27626ae5-6769-80a3-9163-c74854b66aa3
Title:        Git worktrees 101
Last edited:  2026-04-24T14:55:43Z
Created:      2025-09-22T06:41:40Z

Cache sync:
  Last auto-sync: 2026-05-08T10:53:27Z
  Last refetch:   2026-05-08T10:57:32Z
  DB modified:    2026-05-08T11:02:27Z
```

The **Cache sync** section shows when Notion last updated the local database:
- **Last auto-sync**: When Notion last performed an automatic incremental sync
- **Last refetch**: When Notion last performed a full data re-download
- **DB modified**: Wall-clock time the `notion.db` file was last written to

Compare `Last edited` (remote) against `Last auto-sync` (local) to judge staleness.

---

## `nogo sync`

Show when the local Notion cache was last synced. No arguments, no network.

Reads three timestamps from Notion's local SQLite cache:

| Field | Source | Meaning |
|---|---|---|
| **DB modified** | File mtime of `notion.db` | Wall-clock time the cache was last written to. The most reliable "something changed" signal. |
| **Last auto-sync** | `offline_download_metadata` (`autosync`) | When Notion last ran an incremental sync — downloaded changed blocks since the last sync. |
| **Last refetch** | `offline_download_metadata` (`refetch`) | When Notion last performed a full re-download of workspace data. |

### How to use it

Compare **Last auto-sync** against a page's **Last edited** time (from `nogo info`) to judge staleness. If auto-sync is older than the page edit, the cache is stale — run `nogo refresh`.

If **DB modified** is recent but auto-sync is old, Notion may have synced partially (e.g. only pages you opened).

### Output

```
DB modified:    2026-05-08T11:02:27Z
Last auto-sync: 2026-05-08T10:53:27Z
Last refetch:   2026-05-08T10:57:32Z
```

### Examples

```bash
nogo sync
```

---

## `nogo refresh <url-or-id>`

Force a cache sync for a specific page by opening it in Notion, waiting for the cache to update, then quitting Notion.

A page ID or URL is **required** — Notion's autosync on launch only syncs recently-accessed content, so refreshing without a target page is unreliable.

### Arguments

| Arg | Description |
|---|---|
| `<url-or-id>` | Required. A Notion page/database URL or UUID (same formats as `get`). |

### Flags

| Flag | Default | Description |
|---|---|---|
| `--wait` | 30 | Max seconds to wait for the cache to update. 0 = don't wait (fire and forget). |
| `--keep` | false | Leave Notion running after sync instead of quitting it. |

### How it works

1. Opens the specified page in Notion via `notion://` URL scheme
2. Polls the `notion.db` file mtime every second until it changes
3. Quits Notion via AppleScript (`tell application "Notion" to quit`)

Typical cycle time: 3–5 seconds.

### Examples

```bash
# Sync a specific page, then quit Notion
nogo refresh abc123def456

# Sync using a full URL
nogo refresh https://www.notion.so/My-Page-abc123def456

# Allow more time for large pages
nogo refresh abc123def456 --wait=60

# Sync and keep Notion running
nogo refresh abc123def456 --keep
```

### Caveats

- **macOS only.** The AppleScript quit mechanism is macOS-specific.
- **Notion must be installed.** nogo uses the Notion desktop app as its sync engine.
- **The Notion window will briefly appear** during the sync cycle. There is no way to prevent this on macOS.