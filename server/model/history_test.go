package model_test

import (
	"testing"

	"github.com/asciimoo/hister/server/model"
	"github.com/asciimoo/hister/server/testutil"
)

func TestGetLatestHistoryItemsFiltered(t *testing.T) {
	testutil.InitModel(t)

	entries := []struct {
		query string
		url   string
		title string
	}{
		{"go", "https://example.com/go", "Golang Test Guide"},
		{"docs", "https://docs.example.com/rust", "Rust Guide"},
		{"coverage", "https://example.com/coverage", "100% coverage"},
		{"other", "https://example.com/other", "Other result"},
	}
	for _, entry := range entries {
		if err := model.UpdateHistory(0, entry.query, entry.url, entry.title); err != nil {
			t.Fatalf("UpdateHistory failed: %v", err)
		}
	}

	tests := []struct {
		name    string
		filter  string
		wantURL string
	}{
		{"title is case insensitive", "GOLANG TEST", "https://example.com/go"},
		{"URL is case insensitive", "DOCS.EXAMPLE.COM", "https://docs.example.com/rust"},
		{"SQL wildcards are literal", "%", "https://example.com/coverage"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			items, err := model.GetLatestHistoryItemsFiltered(0, 100, 0, tt.filter)
			if err != nil {
				t.Fatalf("GetLatestHistoryItemsFiltered failed: %v", err)
			}
			if len(items) != 1 {
				t.Fatalf("item count = %d, want 1", len(items))
			}
			if items[0].URL != tt.wantURL {
				t.Fatalf("URL = %q, want %q", items[0].URL, tt.wantURL)
			}
		})
	}
}
