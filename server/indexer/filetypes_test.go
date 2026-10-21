package indexer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mmonterroca/docxgo/v2/domain"

	"gopkg.in/yaml.v3"

	"github.com/asciimoo/hister/config"
	"github.com/asciimoo/hister/files"
	"github.com/asciimoo/hister/server/document"
	"github.com/asciimoo/hister/server/testutil"
)

func TestPDFParserProcess(t *testing.T) {
	separator := slices.Index(os.Args, "--pdf-parser-test")
	if separator < 0 {
		return
	}
	args := os.Args[separator+1:]
	data, err := io.ReadAll(os.Stdin)
	if err != nil || !strings.HasPrefix(string(data), "%PDF-") {
		os.Exit(2)
	}
	switch args[0] {
	case "success":
		if _, err := fmt.Printf("External PDF text: %s\n%s", data, strings.Join(args[1:], "|")); err != nil {
			os.Exit(2)
		}
	case "fail":
		if _, err := fmt.Fprint(os.Stdout, "partial output"); err != nil {
			os.Exit(2)
		}
		if _, err := fmt.Fprint(os.Stderr, "private document diagnostic"); err != nil {
			os.Exit(2)
		}
		os.Exit(3)
	case "empty":
		if _, err := fmt.Fprint(os.Stdout, " \n"); err != nil {
			os.Exit(2)
		}
	case "invalid":
		_, _ = os.Stdout.Write([]byte{0xff})
	case "timeout":
		time.Sleep(10 * time.Second)
	case "large":
		chunk := strings.Repeat("x", 1<<20)
		for range 33 {
			if _, err := fmt.Fprint(os.Stdout, chunk); err != nil {
				os.Exit(2)
			}
		}
	}
	os.Exit(0)
}

func externalPDFTestConfig(t *testing.T, mode string) *config.Config {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cfg := testutil.Config(t)
	cfg.Indexer.DetectLanguages = false
	data, err := yaml.Marshal(map[string]any{"indexer": map[string]any{"pdf_parser": map[string]any{
		"command": executable,
		"args":    []string{"-test.run=^TestPDFParserProcess$", "--", "--pdf-parser-test", mode, "literal ; $(echo unsafe)"},
		"timeout": 1,
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestConfiguredFileHandlers(t *testing.T) {
	cfg := externalPDFTestConfig(t, "success")
	handlers := NewFileHandlers(cfg.Indexer)
	// Each set of handlers keeps its own configuration, including its arguments.
	cfg.Indexer.PDFParser.Args[len(cfg.Indexer.PDFParser.Args)-1] = "changed after construction"
	other := NewFileHandlers(cfg.Indexer)
	const input = "%PDF-1.4 test bytes"
	for _, tc := range []struct {
		name     string
		handlers *FileHandlers
		argument string
	}{
		{"original", handlers, "literal ; $(echo unsafe)"},
		{"other", other, "changed after construction"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := &document.Document{}
			if err := tc.handlers.Prepare(context.Background(), "paper.PDF", d, []byte(input)); err != nil {
				t.Fatal(err)
			}
			if d.Text != "External PDF text: "+input+"\n"+tc.argument || d.Metadata["type"] != "pdf" {
				t.Fatalf("unexpected parsed PDF: %#v", d)
			}
		})
	}
	plain := &document.Document{}
	if err := handlers.Prepare(context.Background(), "notes.txt", plain, []byte("plain text")); err != nil || plain.Text != "plain text" {
		t.Fatalf("non-PDF handler changed: document = %#v, error = %v", plain, err)
	}
	if err := PrepareFileContent("paper.pdf", &document.Document{}, []byte(input)); err == nil {
		t.Fatal("configured PDF parser leaked into the default handlers")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := handlers.Prepare(ctx, "paper.pdf", &document.Document{}, []byte(input)); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context cancellation", err)
	}
}

func TestExternalPDFParserUploadsAndWatchedFiles(t *testing.T) {
	cfg := externalPDFTestConfig(t, "success")
	dir := t.TempDir()
	cfg.Indexer.Directories = []*config.Directory{{Path: dir}}
	idx := newTestIndexer(t, cfg)
	t.Cleanup(idx.Close)
	const input = "%PDF-1.4 test bytes"
	d := &document.Document{URL: "https://example.com/paper.pdf"}
	if err := idx.AddPDF(d, []byte(input)); err != nil {
		t.Fatalf("external parser was not used for upload: %v", err)
	}
	if d.Text != "External PDF text: "+input+"\nliteral ; $(echo unsafe)" || d.Metadata["type"] != "pdf" {
		t.Fatalf("unexpected parsed PDF: %#v", d)
	}
	path := testutil.WriteFile(t, dir, "paper.pdf", []byte(input))
	if err := idx.IndexFile(path, 0); err != nil {
		t.Fatal(err)
	}
	if got := idx.GetByURLAndUser(files.PathToFileURL(path), 0); got == nil || got.Text != d.Text {
		t.Fatalf("watched PDF was not parsed externally: %#v", got)
	}
	if err := idx.Reindex(&config.Rules{}, false, false, cfg.Indexer.KeepStopwords, cfg.Indexer.Directories); err != nil {
		t.Fatal(err)
	}
	if got := idx.GetByURLAndUser(files.PathToFileURL(path), 0); got == nil || got.Text != d.Text {
		t.Fatalf("reindex lost PDF parser settings: %#v", got)
	}
	if err := idx.AddPDF(&document.Document{URL: "https://example.com/after.pdf"}, []byte(input)); err != nil {
		t.Fatalf("PDF parser settings were lost after reindex: %v", err)
	}
	builtin := newTestIndexer(t, testutil.Config(t))
	t.Cleanup(builtin.Close)
	if err := builtin.AddPDF(&document.Document{URL: "https://example.com/builtin.pdf"}, []byte(input)); err == nil {
		t.Fatal("parser configuration leaked to another indexer")
	}
}

func TestExternalPDFParserRejectsFailures(t *testing.T) {
	for _, tc := range []struct{ mode, want string }{
		{"fail", "exit status"},
		{"empty", "no extractable text"},
		{"invalid", "UTF"},
		{"timeout", "deadline exceeded"},
		{"large", "output exceeds"},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			idx := newTestIndexer(t, externalPDFTestConfig(t, tc.mode))
			t.Cleanup(idx.Close)
			d := &document.Document{URL: "https://example.com/fail.pdf"}
			err := idx.AddPDF(d, []byte("%PDF-1.4 test bytes"))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
			if strings.Contains(err.Error(), "private document diagnostic") || d.Text != "" {
				t.Fatal("failed parser exposed or stored partial output")
			}
			if idx.GetByURLAndUser(d.URL, 0) != nil {
				t.Fatal("failed PDF was indexed")
			}
		})
	}
}

func TestExternalPDFParserCancellationAndMissingCommand(t *testing.T) {
	cfg := externalPDFTestConfig(t, "timeout")
	idx := newTestIndexer(t, cfg)
	t.Cleanup(idx.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := idx.AddPDFContext(ctx, &document.Document{URL: "https://example.com/canceled.pdf"}, []byte("%PDF-1.4 test bytes")); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want the caller's deadline", err)
	}
	cfg.Indexer.PDFParser.Command = t.TempDir() + "/missing-parser"
	cfg.App.Directory = t.TempDir()
	missing := newTestIndexer(t, cfg)
	t.Cleanup(missing.Close)
	if err := missing.AddPDF(&document.Document{URL: "https://example.com/missing.pdf"}, []byte("%PDF-1.4 test bytes")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("error = %v, want missing executable error", err)
	}
}

func TestFileTypeHandlerForPath(t *testing.T) {
	tests := []struct {
		path string
		want any
	}{
		{path: "paper.pdf", want: &pdfFileType{}},
		{path: "paper.PDF", want: &pdfFileType{}},
		{path: "paper.docx", want: docxFileType{}},
		{path: "paper.DOCX", want: docxFileType{}},
		{path: "notes.md", want: markdownFileType{}},
		{path: "notes.markdown", want: markdownFileType{}},
		{path: "notes.org", want: orgFileType{}},
		{path: "page.html", want: htmlFileType{}},
		{path: "page.HTML", want: htmlFileType{}},
		{path: "page.htm", want: htmlFileType{}},
		{path: "page.HTM", want: htmlFileType{}},
		{path: "notes.txt", want: plainTextFileType{}},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := defaultFileHandlers.forPath(tt.path)
			if reflect.TypeOf(got) != reflect.TypeOf(tt.want) {
				t.Fatalf("handler = %T, want %T", got, tt.want)
			}
		})
	}
}

func TestPrepareFileContent(t *testing.T) {
	docxData := buildDocx(t, []docxTestParagraph{
		{style: domain.StyleIDTitle, text: "Document title"},
		{text: "Document body"},
	})
	tests := []struct {
		name         string
		path         string
		content      []byte
		wantTitle    string
		wantText     []string
		wantHTML     string
		wantMetadata string
	}{
		{
			name:     "plain text",
			path:     "notes.txt",
			content:  []byte("Plain text body"),
			wantText: []string{"Plain text body"},
		},
		{
			name:      "HTML",
			path:      "page.html",
			content:   []byte("<html><head><title>Saved page</title></head><body><p>Saved body</p></body></html>"),
			wantTitle: "Saved page",
			wantText:  []string{"Saved body"},
			wantHTML:  "<p>Saved body</p>",
		},
		{
			name:         "markdown",
			path:         "notes.md",
			content:      []byte("# Markdown title\n\nMarkdown body"),
			wantTitle:    "Markdown title",
			wantText:     []string{"Markdown title", "Markdown body"},
			wantHTML:     "<h1",
			wantMetadata: "markdown",
		},
		{
			name:         "org",
			path:         "notes.org",
			content:      []byte("#+TITLE: Org title\n\n* Org heading\n\nOrg body"),
			wantTitle:    "Org title",
			wantText:     []string{"Org heading", "Org body"},
			wantHTML:     "<h2",
			wantMetadata: "org",
		},
		{
			name:         "docx",
			path:         "notes.docx",
			content:      docxData,
			wantTitle:    "Document title",
			wantText:     []string{"Document title", "Document body"},
			wantMetadata: "docx",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &document.Document{
				URL:    "file:///client/notes",
				Type:   document.Local,
				UserID: 42,
				Label:  "notes",
			}
			if err := PrepareFileContent(tt.path, d, tt.content); err != nil {
				t.Fatalf("PrepareFileContent failed: %v", err)
			}
			if d.URL != "file:///client/notes" || d.Type != document.Local || d.UserID != 42 || d.Label != "notes" {
				t.Fatalf("document identity changed during preparation: %#v", d)
			}
			if d.Processed {
				t.Fatal("document was marked processed during content preparation")
			}
			if d.Title != tt.wantTitle {
				t.Fatalf("title = %q, want %q", d.Title, tt.wantTitle)
			}
			for _, want := range tt.wantText {
				if !strings.Contains(d.Text, want) {
					t.Errorf("text %q does not contain %q", d.Text, want)
				}
			}
			if tt.wantHTML != "" && !strings.Contains(d.HTML, tt.wantHTML) {
				t.Errorf("HTML %q does not contain %q", d.HTML, tt.wantHTML)
			}
			if tt.wantMetadata != "" && d.Metadata["type"] != tt.wantMetadata {
				t.Errorf("metadata type = %v, want %q", d.Metadata["type"], tt.wantMetadata)
			}
		})
	}
}

func TestPrepareFileContentRejectsInvalidContent(t *testing.T) {
	t.Run("binary plain text", func(t *testing.T) {
		err := PrepareFileContent("notes.txt", &document.Document{}, []byte{0xff})
		if !errors.Is(err, ErrBinaryFile) {
			t.Fatalf("error = %v, want %v", err, ErrBinaryFile)
		}
	})

	t.Run("invalid PDF", func(t *testing.T) {
		err := PrepareFileContent("notes.pdf", &document.Document{}, []byte("not a PDF"))
		if err == nil || !strings.Contains(err.Error(), "pdf text extraction") {
			t.Fatalf("error = %v, want PDF extraction error", err)
		}
	})
}
