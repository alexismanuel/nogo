# Substring search, title-only, no local index

`nogo search` filters page and collection titles by case-insensitive exact
substring match. It does not search body content and does not maintain a local
search index.

## Alternatives considered

- **FTS5 local index** (notcrawl-style): Requires a local SQLite archive and a
  sync pipeline. Breaks the zero-config, no-local-state contract established in
  ADR-1. Suitable for a different product.

- **Body scan on every search**: Reading every block's `properties` JSON into
  Go and filtering in-process would be slow on large caches and adds complexity
  for a marginal gain over title search.

- **Fuzzy/semantic matching**: Fuzzy matching (stemming, edit distance) adds
  dependency complexity for marginal improvement. Semantic/LLM search requires
  an API key or local model, which breaks the offline constraint. Users who
  need semantic matching can pipe `nogo list` or `nogo search` output into their
  own LLM.

## Decision

Search is a convenience filter on what `list` already returns. The
implementation reuses `ListPages()` and filters in Go — no new query methods on
Reader. The command is separate from `list` because the intent (finding vs
browsing) is different, and it may acquire different flags later.

Multi-word queries match as a literal substring (`nogo search project meeting`
finds titles containing `"project meeting"` as-is).

No results exits with code 1 (script-composable failure signal).