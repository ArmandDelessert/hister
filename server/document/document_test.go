package document

import (
	"context"
	"testing"
)

func TestProcessSubmittedHTMLFileDoesNotReadFilesystem(t *testing.T) {
	d := &Document{
		URL:  "file:///path/that/does/not/exist/page.html",
		HTML: "<html><body>saved content</body></html>",
	}

	err := d.ProcessContext(context.Background(), nil, func(_ context.Context, got *Document) error {
		got.Text = "saved content"
		return nil
	})
	if err != nil {
		t.Fatalf("ProcessContext() unexpected error: %v", err)
	}
	if d.Text != "saved content" {
		t.Errorf("Text = %q, want extracted saved content", d.Text)
	}
	if d.Type != Web {
		t.Errorf("Type = %v, want web", d.Type)
	}
	if d.Domain != "local" {
		t.Errorf("Domain = %q, want local", d.Domain)
	}
	if !d.Processed {
		t.Error("document was not marked as processed")
	}
}
