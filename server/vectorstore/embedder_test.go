// SPDX-License-Identifier: AGPL-3.0-or-later

package vectorstore

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/asciimoo/hister/config"
)

func newTestEmbedder(endpoint string) *Embedder {
	return NewEmbedder(&config.SemanticSearch{
		EmbeddingEndpoint: endpoint,
		EmbeddingModel:    "test-model",
		Dimensions:        3,
		MaxContextLength:  128,
	})
}

func writeEmbeddingResponse(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(embeddingResponse{
		Data: []struct {
			Embedding []float64 `json:"embedding"`
		}{
			{Embedding: []float64{1, 2, 3}},
		},
	})
}

func TestDocumentEmbeddingInputsSeparateMetadataAndBody(t *testing.T) {
	embedder := newTestEmbedder("")
	embedder.documentPrefix = "passage: "
	documentContext := DocumentContext{
		Title:       "Semantic Search",
		URL:         "https://example.com/search",
		Type:        "article",
		Language:    "en",
		Author:      "Ada Example",
		Description: "How semantic retrieval works.",
		Keywords:    "search, embeddings",
	}

	inputs := embedder.documentEmbeddingInputs("Body content about vector retrieval.", documentContext)
	if len(inputs) != 2 {
		t.Fatalf("expected metadata and body inputs, got %d: %#v", len(inputs), inputs)
	}

	metadata := inputs[0]
	for _, expected := range []string{
		"passage: document:\n",
		"title: Semantic Search",
		"type: article",
		"language: en",
		"author: Ada Example",
		"description: How semantic retrieval works.",
		"keywords: search, embeddings",
		"url: https://example.com/search",
	} {
		if !strings.Contains(metadata.embeddingText, expected) {
			t.Errorf("metadata input does not contain %q: %q", expected, metadata.embeddingText)
		}
	}
	if metadata.chunkText == "" {
		t.Error("metadata chunk text must be available for matched chunk previews")
	}
	if strings.Contains(metadata.embeddingText, "\ndomain:") {
		t.Errorf("metadata input unexpectedly contains a separate domain field: %q", metadata.embeddingText)
	}

	body := inputs[1]
	for _, expected := range []string{
		"passage: title: Semantic Search",
		"language: en",
		"content:\nBody content about vector retrieval.",
	} {
		if !strings.Contains(body.embeddingText, expected) {
			t.Errorf("body input does not contain %q: %q", expected, body.embeddingText)
		}
	}
	for _, repeatedMetadata := range []string{"domain:", "type:", "author:", "description:", "keywords:", "url:"} {
		if strings.Contains(body.embeddingText, repeatedMetadata) {
			t.Errorf("body input unexpectedly repeats %q metadata: %q", repeatedMetadata, body.embeddingText)
		}
	}
	if body.chunkText != "Body content about vector retrieval." {
		t.Errorf("body chunk text = %q", body.chunkText)
	}
	for i, input := range inputs {
		if got := len(tokenize(input.embeddingText)); got > embedder.maxContextLength {
			t.Errorf("input %d exceeds context limit: got %d, limit %d", i, got, embedder.maxContextLength)
		}
	}
}

func TestDocumentEmbeddingInputsSupportMetadataOnlyDocument(t *testing.T) {
	embedder := newTestEmbedder("")
	inputs := embedder.documentEmbeddingInputs("", DocumentContext{Title: "Saved title"})
	if len(inputs) != 1 {
		t.Fatalf("expected one metadata input, got %d: %#v", len(inputs), inputs)
	}
	if !strings.Contains(inputs[0].embeddingText, "title: Saved title") {
		t.Errorf("metadata input does not contain title: %q", inputs[0].embeddingText)
	}
}

func TestDocumentEmbeddingInputsEmptyDocument(t *testing.T) {
	embedder := newTestEmbedder("")
	if inputs := embedder.documentEmbeddingInputs("", DocumentContext{}); inputs != nil {
		t.Fatalf("expected no inputs, got %#v", inputs)
	}
}

func TestFormatEmbeddingFieldsRespectsBudget(t *testing.T) {
	fields := []embeddingField{
		{name: "title", value: "A title with\nextra whitespace"},
		{name: "description", value: strings.Repeat("word ", 50)},
	}
	formatted := formatEmbeddingFields(fields, 12)
	if got := len(tokenize(formatted)); got > 12 {
		t.Fatalf("formatted fields exceed budget: got %d tokens in %q", got, formatted)
	}
	if strings.Contains(formatted, "\nextra") {
		t.Errorf("field value whitespace was not normalized: %q", formatted)
	}
}

func TestEmbedRetriesTransientStatus(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if attempts.Add(1) == 1 {
			http.Error(w, "warming up", http.StatusServiceUnavailable)
			return
		}
		writeEmbeddingResponse(w)
	}))
	defer srv.Close()

	vec, err := newTestEmbedder(srv.URL).Embed(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Embed returned error: %v", err)
	}
	if got := attempts.Load(); got != 2 {
		t.Fatalf("attempts = %d, want 2", got)
	}
	if len(vec) != 3 || vec[0] != 1 || vec[1] != 2 || vec[2] != 3 {
		t.Fatalf("unexpected vector: %#v", vec)
	}
}

func TestEmbedDoesNotRetryNonTransientStatus(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		http.Error(w, "bad input", http.StatusBadRequest)
	}))
	defer srv.Close()

	_, err := newTestEmbedder(srv.URL).Embed(context.Background(), "hello")
	if err == nil {
		t.Fatal("Embed returned nil error")
	}
	if got := attempts.Load(); got != 1 {
		t.Fatalf("attempts = %d, want 1", got)
	}
}

func TestEmbedRequestUsesContext(t *testing.T) {
	requestStarted := make(chan struct{})
	unblockHandler := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(requestStarted)
		select {
		case <-r.Context().Done():
		case <-unblockHandler:
		}
	}))
	defer func() {
		close(unblockHandler)
		srv.Close()
	}()

	errc := make(chan error, 1)
	go func() {
		_, err := newTestEmbedder(srv.URL).Embed(ctx, "hello")
		errc <- err
	}()

	select {
	case <-requestStarted:
	case <-time.After(time.Second):
		t.Fatal("embedding request did not reach test server")
	}

	cancel()

	select {
	case err := <-errc:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Embed error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Embed did not return after context cancellation")
	}
}
