# SQLite cache only — no Notion API

nogo reads exclusively from Notion's local desktop SQLite cache. The Notion
REST API and OAuth flow were removed because they required a daemon, secrets,
and network access — defeating the goal of a lightweight, zero-config tool
that works offline and needs no credentials.

The trade-off: content is only as fresh as the last Notion desktop sync.
`nogo refresh` bridges this gap by programmatically launching Notion to
trigger its sync, then closing it.
