package sanitizer

import (
	stdhtml "html"
	"strings"

	"github.com/microcosm-cc/bluemonday"
)

var (
	htmlSanitizerPolicy *bluemonday.Policy
	textSanitizerPolicy = bluemonday.StrictPolicy()
)

func init() {
	p := bluemonday.NewPolicy()
	p.AllowElements(
		"a",
		"abbr",
		"b",
		"br",
		"canvas",
		"caption",
		"center",
		"cite",
		"code",
		"del",
		"details",
		"div",
		"dt",
		"em",
		"figcaption",
		"figure",
		"h1",
		"h2",
		"h3",
		"h4",
		"h5",
		"h6",
		"hr",
		"i",
		"img",
		"ins",
		"kbd",
		"label",
		"li",
		"math",
		"marquee",
		"media",
		"mediagroup",
		"noscript",
		"ol",
		"p",
		"pre",
		"source",
		"span",
		"strong",
		"sub",
		"summary",
		"sup",
		"svg",
		"table",
		"tbody",
		"td",
		"tfoot",
		"th",
		"thead",
		"title",
		"tr",
		"tt",
		"u",
		"ul",
		"video",
	)
	p.AllowStyles(
		"text-decoration",
		"color",
		"background",
		"background-color",
		"background-image",
		"font-size",
		"text-align",
	).Globally()
	p.AllowAttrs("href").OnElements("a")
	p.AllowAttrs("src", "srcset").OnElements("img", "source")
	p.AllowAttrs(
		"alt",
		"title",
		"id",
	).Globally()
	p.RequireParseableURLs(true)
	p.AllowDataURIImages()
	p.AllowImages()
	p.AllowTables()
	p.RequireNoFollowOnLinks(false)
	p.AllowURLSchemes("mailto", "http", "https")
	htmlSanitizerPolicy = p
}

func SanitizeHTML(h string) string {
	return htmlSanitizerPolicy.Sanitize(h)
}

// SanitizeText strips every HTML tag, decodes entities and trims
// surrounding whitespace, producing safe plain text suitable for
// storing in metadata or displaying verbatim.
func SanitizeText(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	s = textSanitizerPolicy.Sanitize(s)
	s = stdhtml.UnescapeString(s)
	return strings.TrimSpace(s)
}
