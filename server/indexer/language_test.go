package indexer

import (
	"fmt"
	"testing"

	"github.com/asciimoo/hister/config"
	"github.com/asciimoo/hister/server/document"
	"github.com/asciimoo/hister/server/testutil"
)

func TestRestrictedLanguagesKeepExistingIndexesSearchable(t *testing.T) {
	cfg := testutil.Config(t)
	idx := newTestIndexer(t, cfg)
	d := &document.Document{
		URL: "https://example.com/french", Text: "Un document français.",
		Language: "fr", Processed: true,
	}
	if err := idx.Add(d); err != nil {
		idx.Close()
		t.Fatal(err)
	}
	idx.Close()

	cfg.Indexer.Languages = []string{"en", "de"}
	cfg.Indexer.LanguageDetectionAccuracy = "low"
	idx = newTestIndexer(t, cfg)
	defer idx.Close()
	res, err := idx.Search(&Query{Text: "language:fr"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Documents) != 1 || res.Documents[0].URL != d.URL {
		t.Fatalf("existing French document disappeared from search: %+v", res.Documents)
	}
}

func TestRestrictedLanguageSettingsSurviveReindex(t *testing.T) {
	cfg := testutil.Config(t)
	cfg.Indexer.Languages = []string{"en", "de", "nl", "id"}
	cfg.Indexer.LanguageDetectionAccuracy = "low"
	idx := newTestIndexer(t, cfg)
	defer idx.Close()

	// "hello world" distinguishes the accuracy modes at the restricted
	// confidence threshold. "bonjour" verifies the restricted candidate list
	// and fallback to the default index instead of a French language index.
	for round := range 2 {
		for text, want := range map[string]string{"hello world": "en", "bonjour": document.UnknownLanguage} {
			d := &document.Document{
				URL: fmt.Sprintf("https://example.com/%d/%s", round, want), Text: text,
			}
			if err := idx.Add(d); err != nil {
				t.Fatal(err)
			}
			if d.Language != want {
				t.Errorf("round %d: %q detected as %q, want %q", round, text, d.Language, want)
			}
		}
		if round == 0 {
			if err := idx.Reindex(&config.Rules{}, false, true, false, nil); err != nil {
				t.Fatal(err)
			}
			for _, language := range []string{"en", document.UnknownLanguage} {
				url := "https://example.com/0/" + language
				stored := idx.GetByURLAndUser(url, 0)
				if stored == nil || stored.Language != language {
					t.Fatalf("reindexed %s: document = %+v", url, stored)
				}
			}
		}
	}
	for language, name := range map[string]string{"en": indexNameForLanguage("en"), document.UnknownLanguage: defaultIndexerName} {
		count, err := idx.indexers[name].DocCount()
		if err != nil {
			t.Fatal(err)
		}
		if count != 2 {
			t.Errorf("%s index count = %d, want 2", language, count)
		}
	}
}
