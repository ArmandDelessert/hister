package jsonld

import (
	"testing"

	"github.com/asciimoo/hister/server/document"
	"github.com/asciimoo/hister/server/types"
)

func extract(t *testing.T, html string) *document.Document {
	t.Helper()
	d := &document.Document{URL: "https://example.com/", HTML: html}
	e := &JSONLDExtractor{}
	state, err := e.Extract(d)
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	if state != types.ExtractorContinue {
		t.Fatalf("Extract state = %v, want Continue", state)
	}
	return d
}

func TestWikipediaArticle(t *testing.T) {
	const h = `<html><head><script type="application/ld+json">{
		"@context": "https://schema.org",
		"@type": "Article",
		"name": "Kristi Noem",
		"headline": "Kristi Noem",
		"author": {"@type": "Organization", "name": "Contributors to Wikimedia projects"},
		"datePublished": "2010-01-01T00:00:00Z",
		"dateModified": "2026-04-01T00:00:00Z",
		"image": "https://upload.wikimedia.org/noem.jpg"
	}</script></head><body></body></html>`

	d := extract(t, h)
	checks := map[string]string{
		"jsonld_type":      "Article",
		"jsonld_headline":  "Kristi Noem",
		"jsonld_author":    "Contributors to Wikimedia projects",
		"jsonld_published": "2010-01-01T00:00:00Z",
		"jsonld_modified":  "2026-04-01T00:00:00Z",
		"jsonld_image":     "https://upload.wikimedia.org/noem.jpg",
	}
	for k, want := range checks {
		if got, _ := d.Metadata[k].(string); got != want {
			t.Errorf("Metadata[%q] = %q, want %q", k, got, want)
		}
	}
	if nodes, _ := d.Metadata["jsonld"].([]map[string]any); len(nodes) != 1 {
		t.Errorf("expected 1 flattened node, got %d", len(nodes))
	}
}

func TestYoastGraph(t *testing.T) {
	const h = `<html><head><script type="application/ld+json">{
		"@context": "https://schema.org",
		"@graph": [
			{"@type": "WebPage", "name": "About Us", "description": "The about page"},
			{"@type": "BreadcrumbList", "itemListElement": []},
			{"@type": "Organization", "name": "ACME"}
		]
	}</script></head></html>`

	d := extract(t, h)
	nodes, _ := d.Metadata["jsonld"].([]map[string]any)
	if len(nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(nodes))
	}
	if got, _ := d.Metadata["jsonld_type"].(string); got != "WebPage" {
		t.Errorf("jsonld_type = %q, want WebPage", got)
	}
	if got, _ := d.Metadata["jsonld_headline"].(string); got != "About Us" {
		t.Errorf("jsonld_headline = %q, want About Us", got)
	}
	if got, _ := d.Metadata["jsonld_description"].(string); got != "The about page" {
		t.Errorf("jsonld_description = %q", got)
	}
}

func TestMultipleScriptTags(t *testing.T) {
	const h = `<html><head>
		<script type="application/ld+json">{"@type": "Organization", "name": "ACME"}</script>
		<script type="application/ld+json">{"@type": "Article", "headline": "Hello"}</script>
	</head></html>`

	d := extract(t, h)
	nodes, _ := d.Metadata["jsonld"].([]map[string]any)
	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(nodes))
	}
	// Article is preferred over Organization.
	if got, _ := d.Metadata["jsonld_type"].(string); got != "Article" {
		t.Errorf("jsonld_type = %q, want Article", got)
	}
}

func TestArrayForm(t *testing.T) {
	const h = `<html><head><script type="application/ld+json">[
		{"@type": "Person", "name": "Alice"},
		{"@type": "Person", "name": "Bob"}
	]</script></head></html>`

	d := extract(t, h)
	nodes, _ := d.Metadata["jsonld"].([]map[string]any)
	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(nodes))
	}
}

func TestMalformedJSONContinues(t *testing.T) {
	const h = `<html><head>
		<script type="application/ld+json">{not valid json</script>
		<script type="application/ld+json">{"@type": "Article", "headline": "Survives"}</script>
	</head></html>`

	d := extract(t, h)
	if got, _ := d.Metadata["jsonld_headline"].(string); got != "Survives" {
		t.Errorf("jsonld_headline = %q, want Survives", got)
	}
	nodes, _ := d.Metadata["jsonld"].([]map[string]any)
	if len(nodes) != 1 {
		t.Errorf("expected 1 node (malformed skipped), got %d", len(nodes))
	}
}

func TestAuthorShapes(t *testing.T) {
	cases := []struct {
		name, blob, want string
	}{
		{"string", `{"@type":"Article","author":"Jane Doe"}`, "Jane Doe"},
		{"object", `{"@type":"Article","author":{"@type":"Person","name":"Jane Doe"}}`, "Jane Doe"},
		{"array", `{"@type":"Article","author":[{"@type":"Person","name":"Jane Doe"},{"@type":"Person","name":"John"}]}`, "Jane Doe"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := extract(t, `<script type="application/ld+json">`+tc.blob+`</script>`)
			if got, _ := d.Metadata["jsonld_author"].(string); got != tc.want {
				t.Errorf("jsonld_author = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestImageObjectShape(t *testing.T) {
	const h = `<script type="application/ld+json">{"@type":"Article","image":{"@type":"ImageObject","url":"https://ex.com/a.jpg"}}</script>`
	d := extract(t, h)
	if got, _ := d.Metadata["jsonld_image"].(string); got != "https://ex.com/a.jpg" {
		t.Errorf("jsonld_image = %q", got)
	}
}

func TestNoJSONLD(t *testing.T) {
	d := &document.Document{URL: "https://example.com/", HTML: "<html><body><p>hi</p></body></html>"}
	e := &JSONLDExtractor{}
	state, err := e.Extract(d)
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	if state != types.ExtractorContinue {
		t.Fatalf("state = %v, want Continue", state)
	}
	if _, ok := d.Metadata["jsonld"]; ok {
		t.Errorf("Metadata[jsonld] should not be set, got %v", d.Metadata)
	}
}

func TestMatchSkipsPagesWithoutJSONLD(t *testing.T) {
	e := &JSONLDExtractor{}
	if e.Match(&document.Document{HTML: ""}) {
		t.Error("Match should be false for empty HTML")
	}
	if e.Match(&document.Document{HTML: "<html><body><p>hi</p></body></html>"}) {
		t.Error("Match should be false for HTML without the ld+json marker")
	}
	if !e.Match(&document.Document{HTML: `<script type="application/ld+json">{}</script>`}) {
		t.Error("Match should be true when the marker is present")
	}
}

func TestSanitizeHeadlineStripsTagsAndDecodesEntities(t *testing.T) {
	// A real HTML document must escape </script> inside the blob; browsers
	// and golang.org/x/net/html both end the enclosing <script> at </script>
	// regardless of JSON context.
	const h = `<script type="application/ld+json">{
		"@type": "Article",
		"headline": "Smith &amp; Jones: <i>an unlikely<\/i> story",
		"description": "<script>alert(1)<\/script>plain",
		"author": {"@type": "Person", "name": "John O&#39;Brien"}
	}</script>`

	d := extract(t, h)
	if got, _ := d.Metadata["jsonld_headline"].(string); got != "Smith & Jones: an unlikely story" {
		t.Errorf("jsonld_headline = %q", got)
	}
	if got, _ := d.Metadata["jsonld_description"].(string); got != "plain" {
		t.Errorf("jsonld_description = %q", got)
	}
	if got, _ := d.Metadata["jsonld_author"].(string); got != "John O'Brien" {
		t.Errorf("jsonld_author = %q", got)
	}
}

func TestSanitizeImageRejectsNonHTTP(t *testing.T) {
	cases := []struct {
		name, raw, want string
	}{
		{"http", `"http://ex.com/a.jpg"`, "http://ex.com/a.jpg"},
		{"https", `"https://ex.com/a.jpg"`, "https://ex.com/a.jpg"},
		{"data-uri", `"data:image/png;base64,AAAA"`, ""},
		{"javascript", `"javascript:alert(1)"`, ""},
		{"relative", `"/images/a.jpg"`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := extract(t, `<script type="application/ld+json">{"@type":"Article","image":`+tc.raw+`}</script>`)
			got, _ := d.Metadata["jsonld_image"].(string)
			if got != tc.want {
				t.Errorf("jsonld_image = %q, want %q", got, tc.want)
			}
		})
	}
}
