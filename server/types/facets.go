// SPDX-License-Identifier: AGPL-3.0-or-later

package types

// TermCount counts documents matching a term in a facet.
type TermCount struct {
	Term  string `json:"term"`
	Count int    `json:"count"`
	Label string `json:"label,omitempty"`
}

// RangeCount counts documents in a named range.
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

// FacetsResult groups search matches by term and date range.
type FacetsResult struct {
	// Terms maps facet names, such as "domains" and "languages", to their counts.
	Terms         map[string]TermFacet `json:"terms,omitempty"`
	DateHistogram []RangeCount         `json:"date_histogram,omitempty"`
}
