// SPDX-License-Identifier: AGPL-3.0-or-later

package vectorstore

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
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
