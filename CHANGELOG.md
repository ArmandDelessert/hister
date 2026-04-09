# Changelog

## v0.12.0

### New Features

#### Web Crawler

New `hister index -r <url>` command crawls sites recursively using BFS traversal.
Supports an HTTP backend and a headless Chrome backend (chromedp).
Configurable depth, link count, allowed/excluded domains, and URL patterns.

#### PostgreSQL Backend

Full PostgreSQL support as an alternative to SQLite, including pgvector for semantic search.
Configure via a `postgres://` connection string in `server.database`.

#### Extractor Pipeline Overhaul

Extractors are now configurable, have explicit states (continue/done), and expose
a `Preview()` method used by the readability panel. New extractors included:

- Custom `pkg.go.dev` extractor for Go documentation pages
- Basic Stack Overflow extractor

#### Desktop Readability Panel

Focused search results load automatically in a split-pane reader on the right side
on screens wider than 1280 px. The panel is togglable and its open/closed state persists.

### Enhancements

- HTML sanitizer (bluemonday) applied to all extracted content
- `metadata` field added to documents for arbitrary key/value data
- `search` input type attribute on search fields for better mobile UX
- Build commit ID shown in the version string
- Admin users can create global indexes or indexes on behalf of other users
- `hister index` skips already-indexed URLs by default; pass `--force` to reindex them
- URL and domain wildcard matching automatically anchors to start and end
- Table of contents added to the API docs page
- Document indexed date shown in the preview panel
- Search query reflected in the browser tab title
- WebSocket communication optimised to reduce redundant round-trips
- Automatic redirect on zero results is now optional (configurable)
- `import` command renamed to `import-browser` to free `import` for index import/export

### Bug Fixes

- Browser history database opened read-only to avoid lock conflicts (#304)
- History entries now deleted when their associated document is deleted (#303)
- Crawler user-agent correctly applied after redirect handling (#302)
- Fixed field-specific alternation parts in query parser (#274)
- Negated query terms no longer trimmed twice
- HTML field no longer leaks into search results (#268)
- Expanded query hint only shown when the expansion is longer than the original query
- URL changes after HTTP redirects now resolved correctly
- Crawler no longer stops on HTTP errors
- Crawler timeout now applied during browser history import (#278)
- Pinned result titles no longer truncated on narrow screens
- Dark mode handled correctly in the preview panel
- Mobile layout no longer introduces unwanted line breaks
