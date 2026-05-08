# Refresh via launch-and-quit, not background sync

When `nogo refresh` triggers a cache update, it launches Notion via `open -a
Notion` (or `notion://` URL for a specific page), polls the SQLite file mtime
until it changes, then quits Notion via AppleScript (`tell application
"Notion" to quit`).

We considered keeping Notion running in the background (`open -gj`) but macOS
still creates visible windows regardless of `-g`/`-j` flags. The
launch-and-quit pattern takes ~3-5 seconds and leaves no lingering process.

The `--keep` flag is available for users who want Notion to stay running after
sync.