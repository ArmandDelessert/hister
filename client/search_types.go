// SPDX-License-Identifier: AGPL-3.0-or-later

package client

// SearchQuery contains parameters for the search API.
type SearchQuery struct {
	Text              string  `json:"text"`
	Highlight         string  `json:"highlight"`
	Limit             int     `json:"limit"`
	Sort              string  `json:"sort"`
	DateFrom          int64   `json:"date_from"`
	DateTo            int64   `json:"date_to"`
	UserID            uint    `json:"user_id"`
	SemanticEnabled   bool    `json:"semantic_enabled"`
	SemanticThreshold float64 `json:"semantic_threshold"`
	SemanticWeight    float64 `json:"semantic_weight"`
	PageKey           string  `json:"page_key"`
	IncludeHTML       bool    `json:"include_html"`
	IncludeText       bool    `json:"include_text"`
	Facets            bool    `json:"facets,omitempty"`
	// FacetSizes overrides the schema default cap per named facet.
	FacetSizes map[string]int `json:"facet_sizes,omitempty"`
	// FacetsOnly returns only facet counts and requires Facets=true.
	FacetsOnly bool `json:"facets_only,omitempty"`
	// MatchAll ignores Text while retaining ownership, date, and facet filters.
	MatchAll bool `json:"match_all,omitempty"`
	// PriorityPatterns contains URL regex patterns whose matching documents
	// receive a score boost. The server sets these from the user's rules.
	PriorityPatterns []string `json:"priority_patterns,omitempty"`
}

// TermCount and RangeCount describe facet buckets returned when Facets is true.
type TermCount struct {
	Term  string `json:"term"`
	Count int    `json:"count"`
	Label string `json:"label,omitempty"`
}

type RangeCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// TermFacet holds the most common terms for one facet. Other counts documents
// matching terms outside this list.
type TermFacet struct {
	Terms []TermCount `json:"terms,omitempty"`
	Other int         `json:"other,omitempty"`
}

type FacetsResult struct {
	// Terms maps facet names, such as "domains" and "languages", to results.
	Terms         map[string]TermFacet `json:"terms,omitempty"`
	DateHistogram []RangeCount         `json:"date_histogram,omitempty"`
}

// SemanticHit represents a document found via vector similarity search.
type SemanticHit struct {
	DocID        string    `json:"doc_id"`
	Similarity   float64   `json:"similarity"`
	MatchedChunk string    `json:"matched_chunk,omitempty"`
	Document     *Document `json:"document,omitempty"`
}

// SearchResults contains the response to an HTTP or WebSocket search.
type SearchResults struct {
	Total           uint64               `json:"total"`
	Query           *SearchQuery         `json:"query"`
	Documents       []*Document          `json:"documents"`
	History         []*SearchHistoryItem `json:"history"`
	SearchDuration  string               `json:"search_duration"`
	QuerySuggestion string               `json:"query_suggestion"`
	PageKey         string               `json:"page_key"`
	SemanticHits    []SemanticHit        `json:"semantic_hits,omitempty"`
	SemanticEnabled bool                 `json:"semantic_enabled"`
	Facets          *FacetsResult        `json:"facets,omitempty"`
}

// SearchHistoryItem is a previously visited URL included in search results.
type SearchHistoryItem struct {
	URL      string `json:"url"`
	Title    string `json:"title"`
	Text     string `json:"text,omitempty"`
	Count    uint   `json:"count"`
	Pinned   bool   `json:"pinned"`
	DocID    string `json:"id,omitempty"`
	Domain   string `json:"domain,omitempty"`
	Added    int64  `json:"added,omitempty"`
	Updated  int64  `json:"updated,omitempty"`
	AddCount uint   `json:"add_count,omitempty"`
}
