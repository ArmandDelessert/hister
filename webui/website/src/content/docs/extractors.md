---
date: '2026-04-07T11:00:00+00:00'
draft: false
title: 'Extractors'
---

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

Extractors are tried in registration order. Each call to `Extract` or `Preview`
returns an `ExtractorState` value that signals how the chain should proceed:

| State               | Meaning                                                                                                                     |
| ------------------- | --------------------------------------------------------------------------------------------------------------------------- |
| `ExtractorStop`     | The extractor handled the document successfully; stop the chain and return a successful result.                             |
| `ExtractorContinue` | The extractor was inconclusive; try the next matching extractor in the chain.                                               |
| `ExtractorAbort`    | A fatal error occurred; stop the chain immediately and propagate the error to the caller without trying further extractors. |

If no extractor returns `ExtractorStop`, `ErrNoExtractor` is returned.

## The Extractor interface

A custom extractor must implement the following Go interface (defined in
[`server/extractor/extractor.go`](https://github.com/asciimoo/hister/blob/main/server/extractor/extractor.go)):

```go
type Extractor interface {
    // Name returns a human-readable identifier used in logs and config.
    Name() string

    // Match reports whether this extractor applies to the given document.
    // Extract and Preview are only called when Match returns true.
    Match(*document.Document) bool

    // Extract rewrites the document before it is added to the index.
    // Return ExtractorStop on success, ExtractorContinue to fall through to
    // the next extractor, or ExtractorAbort to stop with a fatal error.
    Extract(*document.Document) (types.ExtractorState, error)

    // Preview returns a rendered representation suitable for display.
    // Return ExtractorStop on success, ExtractorContinue to fall through to
    // the next extractor, or ExtractorAbort to stop with a fatal error.
    Preview(*document.Document) (types.PreviewResponse, types.ExtractorState, error)

    // GetConfig returns the extractor's current configuration.
    // Must return sensible defaults before SetConfig is called.
    GetConfig() *config.Extractor

    // SetConfig applies user-supplied configuration on top of defaults.
    // Return an error for any unrecognised option key.
    SetConfig(*config.Extractor) error
}
```

### `ExtractorState`

[`types.ExtractorState`](https://github.com/asciimoo/hister/blob/main/server/types/types.go)
is defined in the `server/types` package:

```go
type ExtractorState int

const (
    ExtractorStop     ExtractorState = iota // success, stop the chain
    ExtractorContinue                       // inconclusive, try next extractor
    ExtractorAbort                          // fatal error, stop immediately
)
```

### `Document`

The whole [`document.Document`](https://github.com/asciimoo/hister/blob/main/server/document/document.go)
struct passed to `Match`, `Extract`, and `Preview`.

### `PreviewResponse`

[`types.PreviewResponse`](https://github.com/asciimoo/hister/blob/main/server/types/types.go)
carries the output of `Preview`:

```go
type PreviewResponse struct {
    Content  string // HTML or plain text to render
    Template string // optional custom front-end template name; leave blank for default
}
```

### Registering a new extractor

Add an instance of your extractor to the `extractors` slice in
[`server/extractor/extractor.go`](https://github.com/asciimoo/hister/blob/main/server/extractor/extractor.go).
Place it **before** the generic fallbacks so that it takes priority for the
pages it targets.

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
5. Implement `Extract` to populate `d.Title`, `d.Text`, and optionally `d.Metadata`.
6. Implement `Preview` to return sanitized HTML (or return `ExtractorContinue`
   to reuse the generic readability preview).
7. Add an import and a `&MyExtractor{}` entry to the `extractors` slice in
   `server/extractor/extractor.go`, before the `&readabilityExtractor{}` line.

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

| Value   | Effect                                              |
| ------- | --------------------------------------------------- |
| `true`  | Extractor is active (the default for all built-ins) |
| `false` | Extractor is skipped for both indexing and preview  |

### `options`

A free-form map of extractor-specific settings. The available keys depend on
the extractor implementation; each extractor validates its `options` in
`SetConfig` and returns an error for any unrecognised key.

### Implementing `GetConfig` and `SetConfig`

`GetConfig` must return the extractor's current configuration (or a default
when no config has been applied yet):

```go
func (e *MyExtractor) GetConfig() *config.Extractor {
    if e.cfg == nil {
        return &config.Extractor{
            Enable:  true,
            Options: map[string]any{},
        }
    }
    return e.cfg
}
```

`SetConfig` should validate that no unknown option keys are present, then store
the config:

```go
func (e *MyExtractor) SetConfig(c *config.Extractor) error {
    allowed := map[string]bool{"timeout": true}
    for k := range c.Options {
        if !allowed[k] {
            return fmt.Errorf("unknown option %q", k)
        }
    }
    e.cfg = c
    return nil
}
```

Config merging (default → user-supplied) is performed automatically by
`extractor.Init` before `SetConfig` is called, so `SetConfig` always receives
the fully resolved configuration.

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
