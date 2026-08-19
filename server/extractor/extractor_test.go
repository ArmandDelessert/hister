package extractor

import (
	"errors"
	"strings"
	"testing"

	"github.com/asciimoo/hister/server/document"
	"github.com/asciimoo/hister/server/types"
)

func TestReadabilityPreviewSanitizesMetaRefresh(t *testing.T) {
	doc := &document.Document{
		URL: "file:///tmp/index.html",
		HTML: `<!doctype html>
<html>
  <head>
    <title>Local docs</title>
    <meta http-equiv="refresh" content="0; url=EEx.html">
  </head>
  <body>
    <article>
      <h1>Local docs</h1>
      <p>This document has enough readable content for the readability extractor to render a preview.</p>
      <p>The preview must not preserve navigation tags from the original head.</p>
    </article>
  </body>
</html>`,
	}

	resp, state, err := (&readabilityExtractor{}).Preview(doc)
	if err != nil {
		t.Fatalf("Preview failed: %v", err)
	}
	if state != types.ExtractorStop {
		t.Fatalf("state = %v, want %v", state, types.ExtractorStop)
	}
	lower := strings.ToLower(resp.Content)
	for _, disallowed := range []string{"http-equiv", "refresh", "eex.html"} {
		if strings.Contains(lower, disallowed) {
			t.Fatalf("preview content contains %q:\n%s", disallowed, resp.Content)
		}
	}
	if !strings.Contains(resp.Content, "readable content") {
		t.Fatalf("preview content missing article text:\n%s", resp.Content)
	}
}

func TestBasicPreviewEscapesMarkup(t *testing.T) {
	doc := &document.Document{
		Text: `<p>safe text</p><meta http-equiv="refresh" content="0; url=EEx.html">`,
	}

	resp, state, err := (&basicExtractor{}).Preview(doc)
	if err != nil {
		t.Fatalf("Preview failed: %v", err)
	}
	if state != types.ExtractorStop {
		t.Fatalf("state = %v, want %v", state, types.ExtractorStop)
	}
	for _, disallowed := range []string{"<p>", "<meta", `http-equiv="refresh"`} {
		if strings.Contains(resp.Content, disallowed) {
			t.Fatalf("preview content contains %q:\n%s", disallowed, resp.Content)
		}
	}
	for _, want := range []string{"&lt;p&gt;safe text&lt;/p&gt;", "&lt;meta"} {
		if !strings.Contains(resp.Content, want) {
			t.Fatalf("preview content missing escaped markup %q:\n%s", want, resp.Content)
		}
	}
	if !strings.Contains(resp.Content, "safe text") {
		t.Fatalf("preview content missing text:\n%s", resp.Content)
	}
}

func TestEnrichersRunBeforeContentExtractor(t *testing.T) {
	doc := &document.Document{
		URL: "https://forum.example.com/t/topic/42",
		HTML: `<html><head>
			<meta name="generator" content="Discourse 2026.8.0">
			<script type="application/ld+json">{
				"@context": "https://schema.org",
				"@type": "QAPage",
				"name": "Topic"
			}</script>
		</head><body>
			<div id="topic-title"><h1 data-topic-id="42"><a>Topic</a></h1></div>
			<div class="post-stream">
				<div class="topic-post" data-post-number="1">
					<article data-post-id="100">
						<div class="names"><span class="username"><a>author</a></span></div>
						<div class="cooked"><p>Topic body.</p></div>
					</article>
				</div>
			</div>
		</body></html>`,
	}

	if err := Extract(doc); err != nil {
		t.Fatalf("Extract failed: %v", err)
	}
	if got := doc.Metadata["type"]; got != "discourse" {
		t.Fatalf("metadata type = %#v, want discourse", got)
	}
	if _, exists := doc.Metadata["jsonld"]; !exists {
		t.Fatal("JSON LD enricher did not run before Discourse selected the content")
	}
}

func TestRegisteredExtractorCapabilities(t *testing.T) {
	for _, candidate := range extractors {
		caps := candidate.Capabilities()
		if !caps.Enrich && !caps.Extract && !caps.Preview {
			t.Errorf("%s declares no capabilities", candidate.Name())
		}

		switch candidate.Name() {
		case "EmbeddedVideo", "JSONLD":
			if !caps.Enrich || caps.Extract || caps.Preview {
				t.Errorf("%s capabilities = %+v, want enrichment only", candidate.Name(), caps)
			}
		case "Markdown", "OrgMode", "GoDoc":
			if caps.Enrich || caps.Extract || !caps.Preview {
				t.Errorf("%s capabilities = %+v, want preview only", candidate.Name(), caps)
			}
		default:
			if caps.Enrich || !caps.Extract || !caps.Preview {
				t.Errorf("%s capabilities = %+v, want content and preview", candidate.Name(), caps)
			}
		}
	}
}

func TestListMatchingPreviewOmitsEnrichers(t *testing.T) {
	doc := &document.Document{
		URL: "https://example.com/article",
		HTML: `<html><head><script type="application/ld+json">{"@type":"Article"}</script></head>
			<body><iframe src="https://www.youtube.com/embed/example"></iframe></body></html>`,
	}

	all := make(map[string]bool)
	for _, info := range ListMatching(doc) {
		all[info.Name] = true
	}
	for _, name := range []string{"EmbeddedVideo", "JSONLD"} {
		if !all[name] {
			t.Fatalf("ListMatching omitted matching enricher %s", name)
		}
	}

	for _, info := range ListMatchingPreview(doc) {
		if !info.Capabilities.Preview {
			t.Errorf("ListMatchingPreview returned non-preview extractor %s", info.Name)
		}
		if info.Name == "EmbeddedVideo" || info.Name == "JSONLD" {
			t.Errorf("ListMatchingPreview returned enricher %s", info.Name)
		}
	}
}

func TestExplicitPreviewRejectsExtractorWithoutPreviewCapability(t *testing.T) {
	doc := &document.Document{
		URL:  "https://example.com/article",
		HTML: `<script type="application/ld+json">{"@type":"Article"}</script>`,
	}

	_, err := Preview(doc, "JSONLD")
	if !errors.Is(err, ErrNoExtractor) {
		t.Fatalf("Preview error = %v, want ErrNoExtractor", err)
	}
}
