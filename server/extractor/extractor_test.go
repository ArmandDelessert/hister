package extractor

import (
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

func TestDiscourseRunsBeforeJSONLD(t *testing.T) {
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
	if _, exists := doc.Metadata["jsonld"]; exists {
		t.Fatal("JSON LD metadata was extracted before Discourse stopped the chain")
	}
}
