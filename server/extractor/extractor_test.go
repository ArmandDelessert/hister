package extractor

import (
	"errors"
	"strings"
	"testing"

	"github.com/asciimoo/hister/config"
	"github.com/asciimoo/hister/server/document"
	"github.com/asciimoo/hister/server/types"
)

type stubExtractor struct {
	name         string
	capabilities types.ExtractorCapabilities
	cfg          *config.Extractor
	match        func(*document.Document) bool
	extract      func(*document.Document) types.ExtractResult
	preview      func(*document.Document) types.PreviewResult
}

func (e *stubExtractor) Name() string { return e.name }

func (e *stubExtractor) Description() string { return e.name }

func (e *stubExtractor) Capabilities() types.ExtractorCapabilities { return e.capabilities }

func (e *stubExtractor) Match(d *document.Document) bool {
	return e.match == nil || e.match(d)
}

func (e *stubExtractor) Extract(d *document.Document) types.ExtractResult {
	if e.extract == nil {
		return types.ExtractFallback(nil)
	}
	return e.extract(d)
}

func (e *stubExtractor) Preview(d *document.Document) types.PreviewResult {
	if e.preview == nil {
		return types.PreviewFallback(nil)
	}
	return e.preview(d)
}

func (e *stubExtractor) GetConfig() *config.Extractor {
	if e.cfg == nil {
		return &config.Extractor{Enable: true, Options: map[string]any{}}
	}
	return e.cfg
}

func (e *stubExtractor) SetConfig(cfg *config.Extractor) error {
	e.cfg = cfg
	return nil
}

func useExtractors(t *testing.T, replacements ...Extractor) {
	t.Helper()
	original := extractors
	extractors = replacements
	t.Cleanup(func() { extractors = original })
}

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

	result := (&readabilityExtractor{}).Preview(doc)
	if result.Err() != nil {
		t.Fatalf("Preview failed: %v", result.Err())
	}
	if result.Decision() != types.ExtractorSuccess {
		t.Fatalf("decision = %v, want %v", result.Decision(), types.ExtractorSuccess)
	}
	resp := result.Response()
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

	result := (&basicExtractor{}).Preview(doc)
	if result.Err() != nil {
		t.Fatalf("Preview failed: %v", result.Err())
	}
	if result.Decision() != types.ExtractorSuccess {
		t.Fatalf("decision = %v, want %v", result.Decision(), types.ExtractorSuccess)
	}
	resp := result.Response()
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

func TestInvalidExtractionResultStopsTheChain(t *testing.T) {
	invalid := &stubExtractor{
		name:         "Invalid",
		capabilities: types.ExtractorCapabilities{Extract: true},
		extract: func(*document.Document) types.ExtractResult {
			return types.ExtractResult{}
		},
	}
	useExtractors(t, invalid)

	err := Extract(&document.Document{URL: "https://example.com"})
	if !errors.Is(err, ErrInvalidExtractorResult) {
		t.Fatalf("Extract error = %v, want ErrInvalidExtractorResult", err)
	}
}

func TestExplicitPreviewFallsBack(t *testing.T) {
	first := &stubExtractor{
		name:         "First",
		capabilities: types.ExtractorCapabilities{Preview: true},
		preview: func(*document.Document) types.PreviewResult {
			return types.PreviewFallback(errors.New("preview declined"))
		},
	}
	second := &stubExtractor{
		name:         "Second",
		capabilities: types.ExtractorCapabilities{Preview: true},
		preview: func(*document.Document) types.PreviewResult {
			return types.Previewed(types.PreviewResponse{Content: "fallback preview"})
		},
	}
	useExtractors(t, first, second)
	doc := &document.Document{URL: "https://example.com"}

	response, err := Preview(doc, "First")
	if err != nil {
		t.Fatalf("Preview failed: %v", err)
	}
	if response.Content != "fallback preview" {
		t.Fatalf("Preview content = %q, want fallback preview", response.Content)
	}
}

func TestExplicitPreviewRequiresEnabledMatchingExtractor(t *testing.T) {
	t.Run("disabled", func(t *testing.T) {
		candidate := &stubExtractor{
			name:         "Disabled",
			capabilities: types.ExtractorCapabilities{Preview: true},
			cfg:          &config.Extractor{Enable: false, Options: map[string]any{}},
		}
		useExtractors(t, candidate)
		_, err := Preview(&document.Document{URL: "https://example.com"}, candidate.Name())
		if !errors.Is(err, ErrNoExtractor) {
			t.Fatalf("Preview error = %v, want ErrNoExtractor", err)
		}
	})

	t.Run("not matching", func(t *testing.T) {
		candidate := &stubExtractor{
			name:         "NotMatching",
			capabilities: types.ExtractorCapabilities{Preview: true},
			match:        func(*document.Document) bool { return false },
		}
		useExtractors(t, candidate)
		_, err := Preview(&document.Document{URL: "https://example.com"}, candidate.Name())
		if !errors.Is(err, ErrNoExtractor) {
			t.Fatalf("Preview error = %v, want ErrNoExtractor", err)
		}
	})
}
