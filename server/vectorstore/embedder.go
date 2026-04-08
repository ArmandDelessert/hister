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
// into float32 vectors.
type Embedder struct {
	endpoint string
	model    string
	client   *http.Client
}

// NewEmbedder creates an Embedder from the semantic search config.
func NewEmbedder(cfg *config.SemanticSearch) *Embedder {
	return &Embedder{
		endpoint: cfg.EmbeddingEndpoint,
		model:    cfg.EmbeddingModel,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type embeddingRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type embeddingBatchRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embeddingResponse struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
}

// Embed converts a single text into a float32 vector.
func (e *Embedder) Embed(text string) (_ []float32, err error) {
	body, err := json.Marshal(embeddingRequest{
		Model: e.model,
		Input: text,
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
	if len(result.Data) == 0 || len(result.Data[0].Embedding) == 0 {
		return nil, fmt.Errorf("embedding response contained no data")
	}

	return toFloat32(result.Data[0].Embedding), nil
}

// EmbedBatch converts multiple texts in a single request.
func (e *Embedder) EmbedBatch(texts []string) (_ [][]float32, err error) {
	body, err := json.Marshal(embeddingBatchRequest{
		Model: e.model,
		Input: texts,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal batch embedding request: %w", err)
	}

	req, err := http.NewRequest("POST", e.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create batch embedding request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("batch embedding request failed: %w", err)
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
		return nil, fmt.Errorf("decode batch embedding response: %w", err)
	}

	vectors := make([][]float32, len(result.Data))
	for i, d := range result.Data {
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
