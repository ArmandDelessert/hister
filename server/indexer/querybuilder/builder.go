package querybuilder

import (
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/asciimoo/hister/files"
	"github.com/asciimoo/hister/server/types"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/search/query"
)

var weights = map[string]float64{
	"text":     1,
	"label":    1,
	"language": 1,
	"url":      4,
	"domain":   8,
	"title":    12,
}

func Build(s string) query.Query {
	if strings.TrimSpace(s) == "" {
		return query.NewMatchNoneQuery()
	}

	qt, err := Tokenize(s)
	if err != nil {
		return createSimpleQuery(s)
	}

	qs := []query.Query{}
	nqs := []query.Query{}

	for _, t := range qt {
		q, negated := getTokenQuery(t)
		if negated {
			nqs = append(nqs, q)
		} else {
			qs = append(qs, q)
		}
	}
	if len(qt) > 1 && !anyFieldSpecific(qt) {
		// create a full phrase query from the query string to get exact matches for the full query
		pq := createCombinedMatchQuery(s, 2)
		qs = []query.Query{
			bleve.NewDisjunctionQuery(
				bleve.NewConjunctionQuery(qs...),
				pq,
			),
		}
	}
	q := query.NewBooleanQuery(qs, nil, nqs)
	if len(qt) == 1 && !isFieldSpecific(qt[0]) {
		// prioritize base url matches if there is only one non field specific search term for easier retrieval of websites.
		uq := bleve.NewRegexpQuery(fmt.Sprintf("https?://(www\\.)?%s[^/]*/", regexp.QuoteMeta(strings.ToLower(qt[0].Value))))
		uq.SetField("url")
		uq.SetBoost(100)
		q.AddShould(uq)
	}
	return q
}

// anyFieldSpecific returns true when at least one token in qt is an explicit
// field query (e.g. url:..., domain:..., user_id:...).  When any token is
// field-specific, adding a full-string phrase query would produce semantically
// wrong results, so the caller skips the phrase-wrapping optimisation.
func anyFieldSpecific(qt []Token) bool {
	return slices.ContainsFunc(qt, isFieldSpecific)
}

// isFieldSpecific reports whether t carries an explicit field: prefix and
// therefore represents a targeted field query rather than free text.
func isFieldSpecific(t Token) bool {
	v := t.Value
	switch t.Type {
	case TokenWord, TokenQuoted:
		v = strings.TrimPrefix(v, "-")
		if _, ok := strings.CutPrefix(v, "type:"); ok {
			return true
		}
		if _, ok := visitCountFilterValue(v); ok {
			return true
		}
		if _, ok := strings.CutPrefix(v, "user_id:"); ok {
			return true
		}
		if strings.HasPrefix(v, "metadata.") && strings.Contains(v, ":") {
			return true
		}
		for f := range weights {
			if strings.HasPrefix(v, f+":") {
				return true
			}
		}
	}
	return false
}

func createSimpleQuery(s string) query.Query {
	return bleve.NewQueryStringQuery(s)
}

func createCombinedMatchQuery(s string, boost float64) query.Query {
	q := bleve.NewDisjunctionQuery(
		createMatchPhraseQuery(s, boost*10),
		createMatchQuery(s, boost),
	)
	q.SetBoost(boost)
	return q
}

// Matches all terms from the query (AND semantics per field)
func createMatchQuery(s string, boost float64) query.Query {
	tiq := bleve.NewMatchQuery(s)
	tiq.SetField("title")
	tiq.SetBoost(weights["title"])
	tiq.Operator = query.MatchQueryOperatorAnd
	teq := bleve.NewMatchQuery(s)
	teq.SetField("text")
	teq.SetBoost(weights["text"])
	teq.Operator = query.MatchQueryOperatorAnd
	q := bleve.NewDisjunctionQuery(tiq, teq)
	q.SetBoost(boost)
	return q
}

// buildFieldQuery creates the appropriate Bleve query for a named field and value.
// It uses WildcardQuery when v contains '*', TermQuery for url/domain fields,
// and MatchQuery for all other fields.
func buildFieldQuery(field, v string) query.Query {
	if strings.Contains(v, "*") {
		q := bleve.NewWildcardQuery(strings.ToLower(v))
		q.SetField(field)
		q.SetBoost(weights[field])
		return q
	}
	if field == "url" || field == "domain" {
		if field == "url" {
			v = normalizeFileURL(v)
		}
		q := bleve.NewTermQuery(v)
		q.SetField(field)
		q.SetBoost(weights[field])
		return q
	}
	q := bleve.NewMatchQuery(v)
	q.SetField(field)
	q.SetBoost(weights[field])
	return q
}

func visitCountFilterValue(v string) (string, bool) {
	if value, ok := strings.CutPrefix(v, "visits:"); ok {
		return value, true
	}
	if value, ok := strings.CutPrefix(v, "add_count:"); ok {
		return value, true
	}
	return "", false
}

func buildVisitCountQuery(v string) (query.Query, bool) {
	parseBound := func(s string) (*float64, bool) {
		if s == "" {
			return nil, true
		}
		n, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil, false
		}
		return &n, true
	}

	var min, max *float64
	if before, after, ok := strings.Cut(v, ".."); ok {
		var valid bool
		min, valid = parseBound(before)
		if !valid {
			return nil, false
		}
		max, valid = parseBound(after)
		if !valid {
			return nil, false
		}
	} else {
		n, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return nil, false
		}
		min = &n
		max = &n
	}
	if min == nil && max == nil {
		return nil, false
	}
	q := bleve.NewNumericRangeInclusiveQuery(min, max, new(true), new(true))
	q.SetField("add_count")
	return q, true
}

// Matches exact phrases without stopwords
func createMatchPhraseQuery(s string, boost float64) query.Query {
	tiq := bleve.NewMatchPhraseQuery(s)
	tiq.SetField("title")
	tiq.SetBoost(weights["title"])
	teq := bleve.NewMatchPhraseQuery(s)
	teq.SetField("text")
	teq.SetBoost(weights["text"])
	q := bleve.NewDisjunctionQuery(tiq, teq)
	q.SetBoost(boost)
	return q
}

func getTokenQuery(t Token) (query.Query, bool) {
	negated := false
	switch t.Type {
	case TokenQuoted:
		v := t.Value
		if strings.HasPrefix(v, "-") {
			negated = true
			v = v[1:]
		}
		var field string
		for f := range weights {
			if strings.HasPrefix(t.Value, f+":") {
				field = f
				break
			}
		}
		if field != "" {
			v := t.Value[len(field)+1:]
			if strings.HasPrefix(v, "-") && len(v) > 1 {
				negated = true
				v = v[1:]
			}
			return buildFieldQuery(field, v), negated
		}
		return createMatchPhraseQuery(v, 1), negated
	case TokenWord:
		if strings.HasPrefix(t.Value, "-") && len(t.Value) > 1 {
			negated = true
			t.Value = t.Value[1:]
		}
		var field string
		if v, ok := strings.CutPrefix(t.Value, "type:"); ok {
			if t, ok := types.DocTypeNames[v]; ok {
				from := float64(t)
				to := float64(t + 1)
				q := bleve.NewNumericRangeQuery(&from, &to)
				q.SetField("type")
				return q, negated
			}
		}
		if v, ok := visitCountFilterValue(t.Value); ok {
			if q, ok := buildVisitCountQuery(v); ok {
				return q, negated
			}
		}
		if v, ok := strings.CutPrefix(t.Value, "user_id:"); ok {
			if uid, err := strconv.ParseUint(v, 10, 64); err == nil {
				f := float64(uid)
				q := bleve.NewNumericRangeInclusiveQuery(&f, &f, new(true), new(true))
				q.SetField("user_id")
				return q, negated
			}
		}
		if strings.HasPrefix(t.Value, "metadata.") && strings.Contains(t.Value, ":") {
			field := strings.Split(t.Value, ":")[0]
			v := strings.TrimPrefix(t.Value, field+":")
			q := bleve.NewTermQuery(v)
			q.SetField(field)
			return q, negated
		}
		for f := range weights {
			if strings.HasPrefix(t.Value, f+":") {
				field = f
				break
			}
		}
		if field != "" {
			v := t.Value[len(field)+1:]
			if strings.HasPrefix(v, "-") && len(v) > 1 {
				negated = true
				v = v[1:]
			}
			// Handle parenthesized alternation groups like field:(a|b|c)
			if strings.HasPrefix(v, "(") && strings.HasSuffix(v, ")") {
				inner := v[1 : len(v)-1]
				parts, err := parseAlternationParts(inner)
				if err == nil {
					if len(parts) > 1 {
						qs := []query.Query{}
						for _, p := range parts {
							partToken := Token{Type: TokenWord, Value: field + ":" + p.Value}
							q, _ := getTokenQuery(partToken)
							qs = append(qs, q)
						}
						return bleve.NewDisjunctionQuery(qs...), negated
					}
					if len(parts) == 1 {
						v = parts[0].Value
					}
				}
			}
			return buildFieldQuery(field, v), negated
		}

		qs := []query.Query{}
		for _, f := range []string{"title", "text"} {
			qs = append(qs, buildFieldQuery(f, t.Value))
		}
		wcq := t.Value
		if !strings.HasPrefix(wcq, "*") {
			wcq = "*" + wcq
		}
		if !strings.HasSuffix(wcq, "*") {
			wcq = wcq + "*"
		}
		qs = append(qs, buildFieldQuery("url", wcq))
		qs = append(qs, buildFieldQuery("domain", wcq))
		return bleve.NewDisjunctionQuery(qs...), negated

	case TokenAlternation:
		qs := []query.Query{}
		for _, p := range t.Parts {
			r, _ := getTokenQuery(p)
			qs = append(qs, r)
		}
		return bleve.NewDisjunctionQuery(qs...), negated
	}
	return bleve.NewQueryStringQuery(t.Value), negated
}

func normalizeFileURL(v string) string {
	if strings.HasPrefix(v, "*") {
		return v
	}
	if strings.Contains(v, "://") {
		return v
	}
	if abs, err := filepath.Abs(v); err == nil {
		v = abs
	}
	return files.PathToFileURL(v)
}
