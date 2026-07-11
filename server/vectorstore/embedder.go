// SPDX-License-Identifier: AGPL-3.0-or-later

package vectorstore

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/asciimoo/hister/config"
)

// Embedder calls an OpenAI-compatible /v1/embeddings endpoint to convert text
// into float32 vectors. It also handles text chunking for long documents.
type Embedder struct {
	endpoint         string
	model            string
	apiKey           string
	headers          map[string]string
	dimensions       int
	client           *http.Client
	maxContextLength int
	chunkOverlap     int
	queryPrefix      string
	documentPrefix   string
	sem              chan struct{} // nil means unlimited concurrency
}

// DocumentContext contains stable metadata used to contextualize document
// embeddings. Description and keywords are embedded only in the dedicated
// metadata vector, not repeated in every body chunk.
type DocumentContext struct {
	Title       string
	URL         string
	Type        string
	Language    string
	Author      string
	Description string
	Keywords    string
}

type embeddingField struct {
	name  string
	value string
}

type documentEmbeddingInput struct {
	embeddingText string
	chunkText     string
}

const embeddingMaxAttempts = 3

// NewEmbedder creates an Embedder from the semantic search config.
func NewEmbedder(cfg *config.SemanticSearch) *Embedder {
	var sem chan struct{}
	if cfg.MaxEmbeddingConcurrency > 0 {
		sem = make(chan struct{}, cfg.MaxEmbeddingConcurrency)
	}
	return &Embedder{
		endpoint:         cfg.EmbeddingEndpoint,
		model:            cfg.EmbeddingModel,
		apiKey:           cfg.APIKey,
		headers:          cfg.Headers,
		dimensions:       cfg.Dimensions,
		maxContextLength: cfg.MaxContextLength,
		chunkOverlap:     cfg.ChunkOverlap,
		queryPrefix:      cfg.QueryPrefix,
		documentPrefix:   cfg.DocumentPrefix,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		sem: sem,
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

type embeddingStatusError struct {
	statusCode int
	body       string
}

func (e *embeddingStatusError) Error() string {
	return fmt.Sprintf("embedding endpoint returned %d: %s", e.statusCode, e.body)
}

func (e *embeddingStatusError) transient() bool {
	switch e.statusCode {
	case http.StatusRequestTimeout,
		http.StatusTooEarly,
		http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func embeddingRetryDelay(attempt int) time.Duration {
	return time.Duration(1<<attempt) * 250 * time.Millisecond
}

func shouldRetryEmbeddingError(ctx context.Context, err error) bool {
	if err == nil {
		return false
	}
	if ctx != nil && ctx.Err() != nil {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return false
	}

	var statusErr *embeddingStatusError
	if errors.As(err, &statusErr) {
		return statusErr.transient()
	}

	var urlErr *url.Error
	return errors.As(err, &urlErr)
}

// doEmbeddingRequestOnce sends one embedding request to the endpoint and returns
// the parsed response. input is either a string (single) or []string (batch).
func (e *Embedder) doEmbeddingRequestOnce(ctx context.Context, input any) (_ *embeddingResponse, err error) {
	body, err := json.Marshal(embeddingRequest{
		Model: e.model,
		Input: input,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal embedding request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", e.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create embedding request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if e.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+e.apiKey)
	}
	for k, v := range e.headers {
		req.Header.Set(k, v)
	}

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
		return nil, &embeddingStatusError{statusCode: resp.StatusCode, body: string(respBody)}
	}

	var result embeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode embedding response: %w", err)
	}
	return &result, nil
}

// doEmbeddingRequest sends an embedding request, retrying transient endpoint or
// network failures while respecting the caller's context.
func (e *Embedder) doEmbeddingRequest(ctx context.Context, input any) (*embeddingResponse, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if e.sem != nil {
		select {
		case e.sem <- struct{}{}:
			defer func() { <-e.sem }()
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	var err error
	for attempt := range embeddingMaxAttempts {
		var result *embeddingResponse
		result, err = e.doEmbeddingRequestOnce(ctx, input)
		if err == nil {
			return result, nil
		}
		if attempt == embeddingMaxAttempts-1 || !shouldRetryEmbeddingError(ctx, err) {
			return nil, err
		}

		timer := time.NewTimer(embeddingRetryDelay(attempt))
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	return nil, err
}

// Embed converts a single text into a float32 vector.
func (e *Embedder) Embed(ctx context.Context, text string) ([]float32, error) {
	result, err := e.doEmbeddingRequest(ctx, text)
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

// EmbedQuery embeds a search query, prepending the configured query prefix
// (e.g. "search_query: ") when set. Many embedding models (BGE, E5, Nomic,
// GTE) produce better recall when queries and documents use distinct prefixes.
func (e *Embedder) EmbedQuery(ctx context.Context, text string) ([]float32, error) {
	return e.Embed(ctx, e.queryPrefix+text)
}

// EmbedBatch converts multiple texts in a single request.
func (e *Embedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	result, err := e.doEmbeddingRequest(ctx, texts)
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

func cleanEmbeddingField(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

// formatEmbeddingFields formats as many complete metadata fields as fit in
// tokenBudget. The last field may be shortened when part of it still fits.
func formatEmbeddingFields(fields []embeddingField, tokenBudget int) string {
	if tokenBudget <= 0 {
		return ""
	}
	lines := make([]string, 0, len(fields))
	remaining := tokenBudget
	for _, field := range fields {
		value := cleanEmbeddingField(field.value)
		if value == "" {
			continue
		}
		line := field.name + ": " + value
		lineTokens := len(tokenize(line))
		if lineTokens > remaining {
			labelTokens := len(tokenize(field.name + ":"))
			valueBudget := remaining - labelTokens
			if valueBudget <= 0 {
				break
			}
			valueTokens := tokenize(value)
			if len(valueTokens) > valueBudget {
				valueTokens = valueTokens[:valueBudget]
			}
			line = field.name + ": " + strings.Join(valueTokens, " ")
			lineTokens = len(tokenize(line))
		}
		lines = append(lines, line)
		remaining -= lineTokens
		if remaining <= 0 {
			break
		}
	}
	return strings.Join(lines, "\n")
}

func fullDocumentFields(d DocumentContext) []embeddingField {
	return []embeddingField{
		{name: "title", value: d.Title},
		{name: "type", value: d.Type},
		{name: "language", value: d.Language},
		{name: "author", value: d.Author},
		{name: "description", value: d.Description},
		{name: "keywords", value: d.Keywords},
		{name: "url", value: d.URL},
	}
}

func bodyDocumentFields(d DocumentContext) []embeddingField {
	return []embeddingField{
		{name: "title", value: d.Title},
		{name: "language", value: d.Language},
	}
}

func (e *Embedder) documentEmbeddingInputs(text string, d DocumentContext) []documentEmbeddingInput {
	var inputs []documentEmbeddingInput
	metadataLabel := e.documentPrefix + "document:\n"
	metadataBudget := e.maxContextLength - len(tokenize(metadataLabel))
	metadata := formatEmbeddingFields(fullDocumentFields(d), metadataBudget)
	if metadata != "" {
		inputs = append(inputs, documentEmbeddingInput{
			embeddingText: metadataLabel + metadata,
			chunkText:     metadata,
		})
	}

	bodyFieldBudget := max(1, e.maxContextLength/4)
	bodyContext := formatEmbeddingFields(bodyDocumentFields(d), bodyFieldBudget)
	bodyHeader := e.documentPrefix
	if bodyContext != "" {
		bodyHeader += bodyContext + "\n"
	}
	bodyHeader += "content:\n"
	contentLimit := max(1, e.maxContextLength-len(tokenize(bodyHeader)))
	textChunks := ChunkText(text, contentLimit, e.chunkOverlap)
	for _, chunk := range textChunks {
		inputs = append(inputs, documentEmbeddingInput{
			embeddingText: bodyHeader + chunk.Text,
			chunkText:     chunk.Text,
		})
	}
	return inputs
}

// ChunkAndEmbed creates a dedicated metadata embedding and separate structured
// body chunk embeddings, then returns them ready for storage. Returns nil when
// both document context and text are empty.
func (e *Embedder) ChunkAndEmbed(ctx context.Context, text string, d DocumentContext) ([]Chunk, error) {
	inputs := e.documentEmbeddingInputs(text, d)
	if len(inputs) == 0 {
		return nil, nil
	}

	texts := make([]string, len(inputs))
	for i, input := range inputs {
		texts[i] = input.embeddingText
	}

	vectors, err := e.EmbedBatch(ctx, texts)
	if err != nil {
		return nil, err
	}
	if len(vectors) != len(inputs) {
		return nil, fmt.Errorf("embedding count mismatch: expected %d, got %d", len(inputs), len(vectors))
	}

	chunks := make([]Chunk, len(inputs))
	for i := range inputs {
		chunks[i] = Chunk{
			Index:     i,
			Text:      inputs[i].chunkText,
			Embedding: vectors[i],
		}
	}
	return chunks, nil
}
