// SPDX-License-Identifier: AGPL-3.0-or-later

package extract

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"strings"

	docx "github.com/mmonterroca/docxgo/v2"
	"github.com/mmonterroca/docxgo/v2/domain"
	"github.com/rs/zerolog/log"
)

type docxFileType struct{}

const maxDOCXUncompressedSize = 64 << 20

func (docxFileType) Match(path string) bool {
	return hasExtension(path, ".docx")
}

func (docxFileType) Prepare(_ context.Context, d *Result, docxData []byte) error {
	text, title, err := extractDocxText(docxData)
	if err != nil {
		return fmt.Errorf("docx text extraction: %w", err)
	}
	if strings.TrimSpace(text) == "" {
		return errors.New("docx contains no extractable text")
	}
	d.Text = text
	if d.Title == "" {
		d.Title = title
	}
	d.Format = DOCX
	return nil
}

func extractDocxText(docxData []byte) (text string, title string, err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Debug().Msgf("docx parser panic: %v\n%s", r, debug.Stack())
			err = fmt.Errorf("docx parser panic: %v", r)
		}
	}()

	archive, err := zip.NewReader(bytes.NewReader(docxData), int64(len(docxData)))
	if err != nil {
		return "", "", fmt.Errorf("open docx archive: %w", err)
	}
	// The parser loads every ZIP entry, including media, into memory. Check the
	// total before handing it the archive; archive/zip also rejects entries that
	// expand beyond their declared size while the parser reads them.
	var uncompressed uint64
	for _, entry := range archive.File {
		if entry.UncompressedSize64 > maxDOCXUncompressedSize-uncompressed {
			return "", "", errors.New("docx uncompressed content exceeds 64 MiB")
		}
		uncompressed += entry.UncompressedSize64
	}

	doc, err := docx.OpenDocumentFromBytes(docxData)
	if err != nil {
		return "", "", fmt.Errorf("open docx: %w", err)
	}

	if meta := doc.Metadata(); meta != nil {
		title = strings.TrimSpace(meta.Title)
		if len(title) > MaxContentSize {
			return "", "", ErrExtractedContentTooLarge
		}
	}

	paragraphs := doc.Paragraphs()
	textParts := make([]string, 0, len(paragraphs))
	textSize := 0
	for _, p := range paragraphs {
		paragraphText := strings.TrimSpace(p.Text())
		if paragraphText == "" {
			continue
		}
		if title == "" && isDocxTitleParagraph(p) {
			title = paragraphText
		}
		partSize := len(paragraphText)
		if len(textParts) > 0 {
			partSize += 2 // Paragraph separator.
		}
		if partSize > MaxContentSize-textSize {
			return "", "", ErrExtractedContentTooLarge
		}
		textSize += partSize
		textParts = append(textParts, paragraphText)
	}
	if title == "" && len(textParts) > 0 {
		title = textParts[0]
	}
	return strings.Join(textParts, "\n\n"), title, nil
}

func isDocxTitleParagraph(p domain.Paragraph) bool {
	style := p.Style()
	if style == nil {
		return false
	}
	styleID := strings.ToLower(strings.ReplaceAll(style.ID(), " ", ""))
	styleName := strings.ToLower(strings.ReplaceAll(style.Name(), " ", ""))
	for _, titleStyle := range []string{
		strings.ToLower(domain.StyleIDTitle),
		strings.ToLower(domain.StyleIDHeading1),
		"title",
		"heading1",
	} {
		if styleID == titleStyle || styleName == titleStyle {
			return true
		}
	}
	return false
}
