// Package sdk defines the stable contracts used to implement Hister extractors.
package sdk

import (
	"context"

	"github.com/asciimoo/hister/config"
	"github.com/asciimoo/hister/server/document"
	"github.com/asciimoo/hister/server/types"
)

// Document is the document passed through the extractor chain.
type Document = document.Document

// Config contains the enabled state and options for one extractor.
type Config = config.Extractor

// Capabilities declares the phases in which an extractor participates.
type Capabilities = types.ExtractorCapabilities

// Decision describes whether an extractor succeeded, requested a fallback, or
// aborted its phase.
type Decision = types.ExtractorDecision

const (
	ExtractorInvalid  = types.ExtractorInvalid
	ExtractorSuccess  = types.ExtractorSuccess
	ExtractorFallback = types.ExtractorFallback
	ExtractorAbort    = types.ExtractorAbort
)

// ExtractResult is the outcome of extraction or enrichment.
type ExtractResult = types.ExtractResult

// PreviewResult is the outcome of preview rendering.
type PreviewResult = types.PreviewResult

// PreviewResponse contains rendered preview content and its optional template.
type PreviewResponse = types.PreviewResponse

// Extracted reports successful extraction or enrichment.
func Extracted() ExtractResult {
	return types.Extracted()
}

// ExtractFallback requests the next matching content extractor.
func ExtractFallback(err error) ExtractResult {
	return types.ExtractFallback(err)
}

// AbortExtraction stops extraction with a fatal error.
func AbortExtraction(err error) ExtractResult {
	return types.AbortExtraction(err)
}

// Previewed reports successful preview rendering.
func Previewed(response PreviewResponse) PreviewResult {
	return types.Previewed(response)
}

// PreviewFallback requests the next matching preview extractor.
func PreviewFallback(err error) PreviewResult {
	return types.PreviewFallback(err)
}

// AbortPreview stops preview rendering with a fatal error.
func AbortPreview(err error) PreviewResult {
	return types.AbortPreview(err)
}

// Extractor defines a component in the extraction and preview chains.
type Extractor interface {
	Name() string
	Description() string
	Capabilities() Capabilities
	Match(*Document) bool
	Extract(*Document) ExtractResult
	Preview(*Document) PreviewResult
	GetConfig() *Config
	SetConfig(*Config) error
}

// ContextExtractor supports cancellation during extraction.
type ContextExtractor interface {
	ExtractContext(context.Context, *Document) ExtractResult
}

// ContextPreviewer supports cancellation during preview rendering.
type ContextPreviewer interface {
	PreviewContext(context.Context, *Document) PreviewResult
}
