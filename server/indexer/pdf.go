// SPDX-License-Identifier: AGPL-3.0-or-later

package indexer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/rs/zerolog/log"

	"github.com/asciimoo/hister/config"
	"github.com/asciimoo/hister/server/document"

	"github.com/asciimoo/pdf"
)

type pdfFileType struct {
	parser config.PDFParser
}

func (pdfFileType) Match(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".pdf")
}

func (p pdfFileType) Prepare(ctx context.Context, d *document.Document, pdfData []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	var text string
	var err error
	if p.parser.Command == "" {
		text, err = extractPDFText(pdfData)
	} else {
		text, err = extractExternalPDFText(ctx, pdfData, p.parser)
	}
	if err != nil {
		return fmt.Errorf("pdf text extraction: %w", err)
	}
	if strings.TrimSpace(text) == "" {
		return errors.New("pdf contains no extractable text")
	}
	d.Text = text
	d.AddMetadata("type", "pdf")
	return nil
}

// AddPDF extracts plain text from pdfData, stores it in d.Text, then indexes
// the document via AddContext. d.URL and d.Type must already be set by the caller.
// d.Title is set to the last path segment of the URL if it is not already set.
func (i *Indexer) AddPDF(d *document.Document, pdfData []byte, options ...AddOption) error {
	return i.AddPDFContext(context.Background(), d, pdfData, options...)
}

// AddPDFContext extracts and indexes a PDF with the configured parser.
func (i *Indexer) AddPDFContext(ctx context.Context, d *document.Document, pdfData []byte, options ...AddOption) error {
	if err := i.fileHandlers.pdf.Prepare(ctx, d, pdfData); err != nil {
		return err
	}
	return i.AddContext(ctx, d, options...)
}

const maxExternalPDFTextSize = maxExtractedContentSize

var errPDFOutputTooLarge = errors.New("external PDF parser output exceeds 32 MiB")

type pdfTextOutput struct {
	buffer   bytes.Buffer
	cancel   context.CancelFunc
	exceeded bool
}

func (b *pdfTextOutput) Write(p []byte) (int, error) {
	if len(p) > maxExternalPDFTextSize-b.buffer.Len() {
		b.exceeded = true
		b.cancel()
		return 0, errPDFOutputTooLarge
	}
	return b.buffer.Write(p)
}

func extractExternalPDFText(ctx context.Context, pdfData []byte, parser config.PDFParser) (string, error) {
	if parser.Timeout == 0 {
		parser.Timeout = 30
	}
	if err := parser.Validate(); err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(parser.Timeout)*time.Second)
	defer cancel()
	output := &pdfTextOutput{cancel: cancel}
	// The executable and arguments come only from local configuration. PDF data
	// goes through stdin and is never interpreted as arguments or shell code.
	cmd := exec.CommandContext(ctx, parser.Command, parser.Args...)
	cmd.Stdin = bytes.NewReader(pdfData)
	cmd.Stdout = output
	// Stderr may contain document text; leave it connected to the null device.
	cmd.WaitDelay = time.Second
	err := cmd.Run()
	if output.exceeded {
		return "", errPDFOutputTooLarge
	}
	if ctx.Err() != nil {
		return "", fmt.Errorf("external PDF parser: %w", ctx.Err())
	}
	if err != nil {
		return "", fmt.Errorf("external PDF parser: %w", err)
	}
	if !utf8.Valid(output.buffer.Bytes()) {
		return "", errors.New("external PDF parser output is not valid UTF-8")
	}
	return output.buffer.String(), nil
}

// extractPDFText reads all pages of a PDF from pdfData and returns the
// concatenated plain text. It recovers from panics in the underlying PDF
// library and converts them to errors.
func extractPDFText(pdfData []byte) (text string, err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Debug().Msgf("pdf parser panic: %v\n%s", r, debug.Stack())
			err = fmt.Errorf("pdf parser panic: %v", r)
		}
	}()

	r := bytes.NewReader(pdfData)
	pr, err := pdf.NewReader(r, int64(len(pdfData)))
	if err != nil {
		return "", fmt.Errorf("open pdf: %w", err)
	}

	var b strings.Builder
	for page := 1; page <= pr.NumPage(); page++ {
		text, err := pr.Page(page).GetPlainText(nil)
		if err != nil {
			return "", fmt.Errorf("read pdf page %d: %w", page, err)
		}
		if len(text) > maxExtractedContentSize-b.Len() {
			return "", ErrExtractedContentTooLarge
		}
		b.WriteString(text)
	}
	return b.String(), nil
}
