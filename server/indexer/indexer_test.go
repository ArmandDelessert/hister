package indexer

import (
	"testing"

	"github.com/asciimoo/hister/server/document"
	"github.com/asciimoo/hister/server/testutil"
)

func TestSearchSortsByMostVisited(t *testing.T) {
	idxCfg := testutil.Config(t)
	if err := Init(idxCfg); err != nil {
		t.Fatalf("failed to init indexer: %v", err)
	}
	defer i.Close()

	lessVisitedURL := "https://example.com/less-visited"
	mostVisitedURL := "https://example.com/most-visited"
	docs := []string{
		lessVisitedURL,
		mostVisitedURL,
		mostVisitedURL,
		mostVisitedURL,
	}
	for _, url := range docs {
		if err := Add(&document.Document{
			URL:   url,
			Title: "Visited sort",
			Text:  "Visited sort document text",
		}); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
	}

	res, err := Search(idxCfg, &Query{Text: "*", Sort: "visits"})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(res.Documents) < 2 {
		t.Fatalf("Search returned %d documents, want at least 2", len(res.Documents))
	}
	if res.Documents[0].URL != mostVisitedURL {
		t.Fatalf("first result URL = %q, want %q", res.Documents[0].URL, mostVisitedURL)
	}
	if res.Documents[0].AddCount != 3 {
		t.Fatalf("first result AddCount = %d, want 3", res.Documents[0].AddCount)
	}
	if res.Documents[1].URL != lessVisitedURL {
		t.Fatalf("second result URL = %q, want %q", res.Documents[1].URL, lessVisitedURL)
	}
}
