package server

import (
	"strings"
	"testing"

	"github.com/asciimoo/hister/server/document"
	"github.com/asciimoo/hister/server/indexer"
	"github.com/asciimoo/hister/server/types"
)

func TestMCPFormatResultsRendersSemanticOnlyDocuments(t *testing.T) {
	res := &indexer.Results{
		Total: 3,
		SemanticHits: []indexer.SemanticHit{
			{
				Similarity:   0.42,
				MatchedChunk: "semantic matched chunk",
				Document: &document.Document{
					URL:    "https://example.com/semantic",
					Title:  "Semantic result",
					Domain: "example.com",
					Text:   "semantic document text",
					Type:   types.Web,
				},
			},
		},
	}

	text := mcpFormatResults("semantic only", res, []string{"score", "domain", "type"})
	for _, want := range []string{
		`Found 3 result(s) for "semantic only"`,
		"Semantic result",
		"URL: https://example.com/semantic",
		"Domain: example.com",
		"Similarity: 0.4200",
		"Type: web",
		"semantic document text",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("MCP result missing %q in:\n%s", want, text)
		}
	}
}
