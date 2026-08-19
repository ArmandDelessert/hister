package sdk_test

import (
	"testing"

	"github.com/asciimoo/hister/server/extractor/sdk"
)

type testExtractor struct {
	config *sdk.Config
}

func (e *testExtractor) Name() string { return "Test" }

func (e *testExtractor) Description() string { return "Test extractor" }

func (e *testExtractor) Capabilities() sdk.Capabilities {
	return sdk.Capabilities{Extract: true, Preview: true}
}

func (e *testExtractor) Match(*sdk.Document) bool { return true }

func (e *testExtractor) Extract(*sdk.Document) sdk.ExtractResult {
	return sdk.Extracted()
}

func (e *testExtractor) Preview(*sdk.Document) sdk.PreviewResult {
	return sdk.Previewed(sdk.PreviewResponse{Content: "preview"})
}

func (e *testExtractor) GetConfig() *sdk.Config {
	if e.config == nil {
		return &sdk.Config{Enable: true, Options: map[string]any{}}
	}
	return e.config
}

func (e *testExtractor) SetConfig(config *sdk.Config) error {
	e.config = config
	return nil
}

func TestSDKExtractorContract(t *testing.T) {
	var candidate sdk.Extractor = &testExtractor{}
	doc := &sdk.Document{URL: "https://example.com"}
	if result := candidate.Extract(doc); result.Decision() != sdk.ExtractorSuccess {
		t.Fatalf("extraction decision = %v, want success", result.Decision())
	}
	result := candidate.Preview(doc)
	if result.Decision() != sdk.ExtractorSuccess || result.Response().Content != "preview" {
		t.Fatalf("preview result = %#v, want successful preview", result)
	}
}
