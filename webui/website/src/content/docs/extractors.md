---
date: '2026-04-07T11:00:00+00:00'
draft: false
title: 'Extractors'
description: 'Understand the extractor chain, configure built in extractors, and develop new content handlers.'
---

<script>
  import ConfigReference from '$lib/ConfigReference.svelte';

  const ytdlpOptions = [
    {
      name: 'binary',
      type: 'string',
      defaultValue: 'yt-dlp',
      description: 'Path to the yt-dlp executable. Use this when the binary is not on PATH or to select a specific version.',
    },
    {
      name: 'timeout',
      type: 'int',
      defaultValue: '15',
      description: 'Seconds to wait for yt-dlp to finish before aborting the request.',
    },
    {
      name: 'max_concurrent_jobs',
      type: 'int',
      defaultValue: '2',
      description: 'Maximum number of yt-dlp processes running at once. Additional jobs wait for a slot. Set it to zero for no limit.',
    },
    {
      name: 'fetch_subtitles',
      type: 'bool',
      defaultValue: 'false',
      description: 'Downloads subtitles or an automatically generated transcript and appends them to the indexed text and preview.',
    },
    {
      name: 'sub_language',
      type: 'string',
      defaultValue: 'auto',
      description: 'Subtitle language code to request. Auto selects automatically generated captions. Used only when fetch_subtitles is true.',
    },
    {
      name: 'cookies_file',
      type: 'string',
      defaultValue: '(none)',
      description: 'Path to a Netscape format cookies file passed to yt-dlp. Useful for restricted content.',
    },
    {
      name: 'cookies_from_browser',
      type: 'string',
      defaultValue: '(none)',
      description: 'Browser from which yt-dlp extracts cookies. This takes precedence over cookies_file.',
    },
    {
      name: 'extra_args',
      type: 'string[]',
      defaultValue: '(none)',
      description: 'Additional yt-dlp command line flags appended verbatim to every invocation.',
    },
  ];
</script>

Extractors are the components responsible for turning raw HTML or file content
into rich, searchable data. Every time a page is added to the index or a
document preview is requested, Hister runs the content through a chain of
extractors until one succeeds.

The chain design means specialist extractors run first; generic ones act as a
safety net for any content that no specialist handles.

## Purpose

Generic HTML-to-text conversion loses a lot of signal. A Stack Overflow answer,
a Go package reference, a local Markdown note, and a news article all have
different structure and a one-size-fits-all parser cannot take advantage of
that structure.

Extractors exist so that each kind of source can be handled in the most
**domain-specific** way possible. A specialist extractor for a particular
website or file format can:

- pull out the parts of the page that actually matter and discard noise (ads,
  navigation, boilerplate)
- produce richer plain text that makes search results more relevant
- surface structured details answers, code snippets, documentation sections
  that a generic parser would flatten or miss entirely
- enable to use a custom front-end template for the document preview panel,
  giving each content type its own layout and presentation

The goal is always to capture **more specialised, higher-quality information**
about the content being processed, so that search results and the document
preview are as useful as possible for the source in question.

When a page is fetched by the browser extension, the CLI, or the crawler
Hister receives its raw HTML (or file bytes). That content needs to be
processed to provide a full `Document` object.

## Extractor chain

Extractor capabilities place each implementation into one or more phases.
Indexing first runs every matching enricher in registration order. It then tries
matching content extractors in registration order until one succeeds. Preview
selection considers only extractors that declare the preview capability.

`Extract` and `Preview` return explicit result values. Each result records one
of three decisions:

| Decision | Meaning                                                                                              |
| -------- | ---------------------------------------------------------------------------------------------------- |
| Success  | The extractor handled the document successfully.                                                     |
| Fallback | The extractor was inconclusive, so the next matching extractor should be tried.                      |
| Abort    | A fatal error occurred. The chain stops immediately and returns the error without trying a fallback. |

Use `Extracted`, `ExtractFallback`, or `AbortExtraction` for extraction
results. Use `Previewed`, `PreviewFallback`, or `AbortPreview` for preview
results. The result fields are private, so an implementation cannot return a
contradictory decision and error combination. The zero value is invalid.

When a caller names a preview extractor explicitly, that extractor must be
enabled, preview capable, and matched to the document. Selection starts there.
If it returns `PreviewFallback`, later matching preview extractors may still
provide the response.

If no extractor succeeds, `ErrNoExtractor` is returned.

## The Extractor interface

A custom extractor can depend on the focused
[`server/extractor/sdk`](https://github.com/asciimoo/hister/tree/main/server/extractor/sdk)
package instead of importing Hister's configuration, document, and result
packages separately. It must implement this interface:

```go
type Extractor interface {
    // Name returns a human-readable identifier used in logs and config.
    Name() string

    // Description returns a short human-readable summary for clients.
    Description() string

    // Capabilities declares the extractor phases this implementation joins.
    Capabilities() sdk.Capabilities

    // Match reports whether this extractor applies to the given document.
    // Extract and Preview are only called when Match returns true.
    Match(*sdk.Document) bool

    // Extract rewrites the document before it is added to the index.
    // Return an explicit success, fallback, or abort result.
    Extract(*sdk.Document) sdk.ExtractResult

    // Preview returns a rendered representation suitable for display.
    // Return an explicit success, fallback, or abort result.
    Preview(*sdk.Document) sdk.PreviewResult

    // GetConfig returns the extractor's current configuration.
    // Must return sensible defaults before SetConfig is called.
    GetConfig() *sdk.Config

    // SetConfig applies user-supplied configuration on top of defaults.
    // Return an error for any unrecognised option key.
    SetConfig(*sdk.Config) error
}
```

### `Capabilities`

Capabilities keep metadata enrichment, searchable content extraction, and
preview rendering independent:

```go
type Capabilities struct {
    Enrich  bool
    Extract bool
    Preview bool
}
```

An enricher annotates every matching document and does not compete to select
the searchable body. A content extractor can populate the title and text. A
preview extractor can render the stored document. Most specialist extractors
declare both content and preview capabilities.

### Result types

`sdk.ExtractResult` and `sdk.PreviewResult` describe the outcome of each phase.
Construct them with these helpers:

```go
return sdk.Extracted()
return sdk.ExtractFallback(err)
return sdk.AbortExtraction(err)

return sdk.Previewed(sdk.PreviewResponse{Content: html})
return sdk.PreviewFallback(err)
return sdk.AbortPreview(err)
```

Fallback errors are optional diagnostics. Abort helpers always carry an error.
Passing `nil` to an abort helper produces a descriptive error rather than an
ambiguous result.

### `Document`

`sdk.Document` aliases the complete
[`document.Document`](https://github.com/asciimoo/hister/blob/main/server/document/document.go)
type. It is passed to `Match`, `Extract`, and `Preview`.

### `PreviewResponse`

`sdk.PreviewResponse` carries the output of `Preview`:

```go
type PreviewResponse struct {
    Content  string // HTML or plain text to render
    Template string // optional custom front-end template name; leave blank for default
}
```

### Registering a new extractor

Built in extractors are constructed by `DefaultExtractors` in
[`server/extractor/registry.go`](https://github.com/asciimoo/hister/blob/main/server/extractor/registry.go).
Add an instance there before the generic fallbacks so it takes priority for
the pages it targets.

Applications can also create an isolated chain or extend a fresh default one:

```go
registry := extractor.NewDefaultRegistry()
if err := registry.RegisterBefore("Readability", &MyExtractor{}); err != nil {
    return err
}
if err := registry.Init(configuredExtractors); err != nil {
    return err
}
```

The package level extraction functions remain available and use
`extractor.DefaultRegistry()`.

## Writing a new extractor

A ready-to-use starting point lives at
[`server/extractor/extractors/_extractor_template/extractor.go`](https://github.com/asciimoo/hister/blob/main/server/extractor/extractors/_extractor_template/extractor.go).
The directory begins with `_` so the Go toolchain ignores it during normal
builds, but the file itself is valid, fully-commented Go.

### Quick-start steps

1. Copy `server/extractor/extractors/_extractor_template/` to
   `server/extractor/extractors/<myname>/` (remove the leading `_`).
2. Change the `package` declaration to match the new directory name.
3. Rename `TemplateExtractor` to something descriptive (e.g. `HackerNewsExtractor`).
4. Update `matchURLPrefix` and the `Match` function for your target site.
5. Update `Capabilities` to declare the phases the extractor supports.
6. Implement `Extract` to populate `d.Title`, `d.Text`, and optionally `d.Metadata`.
7. Implement `Preview` to return sanitized HTML (or return `PreviewFallback`
   to reuse the generic readability preview).
8. Add an import and a `&MyExtractor{}` entry to `DefaultExtractors` in
   `server/extractor/registry.go`, before the readability extractor.

## Configuration

Each extractor can be enabled or disabled, and may expose custom options,
through the `extractors` section of the config file.

```yaml
extractors:
  <extractor-name>:
    enable: true | false
    options:
      key: value
```

The `<extractor-name>` key is the lowercased value returned by the extractor's
`Name()` method.

Only entries you want to change from the default need to be specified. If an
extractor is omitted from the config, its built-in defaults apply.

### `enable`

Controls whether the extractor participates in the chain.

| Value   | Effect                                                            |
| ------- | ----------------------------------------------------------------- |
| `true`  | Extractor is active. This is the default except for `ytdlp`.      |
| `false` | Extractor is skipped for automatic indexing and preview selection |

### `options`

A free-form map of extractor-specific settings. The available keys depend on
the extractor implementation; each extractor validates its `options` in
`SetConfig` and returns an error for any unrecognised key.

### Using `ConfigSupport`

Embed `sdk.ConfigSupport` to provide `GetConfig` and `SetConfig`. Its zero value
enables the extractor and rejects every option key:

```go
type MyExtractor struct {
    sdk.ConfigSupport
}
```

Use `sdk.NewConfigSupport` when an extractor has custom defaults. Keys present in
the default options map are accepted automatically:

```go
func NewMyExtractor() *MyExtractor {
    return &MyExtractor{
        ConfigSupport: sdk.NewConfigSupport(sdk.Config{
            Enable: true,
            Options: map[string]any{
                "timeout": 10,
            },
        }),
    }
}
```

Additional accepted keys without defaults can be passed after the default
configuration. An extractor can implement its own configuration methods when
applying configuration requires extra work.

Config merging (default → user-supplied) is performed automatically by
`Registry.Init` before `SetConfig` is called, so `SetConfig` always receives
the fully resolved configuration.

## Built-in extractors

The extractors below are registered in the order listed. Every matching
enricher runs first. The first matching content extractor that returns
`Extracted()` selects the searchable body. Preview selection follows the
same registration order but includes only preview capable extractors.

### `markdown`

Provides sanitized HTML previews for locally indexed Markdown files. The file
indexer renders `.md` and `.markdown` source into HTML before the extractor
chain runs, so this extractor leaves indexed text unchanged and handles the
preview.

**Matches:** `file://` URLs ending in `.md` or `.markdown`.

### `orgmode`

Provides sanitized HTML previews for locally indexed Org mode files. The file
indexer renders `.org` source into HTML before the extractor chain runs, so this
extractor leaves indexed text unchanged and handles the preview.

**Matches:** `file://` URLs ending in `.org`.

### `embeddedvideo`

Scans `iframe`, `video`, `embed`, and `object` elements for embedded video URLs.
Discovered entries are stored as JSON in the document's `videos` metadata. It
returns `Extracted()` after enriching the document. A later content extractor
still produces the searchable body and preview.

**Matches:** pages whose raw HTML contains a supported embedding element.

### `jsonld`

Parses every `<script type="application/ld+json">` block in the page and writes
normalised [schema.org](https://schema.org) metadata to `d.Metadata`. Captures
the `@type` (content classification) and `headline` fields that the Readability
extractor does not expose.

It returns `Extracted()` after enriching metadata but never produces body text
on its own. The `Readability` or `Basic` extractor further down the chain
handles text extraction.

**Matches:** any page that contains the `application/ld+json` substring.

### `reddit`

Extracts one Reddit post together with every comment already present in the
page. Indexed text and the preview preserve reply depth. Current Reddit pages
are read through semantic custom elements and content slots. Legacy pages are
read through Reddit thing identifiers, with schema.org discussion data as a
fallback.

The extractor does not request or expand hidden comments. Subreddit listings,
profiles, search results, and every other Reddit page are excluded.

**Matches:** Reddit post paths containing `/comments/{post_id}` and short post
links on `redd.it`.

### `discourse`

Extracts one Discourse topic together with every regular post already present
in the page. It combines rendered `article[data-post-id]` content with the
embedded topic stream, which preserves post numbers, authors, timestamps,
reply targets, likes, reactions, and accepted solutions. Schema.org `QAPage`
and crawler markup provide stable fallbacks.

The extractor performs no additional requests. Hidden posts and system action
entries are excluded. Category lists, tag lists, profiles, search pages, and
topic API formats are rejected.

**Matches:** topic paths such as `/t/{slug}/{topic_id}` on pages carrying a
Discourse generator, setup, or environment marker.

### `stackexchange`

Extracts Stack Exchange network question pages, including Stack Overflow,
Server Fault, Super User, Ask Ubuntu, MathOverflow, Stack Apps, and
`*.stackexchange.com` communities. Indexed text includes the question body and
all answers, with accepted answers marked.

The preview pane shows the full question body followed by each answer separated
by a horizontal rule, with accepted answers marked.

**Matches:**

- [stackoverflow.com](https://stackoverflow.com)
- [serverfault.com](https://serverfault.com)
- [superuser.com](https://superuser.com)
- [askubuntu.com](https://askubuntu.com)
- [mathoverflow.net](https://mathoverflow.net)
- [stackapps.com](https://stackapps.com)
- [stackexchange.com](https://stackexchange.com)

### `godoc`

Provides a rich preview for Go package documentation. The preview pane renders
the `Documentation-content` section of the page with relative links rewritten to
absolute `pkg.go.dev` URLs. Text extraction falls through to the `Readability`
extractor.

**Matches:** `https://pkg.go.dev/…`

### `github`

Extracts searchable content and previews from GitHub repository roots, issue
pages, issue lists, and pull request pages. Repository results include the
description, star count, topics, programming languages, and README. Issue and
pull request results include their page specific metadata and discussion
content.

**Matches:** `https://github.com/{owner}/{repo}`, its `/issues` list, individual
`/issues/{number}` pages, and individual `/pull/{number}` pages. GitHub system
paths such as `/settings`, `/topics`, and `/explore` are excluded.

### `lobsters`

Extracts the full content of a lobste.rs submission, including the story
metadata (title, author, tags, submission date), the optional story body, and
the complete nested comment tree. Both indexed text and preview preserve the
parent–child comment hierarchy.

**Matches:** `https://lobste.rs/s/…`

### `wikipedia`

Extracts article content from Wikipedia. Indexed text includes the article
title, infobox key–value pairs, and the body text with navigation boxes,
references, and other noise removed. The preview renders the article HTML with
inline styles applied, videos replaced by their poster frames, and relative URLs
rewritten to absolute Wikipedia URLs.

**Matches:** `https://*.wikipedia.org/wiki/…` (article pages only; non-content
namespaces such as `Special:`, `Talk:`, `User:`, `File:`, and `Category:` are
excluded).

### `mastodon`

Handles Mastodon instance pages by splitting them into individual toot documents.
Each toot found on the page is indexed as a separate document with its own URL
and author, allowing individual posts to appear in search results. The original
aggregator page is not indexed. Links for remote toots are rewritten to point
directly to the account's original server instead of the instance displaying
the federated copy.

Detection is heuristic: the extractor checks for a `"repository":"mastodon/mastodon"`
marker in the page HTML, or for a `type: toot` metadata flag set by a previous
pass.

**Matches:** any Mastodon instance page containing the Mastodon source marker.

### `bluesky`

Handles Bluesky profiles, custom feeds, and post threads by splitting every
available post into a separate document. Each document receives its canonical
`bsky.app` URL, author, handle, publication time, and a
`type: bluesky` metadata marker. The source page itself is not indexed.

The extractor prefers schema.org `DiscussionForumPosting` and `Comment` data
published by Bluesky. This supports profile and individual post pages even
without browser rendering. It also reads rendered feed and thread cards through
semantic test identifiers, roles, post links, and content attributes. Generated
CSS class names are not used. Rich rendered content is preferred when both
representations describe the same post, while canonical URL deduplication keeps
one document per post.

Custom feeds require the `chromedp` or `bidi` crawler backend because their post
lists are rendered in the browser. Profile and individual post pages also work
with the `http` backend through their JSON LD data.

**Matches:** profile, feed, and post pages on `bsky.app`, `www.bsky.app`, and
`embed.bsky.app`.

### `twitter`

Handles Twitter and X feeds, profiles, and tweet pages by splitting every
visible tweet into a separate document. Each tweet receives its canonical X
URL, author, handle, publication time, and a `type: tweet` metadata marker. The
feed or conversation page itself is not indexed.

The extractor first reads schema.org `SocialMediaPosting` metadata. It also
supports rendered tweet elements and Open Graph metadata from individual tweet
pages. This allows it to handle both server rendered pages and pages captured
after the application has loaded in a browser.

When a rendered tweet body is available, it is preferred over semantic metadata
so original link destinations produced by the application are preserved.
Expanded link attributes on anchors inside the tweet body replace `t.co`
aliases in indexed text. When an expanded attribute is absent, a complete URL
rendered as the anchor text is used instead. Truncated visible URLs are not
guessed. Preview anchors point directly to the original destination. A `t.co`
URL with no available destination remains plain text rather than becoming a
clickable redirect link.

**Matches:** feeds, profiles, and tweet pages on `twitter.com` and `x.com`,
including their common web and mobile hostnames.

### `notion`

Extracts the title and rendered block content of Notion pages and produces a
sanitized preview. Notion renders content in the browser, so indexing requires
the `chromedp` or `bidi` crawler backend. The extractor aborts when the rendered
block tree is missing so that Hister does not index the empty application shell.

**Matches:** nonroot pages on `notion.so`, `www.notion.so`, and any
`*.notion.site` domain.

### `ytdlp`

Extracts video metadata from video-hosting sites (YouTube, Vimeo, Twitch, etc.)
using the [`yt-dlp`](https://github.com/yt-dlp/yt-dlp) command-line tool.
Provides a dedicated video preview template that shows the thumbnail, duration,
uploader, description, chapter list, and optional transcript.

The extractor is **disabled by default** because it requires `yt-dlp` to be
installed separately.

**Matches:** a curated list of video-hosting domains (YouTube, Vimeo, Twitch,
Dailymotion, Bilibili, and others), as well as any hostname containing common
video-platform substrings.

#### Options

<ConfigReference items={ytdlpOptions} />

#### Example configuration

```yaml
extractors:
  ytdlp:
    enable: true
    options:
      binary: /usr/local/bin/yt-dlp
      timeout: 30
      max_concurrent_jobs: 2
      fetch_subtitles: true
      sub_language: en
      cookies_from_browser: firefox
      extra_args:
        - --proxy
        - socks5://127.0.0.1:1080
```

### `readability`

Generic article extractor using the
[go-readability](https://codeberg.org/readeck/go-readability) library. Strips
navigation, ads, sidebars, and other boilerplate and returns the main article
content as clean plain text and HTML. Also extracts author, publication date,
description, site name, and canonical image from JSON-LD, OpenGraph, and meta
tags.

**Matches:** every page. Acts as the primary fallback for all content that no
specialist extractor handles.

### `basic`

Ultimate fallback. Walks the raw HTML token stream and collects all visible text
inside `<body>`, discarding `<script>`, `<style>`, and `<noscript>` elements.
Produces plain text with no further processing.

**Matches:** every page. Only reached when `Readability` fails or is disabled.

## Development guidelines

**Avoid additional HTTP requests.** Work with the HTML and metadata already
available in the `Document` struct wherever possible. Making extra requests
inside an extractor adds latency, increases network traffic, and can fail
silently in offline or restricted environments. More importantly, outbound
requests expose the user's IP address and browsing activity to external servers,
which is a privacy concern. Additional requests are not forbidden, but they
should only be made when there is no reasonable alternative.

**Avoid embedding third-party content.** Strip or discard remote images, videos,
iframes, and other externally hosted media before returning content from
`Extract` or `Preview` wherever possible. Embedding such content causes the
browser to contact third-party servers whenever a preview is opened, leaking
the user's IP address without their knowledge. Third-party content is not
forbidden, but it should be avoided unless it is essential to the extractor's
purpose. When multimedia must be surfaced, the preferred approach is to render
a placeholder button that the user can click to load the video, audio, or embed
on demand, so external contact only happens with explicit user intent.

**Use custom preview templates when they add value.** If the extracted content
has a well-defined structure (code documentation, Q&amp;A threads, recipes, and
so on), return a non-empty `Template` in `PreviewResponse` and build a
dedicated Svelte template for it. A tailored layout is almost always more
readable than the generic one.

## Testing against live websites

Live extractor cases are declared in
[`server/extractor/live_cases.yaml`](https://github.com/asciimoo/hister/blob/main/server/extractor/live_cases.yaml).
Each case identifies a public URL, crawler backend, extractor, and a set of
stable content or metadata expectations. Prefer semantic invariants such as the
page type, a durable title fragment, and minimum content length over exact page
snapshots.

Run every live case with:

```bash
./manage.sh run_extractor_tests
```

Pass a case name fragment to run a subset:

```bash
./manage.sh run_extractor_tests discourse
```

The runner fetches each page through the configured Hister crawler backend and
respects its robots rules. It checks the selected extractor directly, checks the
complete extractor chain unless `run_chain` is false, and checks previews when
preview expectations are present. Identical fetches are reused within a test
run.

The live suite is selected through the `live` Go build tag and is not part of a
normal `go test ./...` run. The manifest structure is still validated during
normal tests, so invalid fields and unknown extractor names fail without making
network requests. Validation also requires at least one positive live case for
every registered extractor.

When a live case fails, the fetched HTML, extracted documents, preview, and a
summary are saved under `/tmp/hister-live-extractors`. Set
`HISTER_LIVE_ARTIFACT_DIR` to use another directory. These artifacts make it
possible to distinguish a website structure change from a transient fetch
failure.
