// Package extractor provides HTML content extraction for documents.
package extractor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	stdhtml "html"
	"io"
	"maps"
	"net/url"
	"strings"
	"time"

	readability "codeberg.org/readeck/go-readability/v2"
	"github.com/rs/zerolog/log"
	"golang.org/x/net/html"

	"github.com/asciimoo/hister/config"
	"github.com/asciimoo/hister/server/document"
	"github.com/asciimoo/hister/server/extractor/extractors/bluesky"
	"github.com/asciimoo/hister/server/extractor/extractors/discourse"
	"github.com/asciimoo/hister/server/extractor/extractors/embeddedvideo"
	"github.com/asciimoo/hister/server/extractor/extractors/github"
	"github.com/asciimoo/hister/server/extractor/extractors/godoc"
	"github.com/asciimoo/hister/server/extractor/extractors/jsonld"
	"github.com/asciimoo/hister/server/extractor/extractors/lobsters"
	"github.com/asciimoo/hister/server/extractor/extractors/markdown"
	"github.com/asciimoo/hister/server/extractor/extractors/mastodon"
	"github.com/asciimoo/hister/server/extractor/extractors/notion"
	"github.com/asciimoo/hister/server/extractor/extractors/org"
	"github.com/asciimoo/hister/server/extractor/extractors/reddit"
	"github.com/asciimoo/hister/server/extractor/extractors/stackexchange"
	"github.com/asciimoo/hister/server/extractor/extractors/twitter"
	"github.com/asciimoo/hister/server/extractor/extractors/wikipedia"
	"github.com/asciimoo/hister/server/extractor/extractors/ytdlp"
	"github.com/asciimoo/hister/server/sanitizer"
	"github.com/asciimoo/hister/server/types"
)

// Extractor extracts content from a Document.
type Extractor interface {
	// Name returns a human-readable identifier for the extractor.
	Name() string

	// Description returns a short human-readable summary of what the extractor does.
	Description() string

	// Capabilities declares which extractor phases this implementation joins.
	Capabilities() types.ExtractorCapabilities

	// Match reports whether this extractor is applicable to the given document.
	// Extract and Preview will only be called when Match returns true.
	Match(*document.Document) bool

	// Extract rewrites documents before the documents are added to the index.
	// It returns an explicit success, fallback, or abort result.
	Extract(*document.Document) types.ExtractResult

	// Preview returns a rendered representation of the document suitable for
	// display (e.g. readable HTML or plain text).
	// It returns an explicit success, fallback, or abort result.
	Preview(*document.Document) types.PreviewResult

	// GetConfig returns the extractor's current configuration. Before
	// SetConfig is called, implementations must return their default config.
	GetConfig() *config.Extractor

	// SetConfig applies cfg to the extractor, overwriting defaults.
	// Implementations should return an error for unrecognised option keys.
	SetConfig(*config.Extractor) error
}

// ContextExtractor is implemented by extractors whose extraction work can be
// canceled. The chain uses ExtractContext instead of Extract when available.
type ContextExtractor interface {
	ExtractContext(context.Context, *document.Document) types.ExtractResult
}

// ContextPreviewer is implemented by extractors whose preview work can be
// canceled. The chain uses PreviewContext instead of Preview when available.
type ContextPreviewer interface {
	PreviewContext(context.Context, *document.Document) types.PreviewResult
}

func extractWithContext(ctx context.Context, e Extractor, d *document.Document) types.ExtractResult {
	if contextual, ok := e.(ContextExtractor); ok {
		return contextual.ExtractContext(ctx, d)
	}
	return e.Extract(d)
}

func previewWithContext(ctx context.Context, e Extractor, d *document.Document) types.PreviewResult {
	if contextual, ok := e.(ContextPreviewer); ok {
		return contextual.PreviewContext(ctx, d)
	}
	return e.Preview(d)
}

// ErrNoExtractor is returned when no extractor can handle the document.
var ErrNoExtractor = errors.New("no extractor found")

// ErrExtractorAbort is returned when an extractor aborts its current phase.
var ErrExtractorAbort = errors.New("extractor aborted")

// ErrInvalidExtractorResult is returned when an extractor returns the zero
// value or another result that was not created by a result constructor.
var ErrInvalidExtractorResult = errors.New("invalid extractor result")

// ExtractorInfo holds a summary of an extractor's identity and current state.
type ExtractorInfo struct {
	Name         string                      `json:"name"`
	Description  string                      `json:"description"`
	Enabled      bool                        `json:"enabled"`
	Capabilities types.ExtractorCapabilities `json:"capabilities"`
	Options      map[string]any              `json:"options,omitempty"`
}

// ListMatching returns an ExtractorInfo entry for every enabled extractor that
// matches d, in chain order. Options is omitted so the result is safe to send
// to clients.
func ListMatching(d *document.Document) []ExtractorInfo {
	infos := make([]ExtractorInfo, 0)
	for _, e := range extractors {
		if !e.GetConfig().Enable {
			continue
		}
		if e.Match(d) {
			infos = append(infos, ExtractorInfo{
				Name:         e.Name(),
				Description:  e.Description(),
				Enabled:      true,
				Capabilities: e.Capabilities(),
			})
		}
	}
	return infos
}

// ListMatchingPreview returns every enabled preview extractor matching d in
// chain order. Metadata-only enrichers and content-only extractors are omitted.
func ListMatchingPreview(d *document.Document) []ExtractorInfo {
	infos := make([]ExtractorInfo, 0)
	for _, e := range extractors {
		if !e.GetConfig().Enable || !e.Capabilities().Preview {
			continue
		}
		if e.Match(d) {
			infos = append(infos, ExtractorInfo{
				Name:         e.Name(),
				Description:  e.Description(),
				Enabled:      true,
				Capabilities: e.Capabilities(),
			})
		}
	}
	return infos
}

// ListEnabled returns an ExtractorInfo entry for every enabled extractor in
// chain order. Options is omitted so the result is safe to send to clients.
func ListEnabled() []ExtractorInfo {
	infos := make([]ExtractorInfo, 0, len(extractors))
	for _, e := range extractors {
		if !e.GetConfig().Enable {
			continue
		}
		infos = append(infos, ExtractorInfo{
			Name:         e.Name(),
			Description:  e.Description(),
			Enabled:      true,
			Capabilities: e.Capabilities(),
		})
	}
	return infos
}

// List returns an ExtractorInfo entry for every registered extractor in chain
// order. Options is always populated; callers that should not expose
// configuration must clear or omit it before sending to clients.
func List() []ExtractorInfo {
	infos := make([]ExtractorInfo, 0, len(extractors))
	for _, e := range extractors {
		cfg := e.GetConfig()
		infos = append(infos, ExtractorInfo{
			Name:         e.Name(),
			Description:  e.Description(),
			Enabled:      cfg.Enable,
			Capabilities: e.Capabilities(),
			Options:      cfg.Options,
		})
	}
	return infos
}

var extractors = []Extractor{
	&markdown.MarkdownExtractor{},
	&org.OrgModeExtractor{},
	&embeddedvideo.EmbeddedVideoExtractor{},
	&discourse.DiscourseExtractor{},
	&jsonld.JSONLDExtractor{},
	&reddit.RedditExtractor{},
	&stackexchange.StackExchangeExtractor{},
	&godoc.GoDocExtractor{},
	&github.GitHubExtractor{},
	&lobsters.LobstersExtractor{},
	&wikipedia.WikipediaExtractor{},
	&mastodon.MastodonExtractor{},
	&bluesky.BlueskyExtractor{},
	&twitter.TwitterExtractor{},
	&notion.NotionExtractor{},
	&ytdlp.YtdlpExtractor{},
	&readabilityExtractor{},
	&basicExtractor{},
}

// Init applies user-supplied extractor configurations on top of each
// extractor's defaults. It must be called before Extract or Preview.
// cfgs is keyed by lowercased extractor name (as Viper lowercases YAML keys).
func Init(cfgs map[string]*config.Extractor) error {
	for _, e := range extractors {
		def := e.GetConfig()
		merged := &config.Extractor{
			Enable:  def.Enable,
			Options: make(map[string]any, len(def.Options)),
		}
		maps.Copy(merged.Options, def.Options)
		if user, ok := cfgs[strings.ToLower(e.Name())]; ok && user != nil {
			merged.Enable = user.Enable
			maps.Copy(merged.Options, user.Options)
		}
		if err := e.SetConfig(merged); err != nil {
			return fmt.Errorf("extractor %s: %w", e.Name(), err)
		}
	}
	return nil
}

// Extract first runs every matching enricher, then tries matching content
// extractors in registration order and returns the first successful result.
// Returns ErrNoExtractor if no content extractor succeeds.
func Extract(d *document.Document) error {
	return ExtractContext(context.Background(), d)
}

// ExtractContext runs the extraction chain with caller cancellation.
func ExtractContext(ctx context.Context, d *document.Document) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	for _, e := range extractors {
		if !e.GetConfig().Enable || !e.Capabilities().Enrich {
			continue
		}
		if e.Match(d) {
			result := extractWithContext(ctx, e, d)
			if err := ctx.Err(); err != nil {
				return err
			}
			log.Debug().Str("URL", d.URL).Str("Extractor", e.Name()).Msg("Enriching document")
			switch result.Decision() {
			case types.ExtractorSuccess:
			case types.ExtractorFallback:
				if result.Err() != nil {
					log.Warn().Err(result.Err()).Str("URL", d.URL).Str("Extractor", e.Name()).Msg("Failed to enrich document")
				}
			case types.ExtractorAbort:
				return fmt.Errorf("extractor %s: %w: %w", e.Name(), ErrExtractorAbort, result.Err())
			default:
				return fmt.Errorf("extractor %s: %w", e.Name(), ErrInvalidExtractorResult)
			}
		}
	}
	for _, e := range extractors {
		if !e.GetConfig().Enable || !e.Capabilities().Extract {
			continue
		}
		if e.Match(d) {
			result := extractWithContext(ctx, e, d)
			if err := ctx.Err(); err != nil {
				return err
			}
			log.Debug().Str("URL", d.URL).Str("Extractor", e.Name()).Msg("Extracting data")
			switch result.Decision() {
			case types.ExtractorSuccess:
				return nil
			case types.ExtractorAbort:
				return fmt.Errorf("extractor %s: %w: %w", e.Name(), ErrExtractorAbort, result.Err())
			case types.ExtractorFallback:
				if result.Err() != nil {
					log.Warn().Err(result.Err()).Str("URL", d.URL).Str("Extractor", e.Name()).Msg("Failed to extract content")
				}
			default:
				return fmt.Errorf("extractor %s: %w", e.Name(), ErrInvalidExtractorResult)
			}
		}
	}
	return ErrNoExtractor
}

// Preview returns a rendered preview of the document. When name is empty, the
// first successful matching preview extractor is used. A name selects the
// starting extractor, with later matching preview extractors acting as
// fallbacks. Explicit selection still requires the extractor to be enabled,
// preview capable, and matched to the document.
func Preview(d *document.Document, name string) (types.PreviewResponse, error) {
	return PreviewContext(context.Background(), d, name)
}

// PreviewContext renders a preview with caller cancellation.
func PreviewContext(ctx context.Context, d *document.Document, name string) (types.PreviewResponse, error) {
	if err := ctx.Err(); err != nil {
		return types.PreviewResponse{}, err
	}
	start := 0
	if name != "" {
		lower := strings.ToLower(name)
		found := false
		for i, e := range extractors {
			if strings.ToLower(e.Name()) == lower {
				found = true
				if !e.GetConfig().Enable {
					return types.PreviewResponse{}, fmt.Errorf("%w: %s is disabled", ErrNoExtractor, name)
				}
				if !e.Capabilities().Preview {
					return types.PreviewResponse{}, fmt.Errorf("%w: %s does not provide previews", ErrNoExtractor, name)
				}
				if !e.Match(d) {
					return types.PreviewResponse{}, fmt.Errorf("%w: %s does not match document", ErrNoExtractor, name)
				}
				start = i
				break
			}
		}
		if !found {
			return types.PreviewResponse{}, fmt.Errorf("%w: %s", ErrNoExtractor, name)
		}
	}
	for _, e := range extractors[start:] {
		if !e.GetConfig().Enable || !e.Capabilities().Preview {
			continue
		}
		if e.Match(d) {
			log.Debug().Str("URL", d.URL).Str("Extractor", e.Name()).Msg("Creating preview")
			result := previewWithContext(ctx, e, d)
			if err := ctx.Err(); err != nil {
				return types.PreviewResponse{}, err
			}
			switch result.Decision() {
			case types.ExtractorSuccess:
				return result.Response(), nil
			case types.ExtractorAbort:
				return types.PreviewResponse{}, fmt.Errorf("extractor %s: %w: %w", e.Name(), ErrExtractorAbort, result.Err())
			case types.ExtractorFallback:
				if result.Err() != nil {
					log.Warn().Err(result.Err()).Str("URL", d.URL).Str("Extractor", e.Name()).Msg("Failed to preview content")
				}
			default:
				return types.PreviewResponse{}, fmt.Errorf("extractor %s: %w", e.Name(), ErrInvalidExtractorResult)
			}
		}
	}
	return types.PreviewResponse{}, ErrNoExtractor
}

type basicExtractor struct {
	cfg *config.Extractor
}

type readabilityExtractor struct {
	cfg *config.Extractor
}

func (e *basicExtractor) GetConfig() *config.Extractor {
	if e.cfg == nil {
		return &config.Extractor{Enable: true, Options: map[string]any{}}
	}
	return e.cfg
}

func (e *basicExtractor) SetConfig(c *config.Extractor) error {
	for k := range c.Options {
		return fmt.Errorf("unknown option %q", k)
	}
	e.cfg = c
	return nil
}

func (e *readabilityExtractor) GetConfig() *config.Extractor {
	if e.cfg == nil {
		return &config.Extractor{Enable: true, Options: map[string]any{}}
	}
	return e.cfg
}

func (e *readabilityExtractor) SetConfig(c *config.Extractor) error {
	for k := range c.Options {
		return fmt.Errorf("unknown option %q", k)
	}
	e.cfg = c
	return nil
}

func (e *basicExtractor) Name() string {
	return "Basic"
}

func (e *basicExtractor) Description() string {
	return "Fallback extractor that strips HTML tags and extracts plain text from any web page."
}

func (e *basicExtractor) Capabilities() types.ExtractorCapabilities {
	return types.ExtractorCapabilities{Extract: true, Preview: true}
}

func (e *basicExtractor) Match(_ *document.Document) bool {
	return true
}

func (e *basicExtractor) Extract(d *document.Document) types.ExtractResult {
	var extractedTitle strings.Builder
	r := strings.NewReader(d.HTML)
	doc := html.NewTokenizer(r)
	inBody := false
	skip := false
	var text strings.Builder
	var currentTag string
out:
	for {
		tt := doc.Next()
		switch tt {
		case html.ErrorToken:
			err := doc.Err()
			if errors.Is(err, io.EOF) {
				break out
			}
			return types.AbortExtraction(errors.New("failed to parse html: " + err.Error()))
		case html.SelfClosingTagToken, html.StartTagToken:
			tn, _ := doc.TagName()
			currentTag = string(tn)
			switch currentTag {
			case "body":
				inBody = true
			case "script", "style", "noscript":
				skip = true
			}
		case html.TextToken:
			if currentTag == "title" {
				extractedTitle.WriteString(strings.TrimSpace(string(doc.Text())))
			}
			if inBody && !skip {
				text.Write(doc.Text())
			}
		case html.EndTagToken:
			tn, _ := doc.TagName()
			switch string(tn) {
			case "body":
				inBody = false
			case "script", "style", "noscript":
				skip = false
			}
		}
	}
	d.Text = strings.TrimSpace(text.String())
	if extractedTitle.String() != "" {
		d.Title = extractedTitle.String()
	}
	if d.Text == "" && d.Title == "" {
		return types.ExtractFallback(errors.New("no content found"))
	}
	return types.Extracted()
}

func (e *basicExtractor) Preview(d *document.Document) types.PreviewResult {
	return types.Previewed(types.PreviewResponse{Content: stdhtml.EscapeString(d.Text)})
}

func (e *readabilityExtractor) Name() string {
	return "Readability"
}

func (e *readabilityExtractor) Description() string {
	return "Extracts the main article content from any web page using the go-readability library, filtering out navigation, ads, and other boilerplate."
}

func (e *readabilityExtractor) Capabilities() types.ExtractorCapabilities {
	return types.ExtractorCapabilities{Extract: true, Preview: true}
}

func (e *readabilityExtractor) Match(_ *document.Document) bool {
	return true
}

func (e *readabilityExtractor) Extract(d *document.Document) types.ExtractResult {
	r := strings.NewReader(d.HTML)

	u, err := url.Parse(d.URL)
	if err != nil {
		return types.AbortExtraction(err)
	}
	a, err := readability.FromReader(r, u)
	if err != nil {
		return types.ExtractFallback(err)
	}
	buf := bytes.NewBuffer(nil)
	if err := a.RenderText(buf); err != nil {
		return types.ExtractFallback(err)
	}
	d.Text = buf.String()
	if t := a.Title(); t != "" {
		d.Title = t
	}
	d.SetFaviconURL(a.Favicon())
	writeReadabilityMeta(d, a)
	return types.Extracted()
}

// writeReadabilityMeta copies the rich fields readability already parsed
// (internally from JSON-LD, OpenGraph, and meta tags) onto d.Metadata so
// downstream consumers have byline/date/description without re-parsing.
// The JSON-LD extractor only writes type and headline, so these keys do
// not collide.
func writeReadabilityMeta(d *document.Document, a readability.Article) {
	if d.Metadata == nil {
		d.Metadata = make(map[string]any)
	}
	set := func(k, v string) {
		if v != "" {
			d.Metadata[k] = v
		}
	}
	set("author", a.Byline())
	set("description", a.Excerpt())
	set("site_name", a.SiteName())
	set("image", a.ImageURL())
	set("language", a.Language())
	if t, err := a.PublishedTime(); err == nil && !t.IsZero() {
		d.Metadata["published"] = t.Format(time.RFC3339)
	}
	if t, err := a.ModifiedTime(); err == nil && !t.IsZero() {
		d.Metadata["modified"] = t.Format(time.RFC3339)
	}
}

func (e *readabilityExtractor) Preview(d *document.Document) types.PreviewResult {
	r := strings.NewReader(d.HTML)
	u, err := url.Parse(d.URL)
	if err != nil {
		return types.AbortPreview(err)
	}
	a, err := readability.FromReader(r, u)
	if err != nil {
		return types.PreviewFallback(err)
	}
	var htmlContent strings.Builder
	if err := a.RenderHTML(&htmlContent); err != nil {
		return types.PreviewFallback(err)
	}
	return types.Previewed(types.PreviewResponse{Content: sanitizer.SanitizeHTML(htmlContent.String())})
}
