package types

import "errors"

// DocType represents the type of an indexed document.
type DocType int

const (
	Web DocType = iota
	Local
)

var DocTypeNames = map[string]DocType{
	"web":   Web,
	"file":  Local,
	"local": Local,
}

// String returns the human-readable name of the DocType.
func (t DocType) String() string {
	switch t {
	case Web:
		return "web"
	case Local:
		return "local"
	default:
		return "unknown"
	}
}

// PreviewResponse holds the result of a document preview operation.
// Template should be left blank to use the default template.
type PreviewResponse struct {
	Content  string
	Template string
}

// ExtractorCapabilities declares the roles an extractor participates in.
// Enrichers annotate a document without selecting its searchable body.
// Content extractors compete to produce the searchable title and text.
// Preview extractors compete to render the stored document.
type ExtractorCapabilities struct {
	Enrich  bool `json:"enrich"`
	Extract bool `json:"extract"`
	Preview bool `json:"preview"`
}

// ExtractorDecision tells the caller whether an extractor succeeded, declined
// the document, or encountered a fatal error. The zero value is invalid so an
// uninitialized result cannot be mistaken for success.
type ExtractorDecision int

const (
	ExtractorInvalid ExtractorDecision = iota
	ExtractorSuccess
	ExtractorFallback
	ExtractorAbort
)

var errExtractorAbortedWithoutError = errors.New("extractor aborted without an error")

// ExtractResult is the outcome of searchable content extraction or metadata
// enrichment. Its fields are private so callers must use a valid constructor.
type ExtractResult struct {
	decision ExtractorDecision
	err      error
}

// Extracted reports successful extraction or enrichment.
func Extracted() ExtractResult {
	return ExtractResult{decision: ExtractorSuccess}
}

// ExtractFallback reports that the next matching extractor should be tried.
// err may describe why this extractor declined the document.
func ExtractFallback(err error) ExtractResult {
	return ExtractResult{decision: ExtractorFallback, err: err}
}

// AbortExtraction reports a fatal extraction error.
func AbortExtraction(err error) ExtractResult {
	if err == nil {
		err = errExtractorAbortedWithoutError
	}
	return ExtractResult{decision: ExtractorAbort, err: err}
}

func (r ExtractResult) Decision() ExtractorDecision {
	return r.decision
}

func (r ExtractResult) Err() error {
	return r.err
}

// Unpack returns the validated decision and its optional diagnostic error.
func (r ExtractResult) Unpack() (ExtractorDecision, error) {
	return r.decision, r.err
}

// PreviewResult is the outcome of an extractor preview attempt. Its fields are
// private so a successful result always contains an explicit response and an
// aborted result always contains an error.
type PreviewResult struct {
	decision ExtractorDecision
	response PreviewResponse
	err      error
}

// Previewed reports successful preview rendering.
func Previewed(response PreviewResponse) PreviewResult {
	return PreviewResult{decision: ExtractorSuccess, response: response}
}

// PreviewFallback reports that the next matching preview extractor should be
// tried. err may describe why this extractor declined the document.
func PreviewFallback(err error) PreviewResult {
	return PreviewResult{decision: ExtractorFallback, err: err}
}

// AbortPreview reports a fatal preview error.
func AbortPreview(err error) PreviewResult {
	if err == nil {
		err = errExtractorAbortedWithoutError
	}
	return PreviewResult{decision: ExtractorAbort, err: err}
}

func (r PreviewResult) Decision() ExtractorDecision {
	return r.decision
}

func (r PreviewResult) Response() PreviewResponse {
	return r.response
}

func (r PreviewResult) Err() error {
	return r.err
}

// Unpack returns the validated response, decision, and optional diagnostic
// error.
func (r PreviewResult) Unpack() (PreviewResponse, ExtractorDecision, error) {
	return r.response, r.decision, r.err
}
