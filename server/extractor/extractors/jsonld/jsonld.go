// Package jsonld provides an extractor that parses application/ld+json
// script tags and writes normalized metadata to the document.
package jsonld

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/rs/zerolog/log"
	"golang.org/x/net/html"

	"github.com/asciimoo/hister/config"
	"github.com/asciimoo/hister/server/document"
	"github.com/asciimoo/hister/server/types"
)

// JSONLDExtractor extracts schema.org metadata from JSON-LD script tags.
type JSONLDExtractor struct {
	cfg *config.Extractor
}

// Name returns the extractor's identifier.
func (e *JSONLDExtractor) Name() string {
	return "JSONLD"
}

// Description returns a short summary of what this extractor does.
func (e *JSONLDExtractor) Description() string {
	return "Parses application/ld+json script tags and stores normalized schema.org metadata on the document."
}

// GetConfig returns the extractor's current configuration.
func (e *JSONLDExtractor) GetConfig() *config.Extractor {
	if e.cfg == nil {
		return &config.Extractor{Enable: true, Options: map[string]any{}}
	}
	return e.cfg
}

// SetConfig applies cfg to the extractor. Returns an error for unknown options.
func (e *JSONLDExtractor) SetConfig(c *config.Extractor) error {
	for k := range c.Options {
		return fmt.Errorf("unknown option %q", k)
	}
	e.cfg = c
	return nil
}

// Match returns true when the document has non-empty HTML.
func (e *JSONLDExtractor) Match(d *document.Document) bool {
	return len(d.HTML) > 0
}

// preferredTypes lists @type values that make a node a good source for
// normalized metadata, in descending priority order.
var preferredTypes = []string{
	"Article", "NewsArticle", "BlogPosting",
	"WebPage", "Product", "Recipe",
	"VideoObject", "Event", "Person", "Organization",
}

// Extract parses every application/ld+json script block, flattens @graph/array
// wrappers and writes normalized fields to d.Metadata. Always returns Continue
// so body-text extractors still run.
func (e *JSONLDExtractor) Extract(d *document.Document) (types.ExtractorState, error) {
	blobs := findJSONLDBlobs(d.HTML)
	if len(blobs) == 0 {
		return types.ExtractorContinue, nil
	}

	var nodes []map[string]any
	for _, blob := range blobs {
		var parsed any
		if err := json.Unmarshal([]byte(blob), &parsed); err != nil {
			log.Debug().Err(err).Str("URL", d.URL).Msg("Failed to parse JSON-LD blob")
			continue
		}
		nodes = append(nodes, flatten(parsed)...)
	}
	if len(nodes) == 0 {
		return types.ExtractorContinue, nil
	}

	if d.Metadata == nil {
		d.Metadata = make(map[string]any)
	}
	d.Metadata["jsonld"] = nodes

	best := pickBest(nodes)
	setString(d.Metadata, "jsonld_type", typeString(best))
	setString(d.Metadata, "jsonld_headline", firstString(best, "headline", "name"))
	setString(d.Metadata, "jsonld_description", firstString(best, "description"))
	setString(d.Metadata, "jsonld_author", authorName(best["author"]))
	setString(d.Metadata, "jsonld_published", firstString(best, "datePublished"))
	setString(d.Metadata, "jsonld_modified", firstString(best, "dateModified"))
	setString(d.Metadata, "jsonld_image", imageURL(best["image"]))

	return types.ExtractorContinue, nil
}

// Preview is not implemented; Readability/Default handle rendering.
func (e *JSONLDExtractor) Preview(d *document.Document) (types.PreviewResponse, types.ExtractorState, error) {
	return types.PreviewResponse{}, types.ExtractorContinue, nil
}

// findJSONLDBlobs returns the raw text content of every
// <script type="application/ld+json"> element in rawHTML.
func findJSONLDBlobs(rawHTML string) []string {
	z := html.NewTokenizer(strings.NewReader(rawHTML))
	var blobs []string
	var buf bytes.Buffer
	capturing := false
	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			if errors.Is(z.Err(), io.EOF) {
				return blobs
			}
			return blobs
		case html.StartTagToken:
			tn, hasAttr := z.TagName()
			if string(tn) != "script" || !hasAttr {
				continue
			}
			if isJSONLDScript(z) {
				capturing = true
				buf.Reset()
			}
		case html.TextToken:
			if capturing {
				buf.Write(z.Text())
			}
		case html.EndTagToken:
			if !capturing {
				continue
			}
			tn, _ := z.TagName()
			if string(tn) == "script" {
				s := strings.TrimSpace(buf.String())
				if s != "" {
					blobs = append(blobs, s)
				}
				capturing = false
				buf.Reset()
			}
		}
	}
}

func isJSONLDScript(z *html.Tokenizer) bool {
	for {
		key, val, more := z.TagAttr()
		if string(key) == "type" && strings.EqualFold(strings.TrimSpace(string(val)), "application/ld+json") {
			return true
		}
		if !more {
			return false
		}
	}
}

// flatten turns an arbitrary JSON-LD payload into a flat slice of object
// nodes, unwrapping @graph wrappers and top-level arrays.
func flatten(v any) []map[string]any {
	switch t := v.(type) {
	case map[string]any:
		if g, ok := t["@graph"]; ok {
			return flatten(g)
		}
		return []map[string]any{t}
	case []any:
		var out []map[string]any
		for _, item := range t {
			out = append(out, flatten(item)...)
		}
		return out
	}
	return nil
}

// pickBest returns the first node whose @type matches one of preferredTypes,
// falling back to the first node.
func pickBest(nodes []map[string]any) map[string]any {
	for _, want := range preferredTypes {
		for _, n := range nodes {
			if typeMatches(n["@type"], want) {
				return n
			}
		}
	}
	return nodes[0]
}

func typeMatches(v any, want string) bool {
	switch t := v.(type) {
	case string:
		return t == want
	case []any:
		for _, item := range t {
			if s, ok := item.(string); ok && s == want {
				return true
			}
		}
	}
	return false
}

func typeString(n map[string]any) string {
	switch t := n["@type"].(type) {
	case string:
		return t
	case []any:
		for _, item := range t {
			if s, ok := item.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}

func firstString(n map[string]any, keys ...string) string {
	for _, k := range keys {
		if s, ok := n[k].(string); ok {
			if s = strings.TrimSpace(s); s != "" {
				return s
			}
		}
	}
	return ""
}

// authorName extracts a human-readable author name from string, object, or
// array shapes.
func authorName(v any) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case map[string]any:
		if s, ok := t["name"].(string); ok {
			return strings.TrimSpace(s)
		}
	case []any:
		for _, item := range t {
			if s := authorName(item); s != "" {
				return s
			}
		}
	}
	return ""
}

// imageURL extracts an image URL from string, object (schema.org ImageObject),
// or array shapes.
func imageURL(v any) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case map[string]any:
		if s, ok := t["url"].(string); ok {
			return strings.TrimSpace(s)
		}
	case []any:
		for _, item := range t {
			if s := imageURL(item); s != "" {
				return s
			}
		}
	}
	return ""
}

func setString(m map[string]any, key, value string) {
	if value == "" {
		return
	}
	m[key] = value
}
