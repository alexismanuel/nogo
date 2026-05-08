# nogo

Zero-config Mac Notion cache reader to Markdown.

## Language

**Cache**: The Notion desktop app's local SQLite database at `~/Library/Application Support/Notion/notion.db`. Contains all pages, blocks, collections, and metadata for every workspace the user has opened.

_Avoid_: database, local store, offline data

**Snapshot**: A temporary copy of `notion.db` that nogo creates on every `Open()` call. Needed because SQLite locks the WAL during concurrent access.

_Avoid_: copy, temp db

**Block**: A Notion content unit (paragraph, heading, list item, etc.). The cache stores blocks with internal type names (`text`, `header`, `bulleted_list`) that differ from the API names (`paragraph`, `heading_1`, `bulleted_list_item`).

**Collection**: The cache's name for a database. Stored across the `block` and `collection` tables, linked via `format.collection_pointer`.

**Page ID**: A UUID identifying a Notion page or database. Accepts full URLs, stripped UUIDs, or dashed UUIDs — all normalized to dashed form for cache queries.

**Refresh**: The act of launching the Notion desktop app to trigger its sync mechanism, then closing it. Not an API call.

_Avoid_: sync (when meaning the user-triggered action), download, fetch (when meaning cache refresh)

**Sync timestamp**: Metadata in `offline_download_metadata` recording when Notion last performed autosync, refetch, or refetch_collections.

**Last edited time**: The `last_edited_time` field on a block in the cache. Epoch milliseconds. Not the same as sync timestamp — a page can be edited remotely but not yet synced locally.

## Relationships

- A **Cache** contains many **Blocks** organized in a tree (parent/child via `content` JSON array)
- A **Collection** is referenced by a `collection_view_page` **Block** via `format.collection_pointer`
- `nogo list` reads **Block** and **Collection** tables to enumerate items
- `nogo get` reads **Block** trees and converts them to Markdown
- `nogo info` reads **Block** metadata (`last_edited_time`, `created_time`) and **Sync timestamps**
- `nogo refresh <page-id>` opens a specific page in Notion via `notion://` URL scheme and polls the Cache mtime; requires a page ID because Notion's autosync without a target page is unreliable
- `nogo sync` reads `offline_download_metadata` and the Cache file mtime
- `nogo search` finds **Pages** and **Collections** by title substring match (case-insensitive); excludes collection rows and template copies; mirrors `nogo list` output format
- `nogo list` and `nogo search` show all items by default (`--top` for top-level only); exclude collection rows (`parent_table='collection'`) and template copies (`copied_from_pointer`) to reduce noise

## Example dialogue

> **Dev:** "I ran `nogo get` but the page content looks old."
> **Domain expert:** "Check `nogo info <id>` — compare the **last edited time** against the **sync timestamp** from `nogo sync`. If the sync is older, run `nogo refresh <id>` to force Notion to update the **cache**."

> **Dev:** "Why does `nogo refresh` require a page ID? Can't it just refresh everything?"
> **Domain expert:** "Notion's autosync on launch only syncs recently-accessed content — there's no guarantee a specific stale page gets refreshed. `nogo refresh <id>` opens that exact page, forcing Notion to fetch it."

## Flagged ambiguities

- "sync" can mean either Notion's internal autosync mechanism or the user running `nogo sync` to check timestamps. Resolved: `nogo sync` is the CLI command; **sync timestamp** is the cache metadata.
- "database" in Notion means a structured table (a **Collection** in the cache), not a relational database. The cache itself is a SQLite database. Resolved: use **Collection** for Notion databases, **Cache** for the SQLite file.
- Semantic or LLM-powered search belongs outside nogo. Users can pipe `nogo list` or `nogo search` output into an LLM for fuzzy/semantic matching. Resolved: nogo ships substring search only; no local embedding model or API key.