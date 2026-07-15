package indexer

import (
	"testing"

	"github.com/asciimoo/hister/server/document"
	"github.com/asciimoo/hister/server/testutil"
)

func TestGetLatestDocumentsFiltered(t *testing.T) {
	cfg := testutil.Config(t)
	if err := Init(cfg); err != nil {
		t.Fatalf("failed to init indexer: %v", err)
	}
	defer i.Close()

	docs := []*document.Document{
		{
			URL:       "https://example.com/go",
			Title:     "Golang Test Guide",
			Text:      "Go documentation",
			Processed: true,
		},
		{
			URL:       "https://example.com/Rust/Testing",
			Title:     "Rust Guide",
			Text:      "Rust documentation",
			Processed: true,
		},
		{
			URL:       "https://example.com/other",
			Title:     "Other result",
			Text:      "Unrelated documentation",
			Processed: true,
		},
	}
	for _, doc := range docs {
		if err := Add(doc); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
	}

	tests := []struct {
		name    string
		filter  string
		wantURL string
	}{
		{"title phrase", "golang test", docs[0].URL},
		{"partial title", "olang", docs[0].URL},
		{"URL is case insensitive", "rust/testing", docs[1].URL},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetLatestDocumentsFiltered(100, "", 0, tt.filter)
			if result == nil {
				t.Fatal("filtered result is nil")
			}
			if len(result.Documents) != 1 {
				t.Fatalf("document count = %d, want 1", len(result.Documents))
			}
			if result.Documents[0].URL != tt.wantURL {
				t.Fatalf("URL = %q, want %q", result.Documents[0].URL, tt.wantURL)
			}
		})
	}
}
