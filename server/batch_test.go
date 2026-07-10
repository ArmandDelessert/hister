package server

import (
	"encoding/json"
	"testing"
)

func TestBatchRequestAcceptsCompleteDocument(t *testing.T) {
	var req batchRequest
	data := []byte(`{"ops":[{"op":"add","url":"https://example.com","title":"Example","label":"reference","metadata":{"source":"export"}}]}`)
	if err := json.Unmarshal(data, &req); err != nil {
		t.Fatal(err)
	}
	if len(req.Ops) != 1 {
		t.Fatalf("operation count = %d, want 1", len(req.Ops))
	}
	op := req.Ops[0]
	if op.Op != batchOpAdd || op.URL != "https://example.com" || op.Title != "Example" {
		t.Fatalf("unexpected operation: %#v", op)
	}
	if op.Label != "reference" {
		t.Fatalf("label = %q, want reference", op.Label)
	}
	if op.Metadata["source"] != "export" {
		t.Fatalf("metadata source = %v, want export", op.Metadata["source"])
	}
}
