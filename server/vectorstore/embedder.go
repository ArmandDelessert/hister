// SPDX-License-Identifier: AGPL-3.0-or-later

package vectorstore

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/asciimoo/hister/config"
)

// Embedder calls an OpenAI-compatible /v1/embeddings endpoint to convert text
// into float32 vectors. It also handles text chunking for long documents.
type Embedder struct {
	endpoint         string
	model            string
	dimensions       int
	client           *http.Client
	maxContextLength int
	chunkOverlap     int
}

// NewEmbedder creates an Embedder from the semantic search config.
func NewEmbedder(cfg *config.SemanticSearch) *Embedder {
	return &Embedder{
		endpoint:         cfg.EmbeddingEndpoint,
		model:            cfg.EmbeddingModel,
		dimensions:       cfg.Dimensions,
		maxContextLength: cfg.MaxContextLength,
		chunkOverlap:     cfg.ChunkOverlap,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type embeddingRequest struct {
	Model string `json:"model"`
	Input any    `json:"input"` // string for single, []string for batch
}

type embeddingResponse struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
}

// doEmbeddingRequest sends an embedding request to the endpoint and returns the
// parsed response. input is either a string (single) or []string (batch).
func (e *Embedder) doEmbeddingRequest(input any) (_ *embeddingResponse, err error) {
	body, err := json.Marshal(embeddingRequest{
		Model: e.model,
		Input: input,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal embedding request: %w", err)
	}

	req, err := http.NewRequest("POST", e.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create embedding request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embedding request failed: %w", err)
	}
	defer func() {
		if cerr := resp.Body.Close(); err == nil {
			err = cerr
		}
	}()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("embedding endpoint returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result embeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode embedding response: %w", err)
	}
	return &result, nil
}

// Embed converts a single text into a float32 vector.
func (e *Embedder) Embed(text string) ([]float32, error) {
	result, err := e.doEmbeddingRequest(text)
	if err != nil {
		return nil, err
	}
	if len(result.Data) == 0 || len(result.Data[0].Embedding) == 0 {
		return nil, fmt.Errorf("embedding response contained no data")
	}
	if got := len(result.Data[0].Embedding); e.dimensions > 0 && got != e.dimensions {
		return nil, fmt.Errorf("embedding dimension mismatch: expected %d, got %d", e.dimensions, got)
	}
	return toFloat32(result.Data[0].Embedding), nil
}

// EmbedBatch converts multiple texts in a single request.
func (e *Embedder) EmbedBatch(texts []string) ([][]float32, error) {
	result, err := e.doEmbeddingRequest(texts)
	if err != nil {
		return nil, err
	}
	vectors := make([][]float32, len(result.Data))
	for i, d := range result.Data {
		if got := len(d.Embedding); e.dimensions > 0 && got != e.dimensions {
			return nil, fmt.Errorf("embedding dimension mismatch at index %d: expected %d, got %d", i, e.dimensions, got)
		}
		vectors[i] = toFloat32(d.Embedding)
	}
	return vectors, nil
}

func toFloat32(f64 []float64) []float32 {
	f32 := make([]float32, len(f64))
	for i, v := range f64 {
		f32[i] = float32(v)
	}
	return f32
}

// ChunkAndEmbed splits text into overlapping chunks, batch-embeds them, and
// returns Chunk values ready for storage. Returns nil (not an error) when the
// text is empty.
func (e *Embedder) ChunkAndEmbed(text string) ([]Chunk, error) {
	textChunks := ChunkText(text, e.maxContextLength, e.chunkOverlap)
	if len(textChunks) == 0 {
		return nil, nil
	}

	texts := make([]string, len(textChunks))
	for i, tc := range textChunks {
		texts[i] = tc.Text
	}

	vectors, err := e.EmbedBatch(texts)
	if err != nil {
		return nil, err
	}
	if len(vectors) != len(textChunks) {
		return nil, fmt.Errorf("embedding count mismatch: expected %d, got %d", len(textChunks), len(vectors))
	}

	chunks := make([]Chunk, len(textChunks))
	for i := range textChunks {
		chunks[i] = Chunk{
			Index:     i,
			Text:      textChunks[i].Text,
			Embedding: vectors[i],
		}
	}
	return chunks, nil
}
