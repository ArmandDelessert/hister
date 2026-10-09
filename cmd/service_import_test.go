package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/asciimoo/hister/client"
	"github.com/asciimoo/hister/config"
	"github.com/asciimoo/hister/server/document"

	"github.com/spf13/cobra"
)

func TestServiceImportBufferDownloadsMissingFavicon(t *testing.T) {
	var received *document.Document
	faviconDownloads := 0
	targetHTTPClient := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		var body struct {
			Ops []struct {
				*document.Document
			} `json:"ops"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			return nil, err
		}
		if len(body.Ops) != 1 {
			return nil, fmt.Errorf("received %d batch operations, want one", len(body.Ops))
		}
		received = body.Ops[0].Document
		return jsonHTTPResponse(req, http.StatusOK, `{"results":[{"status":201}]}`), nil
	})}
	target := client.New("http://hister.example", client.WithHTTPClient(targetHTTPClient), client.WithMaxBatchBodyBytes(40<<20))
	buffer, err := newServiceImportBuffer(
		"test",
		target,
		document.NewNullLanguageDetector(),
		nil,
		serviceImportOptions{
			BatchSize: 1,
			FaviconDownloader: func(d *document.Document) error {
				faviconDownloads++
				d.Favicon = "data:image/png;base64,ZGVmYXVsdCBpY29u"
				return nil
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	d := &document.Document{
		URL:       "https://example.com/article",
		Title:     "Article",
		Text:      "Contents",
		Processed: true,
	}
	buffer.Add(context.Background(), d, nil)

	if received == nil {
		t.Fatal("no document was submitted")
	}
	if received.Favicon != "data:image/png;base64,ZGVmYXVsdCBpY29u" {
		t.Errorf("favicon = %q, want downloaded default icon", received.Favicon)
	}
	if faviconDownloads != 1 {
		t.Errorf("favicon downloads = %d, want one", faviconDownloads)
	}
	if buffer.stats.Imported != 1 || buffer.stats.Errors != 0 {
		t.Errorf("stats = %+v, want one import without errors", buffer.stats)
	}
}

func TestServiceImportRejectsUnsupportedLanguages(t *testing.T) {
	oldCfg := cfg
	t.Cleanup(func() { cfg = oldCfg })
	for _, enabled := range []bool{true, false} {
		cfg = config.CreateDefaultConfig()
		cfg.Indexer.DetectLanguages = enabled
		cfg.Indexer.Languages = []string{"en", "xx"}
		command := &cobra.Command{Use: "import-test"}
		addCommonImportFlags(command)
		runtime, err := newServiceImportRuntime(command)
		if runtime != nil {
			_ = runtime.Close()
		}
		if err == nil || !strings.Contains(err.Error(), "indexer.languages") || !strings.Contains(err.Error(), "xx") {
			t.Errorf("detect_languages=%v: error = %v, want unsupported language xx", enabled, err)
		}
	}
}

func TestServiceImportLanguageSettings(t *testing.T) {
	oldCfg := cfg
	t.Cleanup(func() { cfg = oldCfg })
	for _, tc := range []struct {
		name     string
		enabled  bool
		accuracy string
		want     string
	}{
		{"default accuracy", true, "", "en"},
		{"low accuracy", true, "low", "en"},
		{"high accuracy", true, "high", document.UnknownLanguage},
		{"disabled", false, "low", document.UnknownLanguage},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg = config.CreateDefaultConfig()
			cfg.Indexer.DetectLanguages = tc.enabled
			cfg.Indexer.Languages = []string{"en", "de", "nl", "id"}
			cfg.Indexer.LanguageDetectionAccuracy = tc.accuracy
			command := &cobra.Command{Use: "import-test"}
			addCommonImportFlags(command)
			runtime, err := newServiceImportRuntime(command)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := runtime.Close(); err != nil {
					t.Errorf("close service import runtime: %v", err)
				}
			})
			for text, want := range map[string]string{"hello world": tc.want, "bonjour": document.UnknownLanguage} {
				d := &document.Document{URL: "https://example.com/article"}
				fetched := &document.Document{URL: d.URL, Text: text}
				if err := applyServiceContent(context.Background(), d, fetched, "", "", runtime.languageDetector); err != nil {
					t.Fatal(err)
				}
				if d.Language != want {
					t.Errorf("imported %q: language = %q, want %q", text, d.Language, want)
				}
			}
		})
	}
}

func TestApplyServiceContentPreservesFetchedFavicon(t *testing.T) {
	d := &document.Document{URL: "https://example.com/article"}
	fetched := &document.Document{
		URL:     "https://example.com/article",
		HTML:    `<html><head><title>Article</title></head><body><main><p>Downloaded contents.</p></main></body></html>`,
		Favicon: "data:image/png;base64,bGlua2VkIGljb24=",
	}
	if err := applyServiceContent(context.Background(), d, fetched, "", "", document.NewNullLanguageDetector()); err != nil {
		t.Fatal(err)
	}
	if d.Favicon != fetched.Favicon {
		t.Errorf("favicon = %q, want fetched favicon %q", d.Favicon, fetched.Favicon)
	}
}

type serviceLanguageDetectorFunc func(string) string

func (f serviceLanguageDetectorFunc) DetectLanguage(text string) string { return f(text) }

func TestApplyServiceContentReusesFreshLanguageDetection(t *testing.T) {
	const body = "The contents of the fetched page."
	const prefix = "Eine deutsche Zusammenfassung."
	const combined = prefix + "\n\n" + body
	for _, tc := range []struct {
		name             string
		prefix           string
		alreadyProcessed bool
		bodyLanguage     string
		wantText         string
		wantLanguage     string
		wantInputs       []string
	}{
		{"unchanged text", "", false, "en", body, "en", []string{body}},
		{"blank prefix", " \n\t", false, "en", body, "en", []string{body}},
		{"duplicate prefix", body, false, "en", body, "en", []string{body}},
		{"unknown language", "", false, document.UnknownLanguage, body, document.UnknownLanguage, []string{body}},
		{"changed text", prefix, false, "en", combined, "de", []string{body, combined}},
		{"previously processed text", "", true, "en", body, "en", []string{body}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var inputs []string
			detector := serviceLanguageDetectorFunc(func(text string) string {
				inputs = append(inputs, text)
				if text == combined {
					return "de"
				}
				return tc.bodyLanguage
			})
			d := &document.Document{URL: "https://example.com/article", Language: "fr"}
			fetched := &document.Document{
				URL: d.URL, Text: body, Language: "fr", Processed: tc.alreadyProcessed,
			}
			if err := applyServiceContent(context.Background(), d, fetched, tc.prefix, "", detector); err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(inputs, tc.wantInputs) {
				t.Errorf("detected texts = %q, want %q", inputs, tc.wantInputs)
			}
			if d.Text != tc.wantText || d.Language != tc.wantLanguage {
				t.Errorf("imported text = %q, language = %q; want %q, %q", d.Text, d.Language, tc.wantText, tc.wantLanguage)
			}
		})
	}
}
