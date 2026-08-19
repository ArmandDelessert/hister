// SPDX-License-Identifier: AGPL-3.0-or-later

package mastodon

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/asciimoo/hister/config"
	"github.com/asciimoo/hister/server/document"
	"github.com/asciimoo/hister/server/extractor/urlutil"
	"github.com/asciimoo/hister/server/sanitizer"
	"github.com/asciimoo/hister/server/types"

	"github.com/PuerkitoBio/goquery"
	"github.com/rs/zerolog/log"
)

type MastodonExtractor struct {
	cfg *config.Extractor
}

func (e *MastodonExtractor) Name() string {
	return "Mastodon"
}

func (e *MastodonExtractor) Description() string {
	return "Extracts toots as individual documents from Mastodon websites."
}

func (e *MastodonExtractor) Capabilities() types.ExtractorCapabilities {
	return types.ExtractorCapabilities{Extract: true, Preview: true}
}

func (e *MastodonExtractor) GetConfig() *config.Extractor {
	if e.cfg == nil {
		return &config.Extractor{
			Enable:  true,
			Options: map[string]any{},
		}
	}
	return e.cfg
}

func (e *MastodonExtractor) SetConfig(c *config.Extractor) error {
	for k := range c.Options {
		return fmt.Errorf("unknown option %q", k)
	}
	e.cfg = c
	return nil
}

func (e *MastodonExtractor) Match(d *document.Document) bool {
	if strings.Contains(d.HTML, `"repository":"mastodon/mastodon"`) {
		return true
	}
	if d.Metadata != nil && d.Metadata["type"] == "toot" {
		return true
	}
	return false
}

func (e *MastodonExtractor) Extract(d *document.Document) types.ExtractResult {
	if d.Metadata != nil && d.Metadata["type"] == "toot" {
		return types.Extracted()
	}
	d.SkipIndexing = true
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(d.HTML))
	if err != nil {
		return types.ExtractFallback(nil)
	}
	statuses := doc.Find(".status, .detailed-status")
	if statuses.Length() == 0 {
		return types.Extracted()
	}

	pu, err := url.Parse(d.URL)
	if err != nil {
		return types.ExtractFallback(nil)
	}

	statuses.Each(func(_ int, s *goquery.Selection) {
		c := s.Find(".status__content")
		urlutil.RewriteURLs(c, pu)
		u, exists := s.Find(".status__relative-time, .detailed-status__datetime").First().Attr("href")
		if !exists {
			log.Debug().Msg("Failed to find URL for toot")
			return
		}
		h, err := c.Html()
		if err != nil {
			log.Debug().Msg("Failed to extract HTML for toot")
			return
		}
		statusURL := urlutil.ResolveURL(pu, u)
		td := &document.Document{
			URL:    originalStatusURL(statusURL),
			Title:  "Mastodon toot: " + s.Find(".display-name").Text(),
			Text:   c.Text(),
			HTML:   h,
			UserID: d.UserID,
			Metadata: map[string]any{
				"type": "toot",
			},
		}
		d.ExtraDocuments = append(d.ExtraDocuments, td)
	})

	return types.Extracted()
}

func originalStatusURL(rawURL string) string {
	statusURL, err := url.Parse(rawURL)
	if err != nil || (statusURL.Scheme != "http" && statusURL.Scheme != "https") {
		return rawURL
	}

	parts := strings.Split(strings.Trim(statusURL.EscapedPath(), "/"), "/")
	if len(parts) != 2 {
		return rawURL
	}

	account, err := url.PathUnescape(parts[0])
	if err != nil || !strings.HasPrefix(account, "@") {
		return rawURL
	}

	separator := strings.LastIndex(account, "@")
	if separator <= 1 || separator == len(account)-1 {
		return rawURL
	}

	username := account[1:separator]
	remoteHost := account[separator+1:]
	statusID, err := url.PathUnescape(parts[1])
	if err != nil || strings.ContainsAny(username+statusID, "/?#") {
		return rawURL
	}

	remoteURL, err := url.Parse("https://" + remoteHost)
	if err != nil ||
		remoteURL.Hostname() == "" ||
		remoteURL.User != nil ||
		remoteURL.Host != remoteHost ||
		remoteURL.Path != "" ||
		remoteURL.RawQuery != "" ||
		remoteURL.Fragment != "" {
		return rawURL
	}

	return (&url.URL{
		Scheme: "https",
		Host:   remoteURL.Host,
		Path:   "/@" + username + "/" + statusID,
	}).String()
}

func (e *MastodonExtractor) Preview(d *document.Document) types.PreviewResult {
	// TODO enhance the toot preview
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(d.HTML))
	if err != nil {
		return types.PreviewFallback(err)
	}

	var b strings.Builder

	// --- Title -----------------------------------------------------------
	if title := strings.TrimSpace(doc.Find("h1").First().Text()); title != "" {
		fmt.Fprintf(&b, "<h2>%s</h2>\n", title)
	}
	b.WriteString(d.HTML)

	// Always sanitize HTML before returning it to strip scripts, event
	// handlers, and other potentially unsafe markup.
	return types.Previewed(types.PreviewResponse{
		Content: sanitizer.SanitizeHTML(b.String()),
	})
}
