package indexer

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/asciimoo/hister/config"
	"github.com/asciimoo/hister/server/document"
)

type fileTypeHandler interface {
	Match(path string) bool
	Prepare(context.Context, *document.Document, []byte) error
}

var defaultFileHandlers = NewFileHandlers(config.Indexer{})

// FileHandlers prepares file contents using independently configured handlers.
type FileHandlers struct {
	pdf      *pdfFileType
	handlers []fileTypeHandler
}

// NewFileHandlers binds configuration to the handlers once, before use.
func NewFileHandlers(cfg config.Indexer) *FileHandlers {
	pdf := &pdfFileType{parser: cfg.PDFParser}
	pdf.parser.Args = slices.Clone(pdf.parser.Args)
	return &FileHandlers{
		pdf: pdf,
		handlers: []fileTypeHandler{
			pdf,
			docxFileType{},
			markdownFileType{},
			orgFileType{},
			htmlFileType{},
			plainTextFileType{},
		},
	}
}

// Prepare extracts indexable fields without reading or indexing the file.
func (h *FileHandlers) Prepare(ctx context.Context, path string, d *document.Document, content []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return h.forPath(path).Prepare(ctx, d, content)
}

// PrepareFileContent extracts indexable fields with the default handlers.
// It does not read, identify, or index the document.
func PrepareFileContent(path string, d *document.Document, content []byte) error {
	return defaultFileHandlers.Prepare(context.Background(), path, d, content)
}

func (h *FileHandlers) forPath(path string) fileTypeHandler {
	for _, handler := range h.handlers {
		if handler.Match(path) {
			return handler
		}
	}
	return plainTextFileType{}
}

type plainTextFileType struct{}

func (plainTextFileType) Match(_ string) bool {
	return true
}

func (plainTextFileType) Prepare(_ context.Context, d *document.Document, content []byte) error {
	if !utf8.Valid(content) {
		return ErrBinaryFile
	}

	d.Text = string(content)
	return nil
}

func hasExtension(path string, exts ...string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return slices.Contains(exts, ext)
}
