package types

import (
	"errors"
	"testing"
)

func TestExtractResultConstructors(t *testing.T) {
	diagnostic := errors.New("not applicable")
	tests := []struct {
		name       string
		result     ExtractResult
		decision   ExtractorDecision
		wantErr    error
		wantAnyErr bool
	}{
		{name: "success", result: Extracted(), decision: ExtractorSuccess},
		{name: "fallback", result: ExtractFallback(diagnostic), decision: ExtractorFallback, wantErr: diagnostic},
		{name: "abort", result: AbortExtraction(diagnostic), decision: ExtractorAbort, wantErr: diagnostic},
		{name: "abort without error", result: AbortExtraction(nil), decision: ExtractorAbort, wantAnyErr: true},
		{name: "zero value", result: ExtractResult{}, decision: ExtractorInvalid},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.result.Decision(); got != test.decision {
				t.Fatalf("Decision() = %v, want %v", got, test.decision)
			}
			if test.wantErr != nil && !errors.Is(test.result.Err(), test.wantErr) {
				t.Fatalf("Err() = %v, want %v", test.result.Err(), test.wantErr)
			}
			if test.wantAnyErr && test.result.Err() == nil {
				t.Fatal("Err() = nil, want an error")
			}
		})
	}
}

func TestPreviewResultConstructors(t *testing.T) {
	diagnostic := errors.New("not applicable")
	response := PreviewResponse{Content: "preview", Template: "custom"}

	success := Previewed(response)
	if success.Decision() != ExtractorSuccess {
		t.Fatalf("Decision() = %v, want %v", success.Decision(), ExtractorSuccess)
	}
	if got := success.Response(); got != response {
		t.Fatalf("Response() = %#v, want %#v", got, response)
	}

	fallback := PreviewFallback(diagnostic)
	if fallback.Decision() != ExtractorFallback || !errors.Is(fallback.Err(), diagnostic) {
		t.Fatalf("PreviewFallback() = decision %v, error %v", fallback.Decision(), fallback.Err())
	}

	abort := AbortPreview(nil)
	if abort.Decision() != ExtractorAbort || abort.Err() == nil {
		t.Fatalf("AbortPreview(nil) = decision %v, error %v", abort.Decision(), abort.Err())
	}

	var zero PreviewResult
	if zero.Decision() != ExtractorInvalid {
		t.Fatalf("zero Decision() = %v, want %v", zero.Decision(), ExtractorInvalid)
	}
}
