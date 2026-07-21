package indexer

import (
	"strings"
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

func TestSearchFiltersMetadataSourceByLatestUpdate(t *testing.T) {
	idxCfg := testutil.Config(t)
	if err := Init(idxCfg); err != nil {
		t.Fatalf("failed to init indexer: %v", err)
	}
	defer i.Close()

	docs := []*document.Document{
		{
			URL:       "https://example.com/older-linkwarden",
			Title:     "Older Linkwarden document",
			Updated:   100,
			Metadata:  map[string]any{"source": "linkwarden"},
			Processed: true,
		},
		{
			URL:       "https://example.com/newer-linkwarden",
			Title:     "Newer Linkwarden document",
			Updated:   200,
			Metadata:  map[string]any{"source": "linkwarden"},
			Processed: true,
		},
		{
			URL:       "https://example.com/unrelated",
			Title:     "Unrelated document",
			Updated:   300,
			Metadata:  map[string]any{"source": "other"},
			Processed: true,
		},
	}
	for _, doc := range docs {
		if err := Add(doc); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
	}

	res, err := Search(idxCfg, &Query{Text: "metadata.source:linkwarden", Sort: "date", Limit: 1})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(res.Documents) != 1 {
		t.Fatalf("Search returned %d documents, want 1", len(res.Documents))
	}
	if res.Documents[0].URL != docs[1].URL || res.Documents[0].Updated != 200 {
		t.Fatalf("latest Linkwarden document = %#v, want %#v", res.Documents[0], docs[1])
	}
}

func TestSearchFiltersByVisitCount(t *testing.T) {
	idxCfg := testutil.Config(t)
	if err := Init(idxCfg); err != nil {
		t.Fatalf("failed to init indexer: %v", err)
	}
	defer i.Close()

	lessVisitedURL := "https://example.com/visit-filter-less"
	mostVisitedURL := "https://example.com/visit-filter-most"
	docs := []string{
		lessVisitedURL,
		mostVisitedURL,
		mostVisitedURL,
		mostVisitedURL,
	}
	for _, url := range docs {
		if err := Add(&document.Document{
			URL:   url,
			Title: "Visited filter",
			Text:  "Visited filter document text",
		}); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
	}

	res, err := Search(idxCfg, &Query{Text: "Visited filter visits:2..4"})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(res.Documents) != 1 {
		t.Fatalf("Search returned %d documents, want 1", len(res.Documents))
	}
	if res.Documents[0].URL != mostVisitedURL {
		t.Fatalf("result URL = %q, want %q", res.Documents[0].URL, mostVisitedURL)
	}
}

func TestSearchVisitCountFacets(t *testing.T) {
	idxCfg := testutil.Config(t)
	if err := Init(idxCfg); err != nil {
		t.Fatalf("failed to init indexer: %v", err)
	}
	defer i.Close()

	lessVisitedURL := "https://example.com/visit-facet-less"
	mostVisitedURL := "https://example.com/visit-facet-most"
	docs := []string{
		lessVisitedURL,
		mostVisitedURL,
		mostVisitedURL,
		mostVisitedURL,
	}
	for _, url := range docs {
		if err := Add(&document.Document{
			URL:   url,
			Title: "Visited facet",
			Text:  "Visited facet document text",
		}); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
	}

	res, err := Search(idxCfg, &Query{Text: "Visited facet", Facets: true, FacetsOnly: true})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if res.Facets == nil {
		t.Fatal("Facets is nil")
	}
	visits := res.Facets.Terms["visits"].Terms
	counts := make(map[string]int, len(visits))
	labels := make(map[string]string, len(visits))
	for _, bucket := range visits {
		counts[bucket.Term] = bucket.Count
		labels[bucket.Term] = bucket.Label
	}
	if counts["1"] != 1 {
		t.Fatalf("visit bucket 1 = %d, want 1", counts["1"])
	}
	if counts["2..4"] != 1 {
		t.Fatalf("visit bucket 2..4 = %d, want 1", counts["2..4"])
	}
	if labels["2..4"] != "2 to 4" {
		t.Fatalf("visit bucket label 2..4 = %q, want %q", labels["2..4"], "2 to 4")
	}
}

func TestSearchReturnsFaviconKeyWithoutFaviconData(t *testing.T) {
	idxCfg := testutil.Config(t)
	if err := Init(idxCfg); err != nil {
		t.Fatalf("failed to init indexer: %v", err)
	}
	defer i.Close()

	const faviconData = "data:image/png;base64,ZmF2aWNvbg=="
	if err := Add(&document.Document{
		URL:     "https://example.com/favicon-key",
		Title:   "Favicon key",
		Text:    "Favicon key document text",
		Favicon: faviconData,
	}); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	res, err := Search(idxCfg, &Query{Text: "Favicon key"})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(res.Documents) != 1 {
		t.Fatalf("Search returned %d documents, want 1", len(res.Documents))
	}
	doc := res.Documents[0]
	if doc.Favicon != "" {
		t.Fatalf("Favicon data was included in search result: %.32q", doc.Favicon)
	}
	if doc.FaviconKey == "" {
		t.Fatal("FaviconKey is empty")
	}
	if strings.Contains(doc.FaviconKey, "data:") {
		t.Fatalf("FaviconKey contains inline data: %q", doc.FaviconKey)
	}

	data, err := ReadFavicon(doc.FaviconKey)
	if err != nil {
		t.Fatalf("ReadFavicon failed: %v", err)
	}
	if string(data) != faviconData {
		t.Fatalf("ReadFavicon = %q, want %q", string(data), faviconData)
	}
}
