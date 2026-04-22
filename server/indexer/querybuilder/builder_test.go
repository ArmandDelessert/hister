package querybuilder

import (
	"fmt"
	"strings"
	"testing"

	"github.com/blevesearch/bleve/v2/search/query"
)

// --- helpers ---

// buildBoolQ calls Build(s) and asserts the result is a *query.BooleanQuery.
func buildBoolQ(t *testing.T, s string) *query.BooleanQuery {
	t.Helper()
	q := Build(s)
	bq, ok := q.(*query.BooleanQuery)
	if !ok {
		t.Fatalf("Build(%q): expected *query.BooleanQuery, got %T", s, q)
	}
	return bq
}

// mustClauses extracts the Conjuncts from bq.Must, failing if Must is nil or the wrong type.
func mustClauses(t *testing.T, bq *query.BooleanQuery) []query.Query {
	t.Helper()
	if bq.Must == nil {
		t.Fatalf("BooleanQuery.Must is nil")
	}
	cq, ok := bq.Must.(*query.ConjunctionQuery)
	if !ok {
		t.Fatalf("BooleanQuery.Must: expected *query.ConjunctionQuery, got %T", bq.Must)
	}
	return cq.Conjuncts
}

// mustNotClauses extracts the Disjuncts from bq.MustNot, failing if MustNot is nil or the wrong type.
func mustNotClauses(t *testing.T, bq *query.BooleanQuery) []query.Query {
	t.Helper()
	if bq.MustNot == nil {
		t.Fatalf("BooleanQuery.MustNot is nil")
	}
	dq, ok := bq.MustNot.(*query.DisjunctionQuery)
	if !ok {
		t.Fatalf("BooleanQuery.MustNot: expected *query.DisjunctionQuery, got %T", bq.MustNot)
	}
	return dq.Disjuncts
}

// asDisjunction type-asserts q to *query.DisjunctionQuery.
func asDisjunction(t *testing.T, q query.Query) *query.DisjunctionQuery {
	t.Helper()
	dq, ok := q.(*query.DisjunctionQuery)
	if !ok {
		t.Fatalf("expected *query.DisjunctionQuery, got %T", q)
	}
	return dq
}

// asMatch type-asserts q to *query.MatchQuery.
func asMatch(t *testing.T, q query.Query) *query.MatchQuery {
	t.Helper()
	mq, ok := q.(*query.MatchQuery)
	if !ok {
		t.Fatalf("expected *query.MatchQuery, got %T", q)
	}
	return mq
}

// asTerm type-asserts q to *query.TermQuery.
func asTerm(t *testing.T, q query.Query) *query.TermQuery {
	t.Helper()
	tq, ok := q.(*query.TermQuery)
	if !ok {
		t.Fatalf("expected *query.TermQuery, got %T", q)
	}
	return tq
}

// asWildcard type-asserts q to *query.WildcardQuery.
func asWildcard(t *testing.T, q query.Query) *query.WildcardQuery {
	t.Helper()
	wq, ok := q.(*query.WildcardQuery)
	if !ok {
		t.Fatalf("expected *query.WildcardQuery, got %T", q)
	}
	return wq
}

// asMatchPhrase type-asserts q to *query.MatchPhraseQuery.
func asMatchPhrase(t *testing.T, q query.Query) *query.MatchPhraseQuery {
	t.Helper()
	mpq, ok := q.(*query.MatchPhraseQuery)
	if !ok {
		t.Fatalf("expected *query.MatchPhraseQuery, got %T", q)
	}
	return mpq
}

// asNumericRange type-asserts q to *query.NumericRangeQuery.
func asNumericRange(t *testing.T, q query.Query) *query.NumericRangeQuery {
	t.Helper()
	nq, ok := q.(*query.NumericRangeQuery)
	if !ok {
		t.Fatalf("expected *query.NumericRangeQuery, got %T", q)
	}
	return nq
}

// --- Build() tests ---

func Test_build_empty_string(t *testing.T) {
	if _, ok := Build("").(*query.MatchNoneQuery); !ok {
		t.Fatalf("expected *query.MatchNoneQuery, got %T", Build(""))
	}
}

func Test_build_whitespace_only(t *testing.T) {
	if _, ok := Build("   ").(*query.MatchNoneQuery); !ok {
		t.Fatalf("expected *query.MatchNoneQuery, got %T", Build("   "))
	}
}

func Test_build_simple_word_returns_boolean_query(t *testing.T) {
	bq := buildBoolQ(t, "golang")
	clauses := mustClauses(t, bq)
	if len(clauses) != 1 {
		t.Fatalf("expected 1 must clause, got %d", len(clauses))
	}
	// a plain word fans out to title/text MatchQuery + url/domain WildcardQuery
	dq := asDisjunction(t, clauses[0])
	if len(dq.Disjuncts) != 4 {
		t.Fatalf("expected 4 disjuncts (title, text, url, domain), got %d", len(dq.Disjuncts))
	}
}

func Test_build_negated_word(t *testing.T) {
	bq := buildBoolQ(t, "-golang")
	if bq.Must != nil {
		t.Fatalf("negated-only word: expected Must to be nil, got %T", bq.Must)
	}
	nots := mustNotClauses(t, bq)
	if len(nots) != 1 {
		t.Fatalf("expected 1 must_not clause, got %d", len(nots))
	}
	asDisjunction(t, nots[0])
}

func Test_build_quoted_phrase(t *testing.T) {
	bq := buildBoolQ(t, `"hello world"`)
	clauses := mustClauses(t, bq)
	if len(clauses) != 1 {
		t.Fatalf("expected 1 must clause, got %d", len(clauses))
	}
	dq := asDisjunction(t, clauses[0])
	if len(dq.Disjuncts) != 2 {
		t.Fatalf("expected 2 disjuncts (title, text), got %d", len(dq.Disjuncts))
	}
	titlePhrase := asMatchPhrase(t, dq.Disjuncts[0])
	if titlePhrase.MatchPhrase != "hello world" {
		t.Fatalf("expected MatchPhrase %q, got %q", "hello world", titlePhrase.MatchPhrase)
	}
	if titlePhrase.FieldVal != "title" {
		t.Fatalf("expected field %q, got %q", "title", titlePhrase.FieldVal)
	}
}

func Test_build_negated_quoted_phrase(t *testing.T) {
	bq := buildBoolQ(t, `"-hello world"`)
	if bq.Must != nil {
		t.Fatalf("negated quoted phrase: expected Must to be nil, got %T", bq.Must)
	}
	nots := mustNotClauses(t, bq)
	if len(nots) != 1 {
		t.Fatalf("expected 1 must_not clause, got %d", len(nots))
	}
	dq := asDisjunction(t, nots[0])
	mpq := asMatchPhrase(t, dq.Disjuncts[0])
	if mpq.MatchPhrase != "hello world" {
		t.Fatalf("expected MatchPhrase %q, got %q", "hello world", mpq.MatchPhrase)
	}
}

func Test_build_title_field(t *testing.T) {
	bq := buildBoolQ(t, "title:golang")
	clauses := mustClauses(t, bq)
	if len(clauses) != 1 {
		t.Fatalf("expected 1 must clause, got %d", len(clauses))
	}
	mq := asMatch(t, clauses[0])
	if mq.Match != "golang" {
		t.Fatalf("expected Match %q, got %q", "golang", mq.Match)
	}
	if mq.FieldVal != "title" {
		t.Fatalf("expected field %q, got %q", "title", mq.FieldVal)
	}
}

func Test_build_url_field_uses_term_query(t *testing.T) {
	bq := buildBoolQ(t, "url:https://example.com")
	clauses := mustClauses(t, bq)
	if len(clauses) != 1 {
		t.Fatalf("expected 1 must clause, got %d", len(clauses))
	}
	tq := asTerm(t, clauses[0])
	if tq.Term != "https://example.com" {
		t.Fatalf("expected term %q, got %q", "https://example.com", tq.Term)
	}
	if tq.FieldVal != "url" {
		t.Fatalf("expected field %q, got %q", "url", tq.FieldVal)
	}
}

// Test that url:"..." quoted syntax produces a TermQuery (supports spaces in URLs).
func Test_build_url_field_quoted_uses_term_query(t *testing.T) {
	f := "file:///C:/Users/My Documents/notes.txt"
	bq := buildBoolQ(t, fmt.Sprintf(`url:"%s"`, f))
	clauses := mustClauses(t, bq)
	if len(clauses) != 1 {
		t.Fatalf("expected 1 must clause, got %d", len(clauses))
	}
	tq := asTerm(t, clauses[0])
	if tq.Term != f {
		t.Fatalf("expected term %q, got %q", f, tq.Term)
	}
	if tq.FieldVal != "url" {
		t.Fatalf("expected field %q, got %q", "url", tq.FieldVal)
	}
}

func Test_build_domain_field_uses_term_query(t *testing.T) {
	bq := buildBoolQ(t, "domain:example.com")
	clauses := mustClauses(t, bq)
	if len(clauses) != 1 {
		t.Fatalf("expected 1 must clause, got %d", len(clauses))
	}
	tq := asTerm(t, clauses[0])
	if tq.Term != "example.com" {
		t.Fatalf("expected term %q, got %q", "example.com", tq.Term)
	}
	if tq.FieldVal != "domain" {
		t.Fatalf("expected field %q, got %q", "domain", tq.FieldVal)
	}
}

func Test_build_type_web(t *testing.T) {
	bq := buildBoolQ(t, "type:web")
	clauses := mustClauses(t, bq)
	if len(clauses) != 1 {
		t.Fatalf("expected 1 must clause, got %d", len(clauses))
	}
	nq := asNumericRange(t, clauses[0])
	if nq.FieldVal != "type" {
		t.Fatalf("expected field %q, got %q", "type", nq.FieldVal)
	}
}

func Test_build_type_file(t *testing.T) {
	bq := buildBoolQ(t, "type:file")
	clauses := mustClauses(t, bq)
	if len(clauses) != 1 {
		t.Fatalf("expected 1 must clause, got %d", len(clauses))
	}
	nq := asNumericRange(t, clauses[0])
	if nq.FieldVal != "type" {
		t.Fatalf("expected field %q, got %q", "type", nq.FieldVal)
	}
}

func Test_build_unknown_type_falls_through(t *testing.T) {
	// Unknown type value: should not panic, treated as a plain word.
	buildBoolQ(t, "type:nonexistent")
}

func Test_build_user_id(t *testing.T) {
	bq := buildBoolQ(t, "user_id:42")
	clauses := mustClauses(t, bq)
	if len(clauses) != 1 {
		t.Fatalf("expected 1 must clause, got %d", len(clauses))
	}
	nq := asNumericRange(t, clauses[0])
	if nq.FieldVal != "user_id" {
		t.Fatalf("expected field %q, got %q", "user_id", nq.FieldVal)
	}
	if nq.Min == nil || *nq.Min != 42 {
		t.Fatalf("expected Min=42, got %v", nq.Min)
	}
	if nq.Max == nil || *nq.Max != 42 {
		t.Fatalf("expected Max=42, got %v", nq.Max)
	}
}

func Test_build_wildcard_word(t *testing.T) {
	bq := buildBoolQ(t, "go*")
	clauses := mustClauses(t, bq)
	if len(clauses) != 1 {
		t.Fatalf("expected 1 must clause, got %d", len(clauses))
	}
	dq := asDisjunction(t, clauses[0])
	for _, d := range dq.Disjuncts {
		asWildcard(t, d)
	}
}

func Test_build_wildcard_field(t *testing.T) {
	bq := buildBoolQ(t, "title:go*")
	clauses := mustClauses(t, bq)
	if len(clauses) != 1 {
		t.Fatalf("expected 1 must clause, got %d", len(clauses))
	}
	wq := asWildcard(t, clauses[0])
	if wq.Wildcard != "go*" {
		t.Fatalf("expected wildcard %q, got %q", "go*", wq.Wildcard)
	}
	if wq.FieldVal != "title" {
		t.Fatalf("expected field %q, got %q", "title", wq.FieldVal)
	}
}

func Test_build_alternation(t *testing.T) {
	bq := buildBoolQ(t, "(foo|bar)")
	clauses := mustClauses(t, bq)
	if len(clauses) != 1 {
		t.Fatalf("expected 1 must clause, got %d", len(clauses))
	}
	dq := asDisjunction(t, clauses[0])
	if len(dq.Disjuncts) != 2 {
		t.Fatalf("expected 2 disjuncts (foo, bar), got %d", len(dq.Disjuncts))
	}
	// each part is itself a DisjunctionQuery (title/text/url/domain)
	asDisjunction(t, dq.Disjuncts[0])
	asDisjunction(t, dq.Disjuncts[1])
}

// Test for issue #274: field:(a|b) alternation syntax
func Test_build_domain_alternation(t *testing.T) {
	bq := buildBoolQ(t, "domain:(hister.org|docs.hister.org)")
	clauses := mustClauses(t, bq)
	if len(clauses) != 1 {
		t.Fatalf("expected 1 must clause, got %d", len(clauses))
	}
	dq := asDisjunction(t, clauses[0])
	if len(dq.Disjuncts) != 2 {
		t.Fatalf("expected 2 disjuncts, got %d", len(dq.Disjuncts))
	}
	tq0 := asTerm(t, dq.Disjuncts[0])
	if tq0.Term != "hister.org" {
		t.Fatalf("expected term %q, got %q", "hister.org", tq0.Term)
	}
	if tq0.FieldVal != "domain" {
		t.Fatalf("expected field %q, got %q", "domain", tq0.FieldVal)
	}
	tq1 := asTerm(t, dq.Disjuncts[1])
	if tq1.Term != "docs.hister.org" {
		t.Fatalf("expected term %q, got %q", "docs.hister.org", tq1.Term)
	}
	if tq1.FieldVal != "domain" {
		t.Fatalf("expected field %q, got %q", "domain", tq1.FieldVal)
	}
}

func Test_build_domain_alternation_with_wildcard(t *testing.T) {
	bq := buildBoolQ(t, "domain:(hister.org|*.hister.org)")
	clauses := mustClauses(t, bq)
	if len(clauses) != 1 {
		t.Fatalf("expected 1 must clause, got %d", len(clauses))
	}
	dq := asDisjunction(t, clauses[0])
	if len(dq.Disjuncts) != 2 {
		t.Fatalf("expected 2 disjuncts, got %d", len(dq.Disjuncts))
	}
	// First part has no wildcard: TermQuery
	tq := asTerm(t, dq.Disjuncts[0])
	if tq.Term != "hister.org" {
		t.Fatalf("expected term %q, got %q", "hister.org", tq.Term)
	}
	// Second part has wildcard: WildcardQuery
	wq := asWildcard(t, dq.Disjuncts[1])
	if wq.Wildcard != "*.hister.org" {
		t.Fatalf("expected wildcard %q, got %q", "*.hister.org", wq.Wildcard)
	}
	if wq.FieldVal != "domain" {
		t.Fatalf("expected field %q, got %q", "domain", wq.FieldVal)
	}
}

func Test_build_domain_alternation_single_part(t *testing.T) {
	// A single-part group should be equivalent to a plain domain query.
	bq := buildBoolQ(t, "domain:(hister.org)")
	clauses := mustClauses(t, bq)
	if len(clauses) != 1 {
		t.Fatalf("expected 1 must clause, got %d", len(clauses))
	}
	tq := asTerm(t, clauses[0])
	if tq.Term != "hister.org" {
		t.Fatalf("expected term %q, got %q", "hister.org", tq.Term)
	}
	if tq.FieldVal != "domain" {
		t.Fatalf("expected field %q, got %q", "domain", tq.FieldVal)
	}
}

func Test_build_title_alternation(t *testing.T) {
	bq := buildBoolQ(t, "title:(hello|world)")
	clauses := mustClauses(t, bq)
	if len(clauses) != 1 {
		t.Fatalf("expected 1 must clause, got %d", len(clauses))
	}
	dq := asDisjunction(t, clauses[0])
	if len(dq.Disjuncts) != 2 {
		t.Fatalf("expected 2 disjuncts, got %d", len(dq.Disjuncts))
	}
	mq0 := asMatch(t, dq.Disjuncts[0])
	if mq0.Match != "hello" {
		t.Fatalf("expected match %q, got %q", "hello", mq0.Match)
	}
	if mq0.FieldVal != "title" {
		t.Fatalf("expected field %q, got %q", "title", mq0.FieldVal)
	}
	mq1 := asMatch(t, dq.Disjuncts[1])
	if mq1.Match != "world" {
		t.Fatalf("expected match %q, got %q", "world", mq1.Match)
	}
}

func Test_build_multiple_words(t *testing.T) {
	bq := buildBoolQ(t, "foo bar")
	clauses := mustClauses(t, bq)
	if len(clauses) != 2 {
		t.Fatalf("expected 2 must clauses, got %d", len(clauses))
	}
}

func Test_build_multiple_tokens_positive_and_negative(t *testing.T) {
	bq := buildBoolQ(t, "foo -bar")
	musts := mustClauses(t, bq)
	if len(musts) != 1 {
		t.Fatalf("expected 1 must clause, got %d", len(musts))
	}
	nots := mustNotClauses(t, bq)
	if len(nots) != 1 {
		t.Fatalf("expected 1 must_not clause, got %d", len(nots))
	}
}

// --- normalizeFileURL tests ---

func Test_normalizeFileURL(t *testing.T) {
	cases := []struct {
		input     string
		wantExact string
		wantHas   []string
	}{
		{input: "https://example.com/path", wantExact: "https://example.com/path"},
		{input: "*foo", wantExact: "*foo"},
		{input: "/home/user/doc.pdf", wantHas: []string{"file://", "/home/user/doc.pdf"}},
		{input: "docs/file.txt", wantHas: []string{"file://"}},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got := normalizeFileURL(tc.input)
			if tc.wantExact != "" && got != tc.wantExact {
				t.Fatalf("normalizeFileURL(%q): expected %q, got %q", tc.input, tc.wantExact, got)
			}
			for _, sub := range tc.wantHas {
				if !strings.Contains(got, sub) {
					t.Fatalf("normalizeFileURL(%q): expected result to contain %q, got %q", tc.input, sub, got)
				}
			}
		})
	}
}
