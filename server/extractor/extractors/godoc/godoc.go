// Package godoc provides an extractor for pkg.go.dev documentation pages.
package godoc

import (
	"bytes"
	"errors"
	"fmt"
	stdhtml "html"
	"io"
	"net/url"
	"slices"
	"strings"

	"golang.org/x/net/html"

	"github.com/asciimoo/hister/config"
	"github.com/asciimoo/hister/server/document"
	"github.com/asciimoo/hister/server/sanitizer"
	"github.com/asciimoo/hister/server/types"
)

const pkgGoDevPrefix = "https://pkg.go.dev/"

// GoDocExtractor extracts content from pkg.go.dev documentation pages.
type GoDocExtractor struct {
	cfg *config.Extractor
}

// Name returns the extractor's identifier.
func (e *GoDocExtractor) Name() string {
	return "GoDoc"
}

// Description returns a short summary of what this extractor does.
func (e *GoDocExtractor) Description() string {
	return "Extracts and renders Go package documentation from pkg.go.dev pages."
}

// GetConfig returns the extractor's current configuration.
func (e *GoDocExtractor) GetConfig() *config.Extractor {
	if e.cfg == nil {
		return &config.Extractor{Enable: true, Options: map[string]any{}}
	}
	return e.cfg
}

// SetConfig applies cfg to the extractor. Returns an error for unknown options.
func (e *GoDocExtractor) SetConfig(c *config.Extractor) error {
	for k := range c.Options {
		return fmt.Errorf("unknown option %q", k)
	}
	e.cfg = c
	return nil
}

// Match returns true for any pkg.go.dev URL that has a non-empty path beyond
// the host (i.e. at least one character after https://pkg.go.dev/).
func (e *GoDocExtractor) Match(d *document.Document) bool {
	return strings.HasPrefix(d.URL, pkgGoDevPrefix) && len(d.URL) > len(pkgGoDevPrefix)
}

// Extract does not provide a custom extractor
func (e *GoDocExtractor) Extract(d *document.Document) (types.ExtractorState, error) {
	return types.ExtractorContinue, nil
}

// Preview returns the sanitized HTML of the documentation base
// element with all relative links and image sources rewritten
// to absolute URLs.
func (e *GoDocExtractor) Preview(d *document.Document) (types.PreviewResponse, types.ExtractorState, error) {
	base, err := url.Parse(d.URL)
	if err != nil {
		return types.PreviewResponse{}, types.ExtractorContinue, err
	}
	content, err := extractArticle(d.HTML, true, base)
	if err != nil {
		return types.PreviewResponse{}, types.ExtractorContinue, err
	}
	return types.PreviewResponse{Content: sanitizer.SanitizeHTML(content)}, types.ExtractorStop, nil
}

func extractArticle(rawHTML string, renderHTML bool, base *url.URL) (string, error) {
	z := html.NewTokenizer(strings.NewReader(rawHTML))
	depth := 0
	inArticle := false
	var buf bytes.Buffer

	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			if errors.Is(z.Err(), io.EOF) {
				return buf.String(), nil
			}
			return "", z.Err()

		case html.StartTagToken, html.SelfClosingTagToken:
			tn, hasAttr := z.TagName()
			tag := string(tn)
			attrs := collectAttrs(z)

			if !inArticle {
				if tag == "div" && hasAttr && hasClass(attrs["class"], "Documentation-content") {
					inArticle = true
					depth = 1
					if renderHTML {
						writeTag(&buf, tag, attrs, false, base)
					}
				}
			} else {
				if tt == html.StartTagToken {
					depth++
				}
				if renderHTML {
					writeTag(&buf, tag, attrs, tt == html.SelfClosingTagToken, base)
				}
			}

		case html.EndTagToken:
			if inArticle {
				depth--
				if depth == 0 {
					if renderHTML {
						buf.WriteString("</div>")
					}
					return buf.String(), nil
				}
				if renderHTML {
					buf.Write(z.Raw())
				}
			}

		case html.TextToken:
			if inArticle {
				if renderHTML {
					buf.Write(z.Raw())
				} else {
					buf.Write(z.Text())
				}
			}

		default:
			if inArticle && renderHTML {
				buf.Write(z.Raw())
			}
		}
	}
}

// writeTag writes a reconstructed start or self-closing tag to buf, rewriting
// href and src attribute values to absolute URLs using base.
func writeTag(buf *bytes.Buffer, tag string, attrs map[string]string, selfClosing bool, base *url.URL) {
	buf.WriteByte('<')
	buf.WriteString(tag)
	for k, v := range attrs {
		if base != nil && (k == "href" || k == "src") {
			v = resolveURL(base, v)
		}
		buf.WriteByte(' ')
		buf.WriteString(k)
		buf.WriteString(`="`)
		buf.WriteString(escapeAttr(v))
		buf.WriteByte('"')
	}
	if selfClosing {
		buf.WriteString("/>")
	} else {
		buf.WriteByte('>')
	}
}

// resolveURL resolves ref against base. Returns ref unchanged if it is already
// absolute or cannot be parsed.
func resolveURL(base *url.URL, ref string) string {
	if ref == "" || strings.HasPrefix(ref, "#") || strings.HasPrefix(ref, "data:") {
		return ref
	}
	u, err := url.Parse(ref)
	if err != nil || u.IsAbs() {
		return ref
	}
	return base.ResolveReference(u).String()
}

// escapeAttr escapes special characters in HTML attribute values using the
// standard library's html.EscapeString.
func escapeAttr(s string) string {
	return stdhtml.EscapeString(s)
}

func collectAttrs(z *html.Tokenizer) map[string]string {
	attrs := make(map[string]string)
	for {
		key, val, more := z.TagAttr()
		if len(key) > 0 {
			attrs[string(key)] = string(val)
		}
		if !more {
			break
		}
	}
	return attrs
}

func hasClass(classAttr, target string) bool {
	return slices.Contains(strings.Fields(classAttr), target)
}
