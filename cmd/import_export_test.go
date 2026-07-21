package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/asciimoo/hister/client"
	"github.com/asciimoo/hister/server/document"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestExpandImportInputsExpandsDirectory(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{
		"b.html",
		"a.json",
		"c.7z",
		"d.htm",
		"ignored.txt",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("test"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "subdir", "nested.html"), []byte("test"), 0o600); err != nil {
		t.Fatal(err)
	}

	inputs, err := expandImportInputs([]string{"first.json", dir, "last"})
	if err != nil {
		t.Fatal(err)
	}

	want := []string{
		"first.json",
		filepath.Join(dir, "a.json"),
		filepath.Join(dir, "b.html"),
		filepath.Join(dir, "c.7z"),
		filepath.Join(dir, "d.htm"),
		"last",
	}
	if !reflect.DeepEqual(inputs, want) {
		t.Fatalf("expandImportInputs() = %#v, want %#v", inputs, want)
	}
}

func TestIsSupportedImportInput(t *testing.T) {
	tests := map[string]bool{
		"export.json": true,
		"backup.7z":   true,
		"page.html":   true,
		"page.htm":    true,
		"page.HTML":   true,
		"notes.txt":   false,
		"README":      false,
	}

	for input, want := range tests {
		if got := isSupportedImportInput(input); got != want {
			t.Fatalf("isSupportedImportInput(%q) = %v, want %v", input, got, want)
		}
	}
}

func TestImportJSONFileUsesConfiguredBatchSize(t *testing.T) {
	var batchSizes []int
	var receivedLabel string
	var receivedMetadata map[string]any
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/api/batch" {
			t.Errorf("request path = %q, want /api/batch", r.URL.Path)
		}
		var req struct {
			Ops []struct {
				Op        string         `json:"op"`
				Label     string         `json:"label"`
				Metadata  map[string]any `json:"metadata"`
				Processed bool           `json:"processed"`
			} `json:"ops"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return nil, fmt.Errorf("decode request: %w", err)
		}
		batchSizes = append(batchSizes, len(req.Ops))
		if len(batchSizes) == 1 && len(req.Ops) > 0 {
			receivedLabel = req.Ops[0].Label
			receivedMetadata = req.Ops[0].Metadata
		}
		for _, op := range req.Ops {
			if !op.Processed {
				t.Error("imported JSON document was not marked as processed")
			}
		}
		results := make([]map[string]any, len(req.Ops))
		for i, op := range req.Ops {
			if op.Op != "add" {
				t.Errorf("operation = %q, want add", op.Op)
			}
			results[i] = map[string]any{"status": http.StatusCreated}
		}
		var response bytes.Buffer
		if err := json.NewEncoder(&response).Encode(map[string]any{"results": results}); err != nil {
			return nil, fmt.Errorf("encode response: %w", err)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(bytes.NewReader(response.Bytes())),
			Request:    r,
		}, nil
	})}

	var input strings.Builder
	for i := range 23 {
		doc := &document.Document{
			URL:   fmt.Sprintf("https://example.com/%d", i),
			Title: fmt.Sprintf("Document %d", i),
		}
		if i == 0 {
			doc.Label = "reference"
			doc.Metadata = map[string]any{"source": "export"}
		}
		line, err := json.Marshal(doc)
		if err != nil {
			t.Fatal(err)
		}
		input.Write(line)
		input.WriteByte('\n')
	}
	inputFile := filepath.Join(t.TempDir(), "export.json")
	if err := os.WriteFile(inputFile, []byte(input.String()), 0o600); err != nil {
		t.Fatal(err)
	}

	imported, skipped, errCount := importJSONFile(client.New("http://hister.test", client.WithHTTPClient(httpClient)), inputFile, false, 0, 0, 10)
	if imported != 23 || skipped != 0 || errCount != 0 {
		t.Fatalf("importJSONFile() = (%d, %d, %d), want (23, 0, 0)", imported, skipped, errCount)
	}
	if want := []int{10, 10, 3}; !reflect.DeepEqual(batchSizes, want) {
		t.Fatalf("batch sizes = %v, want %v", batchSizes, want)
	}
	if receivedLabel != "reference" {
		t.Fatalf("label = %q, want reference", receivedLabel)
	}
	if receivedMetadata["source"] != "export" {
		t.Fatalf("metadata source = %v, want export", receivedMetadata["source"])
	}
}

func TestImportBatchSizeDefault(t *testing.T) {
	batchSize, err := importFileCmd.Flags().GetInt("batch-size")
	if err != nil {
		t.Fatal(err)
	}
	if batchSize != 10 {
		t.Fatalf("batch size default = %d, want 10", batchSize)
	}
}

func TestImportCommandHierarchy(t *testing.T) {
	tests := map[string]*cobra.Command{
		"file":       importFileCmd,
		"browser":    importBrowserCmd,
		"linkwarden": importLinkwardenCmd,
	}
	for name, want := range tests {
		got, _, err := importCmd.Find([]string{name})
		if err != nil {
			t.Fatalf("import %s command lookup failed: %v", name, err)
		}
		if got != want {
			t.Fatalf("import %s command = %q, want %q", name, got.Name(), want.Name())
		}
	}
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "import-browser" {
			t.Fatal("legacy import-browser command remains registered at the root")
		}
	}
}

func TestImportSubcommandFlagOwnership(t *testing.T) {
	for _, name := range []string{"batch-size", "start-date", "end-date", "skip-existing", "global", "user-id"} {
		for _, importCommand := range []*cobra.Command{importFileCmd, importLinkwardenCmd} {
			if importCommand.Flags().Lookup(name) == nil {
				t.Errorf("import %s is missing --%s", importCommand.Name(), name)
			}
		}
		if importBrowserCmd.Flags().Lookup(name) != nil {
			t.Errorf("import browser unexpectedly has --%s", name)
		}
	}
	for _, name := range []string{"min-visit", "backend", "backend-option", "header", "cookie"} {
		if importBrowserCmd.Flags().Lookup(name) == nil {
			t.Errorf("import browser is missing --%s", name)
		}
		if importFileCmd.Flags().Lookup(name) != nil {
			t.Errorf("import file unexpectedly has --%s", name)
		}
		if importLinkwardenCmd.Flags().Lookup(name) != nil {
			t.Errorf("import linkwarden unexpectedly has --%s", name)
		}
	}
	if importLinkwardenCmd.Flags().Lookup("api-token") == nil {
		t.Error("import linkwarden is missing --api-token")
	}
	if importLinkwardenCmd.Flags().Lookup("source-token") != nil {
		t.Error("import linkwarden still has the old --source-token flag")
	}
	for _, importCommand := range []*cobra.Command{importFileCmd, importBrowserCmd} {
		if importCommand.Flags().Lookup("api-token") != nil {
			t.Errorf("import %s unexpectedly has --api-token", importCommand.Name())
		}
	}
}
