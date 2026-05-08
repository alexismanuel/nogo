# Refresh via launch-and-quit, targeting a specific page

When `nogo refresh <page-id>` triggers a cache update, it opens that specific
page in Notion via the `notion://` URL scheme, polls the SQLite file mtime
until it changes, then quits Notion via AppleScript (`tell application
"Notion" to quit`).

A page ID or URL is required — we do **not** support refreshing without a
target. Launching Notion without a specific page provides no guarantee that
any given page's cache will be updated; Notion's autosync may only refresh
recently-accessed content. Requiring a page ID ensures the user gets the
content they actually want refreshed.

We considered keeping Notion running in the background (`open -gj`) but macOS
still creates visible windows regardless of `-g`/`-j` flags. The
launch-and-quit pattern takes ~3-5 seconds and leaves no lingering process.

The `--keep` flag is available for users who want Notion to stay running after
sync.