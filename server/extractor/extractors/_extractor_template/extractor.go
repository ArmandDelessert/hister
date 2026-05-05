// SPDX-License-Identifier: AGPL-3.0-or-later

// Package extractor_template is a starting point for implementing a new Hister
// extractor. To use it:
//
//  1. Copy this directory to server/extractor/extractors/<myname>/.
//  2. Rename the directory (remove the leading "_" so Go picks it up).
//  3. Change the package declaration below to match the new directory name.
//  4. Rename TemplateExtractor to something descriptive.
//  5. Update matchURLPrefix (and the Match function) for your target site.
//  6. Implement Extract and Preview.
//  7. Register the extractor in server/extractor/extractor.go.
//
// The directory name starts with "_" so the Go toolchain ignores it during
// normal builds. That means this file is never compiled as-is, but it is valid
// Go so editors and linters can still analyse it.
package extractor_template

import (
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"github.com/asciimoo/hister/config"
	"github.com/asciimoo/hister/server/document"
	"github.com/asciimoo/hister/server/sanitizer"
	"github.com/asciimoo/hister/server/types"
)

// matchURLPrefix is the URL prefix this extractor targets.
// Adjust to match your site, e.g. "https://example.com/articles/".
const matchURLPrefix = "https://example.com/"

// TemplateExtractor extracts content from example.com pages.
// Rename this type to reflect the site or content type you are targeting.
type TemplateExtractor struct {
	cfg *config.Extractor
}

// Name returns a short human-readable identifier used in log messages and as
// the YAML config key (lowercased). "MyExtractor" → yaml key "myextractor".
func (e *TemplateExtractor) Name() string {
	return "Template"
}

// Description returns a short summary of what this extractor does.
// It is surfaced by the /api/config endpoint.
func (e *TemplateExtractor) Description() string {
	return "Template extractor. Replace this with a description of what your extractor does."
}

// GetConfig returns the extractor's current configuration, or built-in
// defaults when SetConfig has not been called yet.
//
// Set Enable to false if the extractor requires external tools or credentials
// that are not present by default (e.g. a binary, an API key). Users must
// then explicitly opt-in via the config file.
//
// Declare every supported option key with its default value in Options.
// extractor.Init merges defaults with any user-supplied values before calling
// SetConfig, so SetConfig always receives the fully resolved map.
func (e *TemplateExtractor) GetConfig() *config.Extractor {
	if e.cfg == nil {
		return &config.Extractor{
			Enable:  true,
			Options: map[string]any{
				// Declare extractor-specific options and their defaults here.
				// Example:
				//   "max_items": 20,
				//   "include_comments": false,
			},
		}
	}
	return e.cfg
}

// SetConfig applies cfg to the extractor.
// Reject any unrecognised option key with a descriptive error so the user
// gets immediate feedback about typos in the config file.
func (e *TemplateExtractor) SetConfig(c *config.Extractor) error {
	for k := range c.Options {
		switch k {
		// List accepted option keys here. Example:
		//   case "max_items", "include_comments":
		default:
			return fmt.Errorf("unknown option %q", k)
		}
	}
	e.cfg = c
	return nil
}

// Match reports whether this extractor should handle the given document.
// It is called for every document before Extract or Preview; keep it fast.
//
// Common patterns:
//
//	URL prefix:  strings.HasPrefix(d.URL, matchURLPrefix)
//	Exact domain: d.Domain == "example.com"
//	Path suffix:  strings.HasSuffix(d.URL, ".rss")
func (e *TemplateExtractor) Match(d *document.Document) bool {
	return strings.HasPrefix(d.URL, matchURLPrefix) && len(d.URL) > len(matchURLPrefix)
}

// Extract rewrites the document before it is added to the search index.
// Populate d.Title and d.Text with the best searchable representation of the
// content. Both fields are stored in Bleve and searched by default.
//
// Return values:
//   - (ExtractorStop,     nil)  success; stop the chain.
//   - (ExtractorContinue, _)    inconclusive; let the next extractor try.
//   - (ExtractorAbort,    err)  fatal; stop the chain and propagate the error.
//
// Optional features:
//   - Set d.Metadata["author"], d.Metadata["published"], etc. to surface
//     structured fields in the preview panel header.
//   - Call d.SetFaviconURL(url) to override the default favicon discovery.
//   - Append to d.ExtraDocuments to queue additional URLs for indexing
//     (e.g. paginated content, linked sub-pages).
//   - Set d.SkipIndexing = true when the current document should not be
//     indexed itself and only its ExtraDocuments matter.
func (e *TemplateExtractor) Extract(d *document.Document) (types.ExtractorState, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(d.HTML))
	if err != nil {
		// Return Continue so the next extractor can try; use Abort only when
		// the error makes any further processing pointless.
		return types.ExtractorContinue, err
	}

	// --- Title -----------------------------------------------------------
	d.Title = strings.TrimSpace(doc.Find("h1").First().Text())

	// --- Body text -------------------------------------------------------
	var b strings.Builder
	doc.Find("article").Each(func(_ int, s *goquery.Selection) {
		if text := strings.TrimSpace(s.Text()); text != "" {
			b.WriteString(text)
			b.WriteString("\n\n")
		}
	})
	d.Text = strings.TrimSpace(b.String())

	if d.Title == "" && d.Text == "" {
		return types.ExtractorContinue, fmt.Errorf("no content found")
	}

	// --- Metadata (optional) --------------------------------------------
	// Fields written here appear in the preview panel header. Recognised keys:
	// "author", "published", "modified", "description", "image", "site_name".
	if d.Metadata == nil {
		d.Metadata = make(map[string]any)
	}
	if author := strings.TrimSpace(doc.Find(".author").First().Text()); author != "" {
		d.Metadata["author"] = author
	}

	// --- Extra documents (optional) -------------------------------------
	// Queue related URLs for indexing. Each extra document is processed and
	// indexed independently after the current document.
	//
	//   doc.Find("a.related-link").Each(func(_ int, s *goquery.Selection) {
	//       if href, ok := s.Attr("href"); ok {
	//           d.ExtraDocuments = append(d.ExtraDocuments, &document.Document{
	//               URL:    resolveAbsoluteURL(d.URL, href),
	//               UserID: d.UserID,
	//           })
	//       }
	//   })

	return types.ExtractorStop, nil
}

// Preview returns a rendered representation of the document for the preview
// panel. Content is typically sanitized HTML. The optional Template field
// selects a custom Svelte front-end template; leave it empty for the default.
//
// Return values follow the same ExtractorState convention as Extract.
//
// If you do not need a custom preview, return (PreviewResponse{}, ExtractorContinue, nil)
// and the generic readability/default extractor will provide one automatically.
func (e *TemplateExtractor) Preview(d *document.Document) (types.PreviewResponse, types.ExtractorState, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(d.HTML))
	if err != nil {
		return types.PreviewResponse{}, types.ExtractorContinue, err
	}

	var b strings.Builder

	// --- Title -----------------------------------------------------------
	if title := strings.TrimSpace(doc.Find("h1").First().Text()); title != "" {
		fmt.Fprintf(&b, "<h2>%s</h2>\n", title)
	}

	// --- Body ------------------------------------------------------------
	if body, err := doc.Find("article").First().Html(); err == nil && strings.TrimSpace(body) != "" {
		b.WriteString(body)
	}

	if b.Len() == 0 {
		return types.PreviewResponse{}, types.ExtractorContinue, fmt.Errorf("no preview content")
	}

	// Always sanitize HTML before returning it to strip scripts, event
	// handlers, and other potentially unsafe markup.
	return types.PreviewResponse{
		Content: sanitizer.SanitizeHTML(b.String()),
		// Template: "my_template", // leave empty to use the default layout
	}, types.ExtractorStop, nil
}
