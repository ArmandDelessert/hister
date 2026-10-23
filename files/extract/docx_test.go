// SPDX-License-Identifier: AGPL-3.0-or-later

package extract

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"

	docx "github.com/mmonterroca/docxgo/v2"
	"github.com/mmonterroca/docxgo/v2/domain"
)

func TestExtractDocxTextLimitsUncompressedContent(t *testing.T) {
	original := buildDocx(t, []docxTestParagraph{{text: "Document body"}})
	reader, err := zip.NewReader(bytes.NewReader(original), int64(len(original)))
	if err != nil {
		t.Fatal(err)
	}
	var originalSize int
	for _, entry := range reader.File {
		originalSize += int(entry.UncompressedSize64)
	}
	for _, tc := range []struct {
		name     string
		sizes    []int
		tooLarge bool
	}{
		{name: "at limit", sizes: []int{(64 << 20) - originalSize}},
		{name: "single entry", sizes: []int{65 << 20}, tooLarge: true},
		{name: "aggregate", sizes: []int{33 << 20, 33 << 20}, tooLarge: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			writer := zip.NewWriter(&buf)
			for _, entry := range reader.File {
				if err := writer.Copy(entry); err != nil {
					t.Fatal(err)
				}
			}
			chunk := strings.Repeat("x", 1<<20)
			for i, size := range tc.sizes {
				entry, err := writer.Create(fmt.Sprintf("extra/%d.bin", i))
				if err != nil {
					t.Fatal(err)
				}
				for size > 0 {
					n := min(size, len(chunk))
					if _, err := io.WriteString(entry, chunk[:n]); err != nil {
						t.Fatal(err)
					}
					size -= n
				}
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			text, _, err := extractDocxText(buf.Bytes())
			if tc.tooLarge {
				if err == nil || !strings.Contains(err.Error(), "uncompressed") {
					t.Fatalf("oversized DOCX returned text %q and error %v", text, err)
				}
			} else if err != nil || text != "Document body" {
				t.Fatalf("DOCX at limit returned text %q and error %v", text, err)
			}
		})
	}
}

func TestExtractDocxTextUsesStyledTitleAndParagraphs(t *testing.T) {
	docxData := buildDocx(t, []docxTestParagraph{
		{style: domain.StyleIDTitle, text: "Document title"},
		{text: "First body paragraph."},
		{text: "Second body paragraph."},
	})

	text, title, err := extractDocxText(docxData)
	if err != nil {
		t.Fatalf("extractDocxText failed: %v", err)
	}
	if title != "Document title" {
		t.Fatalf("title = %q, want %q", title, "Document title")
	}
	for _, want := range []string{
		"Document title",
		"First body paragraph.",
		"Second body paragraph.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("text %q missing %q", text, want)
		}
	}
}

func TestExtractDocxTextFallsBackToStyledTitle(t *testing.T) {
	docxData := buildDocx(t, []docxTestParagraph{
		{style: domain.StyleIDHeading1, text: "Styled heading"},
		{text: "Body paragraph."},
	})

	text, title, err := extractDocxText(docxData)
	if err != nil {
		t.Fatalf("extractDocxText failed: %v", err)
	}
	if title != "Styled heading" {
		t.Fatalf("title = %q, want %q", title, "Styled heading")
	}
	if text != "Styled heading\n\nBody paragraph." {
		t.Fatalf("text = %q", text)
	}
}

func TestExtractDocxTextFallsBackToFirstParagraph(t *testing.T) {
	docxData := buildDocx(t, []docxTestParagraph{
		{text: "First paragraph."},
		{text: "Second paragraph."},
	})

	text, title, err := extractDocxText(docxData)
	if err != nil {
		t.Fatalf("extractDocxText failed: %v", err)
	}
	if title != "First paragraph." {
		t.Fatalf("title = %q, want %q", title, "First paragraph.")
	}
	if text != "First paragraph.\n\nSecond paragraph." {
		t.Fatalf("text = %q", text)
	}
}

type docxTestParagraph struct {
	style string
	text  string
}

func buildDocx(t *testing.T, paragraphs []docxTestParagraph) []byte {
	t.Helper()

	doc := docx.NewDocument()
	for _, paragraph := range paragraphs {
		p, err := doc.AddParagraph()
		if err != nil {
			t.Fatalf("AddParagraph failed: %v", err)
		}
		if paragraph.style != "" {
			if err := p.SetStyle(paragraph.style); err != nil {
				t.Fatalf("SetStyle failed: %v", err)
			}
		}
		run, err := p.AddRun()
		if err != nil {
			t.Fatalf("AddRun failed: %v", err)
		}
		if err := run.SetText(paragraph.text); err != nil {
			t.Fatalf("SetText failed: %v", err)
		}
	}

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}
	return buf.Bytes()
}
