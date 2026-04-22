package querybuilder

import (
	"path/filepath"
	"strconv"
	"strings"

	"github.com/asciimoo/hister/files"
	"github.com/asciimoo/hister/server/types"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/search/query"
)

var weights = map[string]float64{
	"text":     1,
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
	return query.NewBooleanQuery(qs, nil, nqs)
}

func createSimpleQuery(s string) query.Query {
	return bleve.NewQueryStringQuery(s)
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
			if field == "url" || field == "domain" {
				if field == "url" {
					v = normalizeFileURL(v)
				}
				q := bleve.NewTermQuery(v)
				q.SetField(field)
				q.SetBoost(weights[field])
				return q, negated
			}
			q := bleve.NewMatchQuery(v)
			q.SetField(field)
			q.SetBoost(weights[field])
			return q, negated
		}
		titleq := bleve.NewMatchPhraseQuery(v)
		titleq.SetField("title")
		titleq.SetBoost(weights["title"])
		textq := bleve.NewMatchPhraseQuery(v)
		textq.SetField("text")
		textq.SetBoost(weights["text"])
		return bleve.NewDisjunctionQuery(titleq, textq), negated
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
		if v, ok := strings.CutPrefix(t.Value, "user_id:"); ok {
			if uid, err := strconv.ParseUint(v, 10, 64); err == nil {
				f := float64(uid)
				q := bleve.NewNumericRangeInclusiveQuery(&f, &f, new(true), new(true))
				q.SetField("user_id")
				return q, negated
			}
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
			if strings.Contains(v, "*") {
				q := bleve.NewWildcardQuery(strings.ToLower(v))
				q.SetField(field)
				q.SetBoost(weights[field])
				return q, negated
			}
			if field == "url" || field == "domain" {
				if field == "url" {
					v = normalizeFileURL(v)
				}
				q := bleve.NewTermQuery(v)
				q.SetField(field)
				q.SetBoost(weights[field])
				return q, negated
			}
			q := bleve.NewMatchQuery(v)
			q.SetField(field)
			q.SetBoost(weights[field])
			return q, negated
		}

		qs := []query.Query{}
		for _, f := range []string{"title", "text"} {
			if strings.Contains(t.Value, "*") {
				q := bleve.NewWildcardQuery(strings.ToLower(t.Value))
				q.SetField(f)
				q.SetBoost(weights[f])
				qs = append(qs, q)
			} else {
				q := bleve.NewMatchQuery(t.Value)
				q.SetField(f)
				q.SetBoost(weights[f])
				qs = append(qs, q)
			}
		}
		wcq := t.Value
		if !strings.HasPrefix(wcq, "*") {
			wcq = "*" + wcq
		}
		if !strings.HasSuffix(wcq, "*") {
			wcq = wcq + "*"
		}

		urlq := bleve.NewWildcardQuery(wcq)
		urlq.SetField("url")
		urlq.SetBoost(weights["url"])
		qs = append(qs, urlq)

		domainq := bleve.NewWildcardQuery(wcq)
		domainq.SetField("domain")
		domainq.SetBoost(weights["domain"])
		qs = append(qs, domainq)
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
