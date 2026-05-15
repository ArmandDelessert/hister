---
date: '2026-05-15T00:00:00+00:00'
draft: false
title: 'Rules'
---

Rules let you control how Hister indexes and surfaces documents. They live in
`{data_directory}/rules.json` in single-user mode, or per-user in the database
when [user handling](user-handling) is enabled. The **Rules** tab in the web
interface (and the TUI) is the easiest way to manage them.

## Rule Types

### Skip rules

Skip rules prevent matching URLs from being added to the index. When a URL
matches any pattern in the skip list, Hister silently discards the document
during indexing and during `hister reindex`.

**Use cases**: block ad networks, login pages, cookie-consent walls, or any
site you never want in your search results.

```
^https://ads\.example\.com
^https?://(login|mail)\.example\.com/
.*\?utm_source=
```

### Priority rules

Priority rules push matching documents to the top of search results regardless
of their relevance score. Hister applies a large score boost to any document
whose URL matches a priority pattern.

**Use cases**: always surface your personal wiki, your company's internal
documentation, or a trusted source before other results.

```
^https://wiki\.example\.com/
^https://docs\.example\.com/
```

### Versioning rules

Versioning rules tell Hister to **track changes** to a document every time it
is re-indexed. When a URL matches a versioning pattern and the document content
differs from the previously indexed version, Hister stores a unified diff of
the changes in the database.

**Use cases**: monitor pages for edits (privacy policies, documentation, news
articles), build a personal changelog of sites you follow, or audit when a
trusted resource last changed.

```
^https://example\.com/privacy-policy$
^https://docs\.example\.com/
```

#### Viewing stored versions

Once Hister has recorded at least one version diff for a document, a **version
count badge** appears on the document's card in the search results and in the
offline preview popup. Clicking it opens a changelog that shows each recorded
diff with its timestamp.

Previous versions are also checked when you open the offline preview of a
document: if the stored HTML has changed since the previous visit, the diff is
shown inline in the preview.

#### API

```
GET /api/versions?url=<url>
```

Returns all stored version diffs for the given URL (newest first). Each entry
contains `id`, `created_at`, `url`, `user_id`, `html_diff`, and `text_diff`
fields. The diff values are [diff-match-patch](https://github.com/google/diff-match-patch)
patch strings.

### Aliases

Aliases are query-time shortcuts: before Hister executes a search, it replaces
any alias keyword with its expanded form.

```json
{
  "gh": "domain:github.com",
  "local": "type:file",
  "work": "domain:(internal.example.com|jira.example.com)"
}
```

With the `work` alias above, searching for `work deployment` is equivalent to
`domain:(internal.example.com|jira.example.com) deployment`.

## Pattern syntax

Skip, priority, and versioning rules are matched against the **full URL**
(including scheme, host, path, and query string). A few important details:

- Patterns follow [Go regular expression syntax](https://pkg.go.dev/regexp/syntax).
  Look-ahead and look-behind are **not** supported.
- Anchoring must include the scheme: `^https://foo.com` or
  `^https?://(login|mail)\.` are valid; `^foo.com` is not.
- The URL hash is stripped before matching
  (`https://foo.com/#section` becomes `https://foo.com/`).
- Query-string parameters are **not** reordered; only `utm_*` parameters are
  stripped.
- A trailing `$` anchor will **not** match URLs that have a query string:
  `/login$` does not match `https://foo.com/login?auth=1`.

## Storage

In **single-user mode** (user handling disabled), rules are saved to and read
from `rules.json` in the configured data directory. Changes made through the
web interface or API update this file directly.

In **multi-user mode** (user handling enabled), each user has a private copy of
their rules stored in the database. Changes affect only the authenticated user
and do not touch `rules.json`. See [User Handling](user-handling) for details.
