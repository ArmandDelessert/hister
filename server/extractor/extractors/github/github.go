// SPDX-License-Identifier: AGPL-3.0-or-later

// Package github provides an extractor for GitHub repository pages.
package github

import (
	"encoding/json"
	"fmt"
	stdhtml "html"
	"regexp"
	"strings"

	"github.com/asciimoo/hister/config"
	"github.com/asciimoo/hister/server/document"
	"github.com/asciimoo/hister/server/sanitizer"
	"github.com/asciimoo/hister/server/types"

	"github.com/PuerkitoBio/goquery"
)

const (
	githubBase      = "https://github.com"
	githubURLPrefix = githubBase + "/"
)

// relativeURLRe matches src="/" or href="/" attributes with root-relative paths
// (but not protocol-relative URLs starting with "//").
var relativeURLRe = regexp.MustCompile(`(?i)((?:src|href)=")(\/[^/"])`)

// githubSystemPaths are top-level GitHub path segments that are never
// repository owner namespaces.
var githubSystemPaths = map[string]bool{
	"settings":       true,
	"topics":         true,
	"sponsors":       true,
	"features":       true,
	"notifications":  true,
	"explore":        true,
	"marketplace":    true,
	"login":          true,
	"organizations":  true,
	"orgs":           true,
	"copilot":        true,
	"github-copilot": true,
	"new":            true,
	"issues":         true,
	"pulls":          true,
	"gist":           true,
	"about":          true,
	"contact":        true,
	"pricing":        true,
	"security":       true,
	"enterprise":     true,
	"apps":           true,
}

var starsRe = regexp.MustCompile(`^([\d,]+)\s+users?\s+starred\s+this\s+repository$`)

// GitHubExtractor extracts project details and README content from GitHub repository pages.
type GitHubExtractor struct {
	cfg *config.Extractor
}

func (e *GitHubExtractor) Name() string { return "GitHub" }

func (e *GitHubExtractor) Description() string {
	return "Extracts repository metadata (description, stars, topics, languages) and README content from GitHub project pages."
}

func (e *GitHubExtractor) GetConfig() *config.Extractor {
	if e.cfg == nil {
		return &config.Extractor{Enable: true, Options: map[string]any{}}
	}
	return e.cfg
}

func (e *GitHubExtractor) SetConfig(c *config.Extractor) error {
	for k := range c.Options {
		return fmt.Errorf("unknown option %q", k)
	}
	e.cfg = c
	return nil
}

// Match returns true for github.com/{owner}/{repo} URLs, excluding known
// GitHub system path prefixes.
func (e *GitHubExtractor) Match(d *document.Document) bool {
	if !strings.HasPrefix(d.URL, githubURLPrefix) {
		return false
	}
	path := strings.TrimPrefix(d.URL, githubURLPrefix)
	// Strip query string and fragment.
	if i := strings.IndexAny(path, "?#"); i >= 0 {
		path = path[:i]
	}
	path = strings.TrimSuffix(path, "/")
	parts := strings.SplitN(path, "/", 3)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return false
	}
	return !githubSystemPaths[strings.ToLower(parts[0])]
}

// repoInfo holds the extracted fields from a GitHub repository page.
type repoInfo struct {
	description string
	stars       string
	topics      []string
	languages   []string
	readmeHTML  string
}

// Extract populates d.Title and d.Text with repository metadata and README
// plain text, making the content fully searchable.
func (e *GitHubExtractor) Extract(d *document.Document) (types.ExtractorState, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(d.HTML))
	if err != nil {
		return types.ExtractorContinue, err
	}

	info := parseRepoPage(doc, d.HTML)
	if info == nil {
		return types.ExtractorContinue, nil
	}

	d.Title = strings.TrimSpace(doc.Find("title").First().Text())

	var b strings.Builder
	if info.description != "" {
		b.WriteString(info.description)
		b.WriteString("\n\n")
	}
	if len(info.topics) > 0 {
		b.WriteString("topics: ")
		b.WriteString(strings.Join(info.topics, ", "))
		b.WriteString("\n")
	}
	if len(info.languages) > 0 {
		b.WriteString("languages: ")
		b.WriteString(strings.Join(info.languages, ", "))
		b.WriteString("\n")
	}
	if info.stars != "" {
		b.WriteString("stars: ")
		b.WriteString(info.stars)
		b.WriteString("\n")
	}
	if info.readmeHTML != "" {
		readmeDoc, err := goquery.NewDocumentFromReader(strings.NewReader(info.readmeHTML))
		if err == nil {
			b.WriteString("\n")
			b.WriteString(strings.TrimSpace(readmeDoc.Text()))
		}
	}

	d.Text = strings.TrimSpace(b.String())
	if d.Text == "" && d.Title == "" {
		return types.ExtractorContinue, fmt.Errorf("no content found")
	}
	return types.ExtractorStop, nil
}

// Preview renders a summary card (description, stars, topics, languages) and
// the sanitized README HTML suitable for the preview panel.
func (e *GitHubExtractor) Preview(d *document.Document) (types.PreviewResponse, types.ExtractorState, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(d.HTML))
	if err != nil {
		return types.PreviewResponse{}, types.ExtractorContinue, err
	}

	info := parseRepoPage(doc, d.HTML)
	if info == nil {
		return types.PreviewResponse{}, types.ExtractorContinue, nil
	}

	var b strings.Builder

	// Metadata card.
	b.WriteString(`<div class="gh-meta">`)

	if info.description != "" {
		fmt.Fprintf(&b, `<p class="gh-description">%s</p>`, stdhtml.EscapeString(info.description))
	}

	if info.stars != "" || len(info.languages) > 0 {
		b.WriteString(`<p class="gh-stats">`)
		parts := make([]string, 0, 2)
		if info.stars != "" {
			parts = append(parts, fmt.Sprintf("&#9733; %s stars", stdhtml.EscapeString(info.stars)))
		}
		if len(info.languages) > 0 {
			parts = append(parts, stdhtml.EscapeString(strings.Join(info.languages, " / ")))
		}
		b.WriteString(strings.Join(parts, " &nbsp;&middot;&nbsp; "))
		b.WriteString("</p>")
	}

	if len(info.topics) > 0 {
		b.WriteString(`<p class="gh-topics">`)
		for _, t := range info.topics {
			fmt.Fprintf(&b, `<code>%s</code> `, stdhtml.EscapeString(t))
		}
		b.WriteString("</p>")
	}

	b.WriteString("</div>")

	if info.readmeHTML != "" {
		b.WriteString("<hr>")
		b.WriteString(sanitizer.SanitizeHTML(info.readmeHTML))
	}

	return types.PreviewResponse{Content: b.String()}, types.ExtractorStop, nil
}

// parseRepoPage extracts repository metadata from the parsed goquery document.
// Returns nil if the page does not appear to be a repository overview page.
func parseRepoPage(doc *goquery.Document, rawHTML string) *repoInfo {
	info := &repoInfo{}

	// Description: the sidebar "about" paragraph (class varies by page version).
	desc := strings.TrimSpace(doc.Find("p.f4").First().Text())
	if desc == "" {
		return nil
	}
	info.description = desc

	// Star count from the star button aria-label.
	doc.Find("[aria-label]").Each(func(_ int, s *goquery.Selection) {
		label, _ := s.Attr("aria-label")
		if m := starsRe.FindStringSubmatch(strings.TrimSpace(label)); m != nil {
			info.stars = m[1]
		}
	})

	// Topics from sidebar topic tag links.
	seen := make(map[string]bool)
	doc.Find(`a[href^="/topics/"].topic-tag-link`).Each(func(_ int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		topic := strings.TrimPrefix(href, "/topics/")
		if topic != "" && !seen[topic] {
			seen[topic] = true
			info.topics = append(info.topics, topic)
		}
	})

	// Primary languages from the language bar.
	seenLang := make(map[string]bool)
	doc.Find("span.color-fg-default.text-bold.mr-1").Each(func(_ int, s *goquery.Selection) {
		lang := strings.TrimSpace(s.Text())
		if lang != "" && lang != "Other" && !seenLang[lang] {
			seenLang[lang] = true
			info.languages = append(info.languages, lang)
		}
	})

	// README HTML from the embedded JSON payload (works for both the
	// react-app.embeddedData and react-partial.embeddedData formats).
	if rt := extractReadmeHTML(doc); rt != "" {
		info.readmeHTML = resolveRelativeURLs(rt)
	}

	return info
}

// resolveRelativeURLs rewrites root-relative src/href attributes in README HTML
// to absolute github.com URLs (e.g. "/owner/repo/raw/..." → "https://github.com/owner/repo/raw/...").
// Protocol-relative URLs ("//...") are left untouched.
func resolveRelativeURLs(html string) string {
	return relativeURLRe.ReplaceAllString(html, "${1}"+githubBase+"${2}")
}

// extractReadmeHTML searches all application/json script blocks for the first
// overviewFiles entry that has non-empty richText (the rendered README HTML).
func extractReadmeHTML(doc *goquery.Document) string {
	var result string
	doc.Find(`script[type="application/json"]`).EachWithBreak(func(_ int, s *goquery.Selection) bool {
		raw := s.Text()
		if !strings.Contains(raw, "overviewFiles") {
			return true // continue
		}
		var payload any
		if err := json.Unmarshal([]byte(raw), &payload); err != nil {
			return true
		}
		if rt := findRichText(payload); rt != "" {
			result = rt
			return false // stop
		}
		return true
	})
	return result
}

// findRichText recursively walks a JSON-decoded value and returns the first
// non-empty richText string found inside an overviewFiles list.
func findRichText(v any) string {
	switch val := v.(type) {
	case map[string]any:
		if files, ok := val["overviewFiles"]; ok {
			if rt := richTextFromFiles(files); rt != "" {
				return rt
			}
		}
		for _, child := range val {
			if rt := findRichText(child); rt != "" {
				return rt
			}
		}
	case []any:
		for _, item := range val {
			if rt := findRichText(item); rt != "" {
				return rt
			}
		}
	}
	return ""
}

// richTextFromFiles extracts the first non-empty richText from an overviewFiles
// JSON array value.
func richTextFromFiles(v any) string {
	files, ok := v.([]any)
	if !ok {
		return ""
	}
	for _, f := range files {
		entry, ok := f.(map[string]any)
		if !ok {
			continue
		}
		rt, _ := entry["richText"].(string)
		if rt != "" {
			return rt
		}
	}
	return ""
}
