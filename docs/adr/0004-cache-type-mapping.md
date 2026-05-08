# Cache type mapping — internal names to API names

The Notion desktop cache uses different type names than the REST API. For
example, `text` → `paragraph`, `header` → `heading_1`, `bulleted_list` →
`bulleted_list_item`, `collection_view_page` → `child_database`.

nogo maps these in `mapBlockType()` and `populateBlock()` so the downstream
Markdown converter works with API-style block types. This is a translation
layer, not a schema change — the original cache type names are never exposed
to consumers.