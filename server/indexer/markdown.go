// SPDX-License-Identifier: AGPL-3.0-or-later

package indexer

import (
	"errors"
	"strings"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"

	"github.com/asciimoo/hister/server/document"
)

// AddMarkdown renders mdData to HTML, stores it in d.HTML, and stores the raw
// source in d.Text for full-text indexing.
func AddMarkdown(d *document.Document, mdData []byte) error {
	src := strings.TrimSpace(string(mdData))
	if src == "" {
		return errors.New("markdown file is empty")
	}
	d.HTML = renderMarkdown(mdData)
	d.Text = src
	d.AddMetadata("type", "markdown")
	return Add(d)
}

func renderMarkdown(src []byte) string {
	p := parser.NewWithExtensions(
		parser.CommonExtensions | parser.AutoHeadingIDs | parser.NoEmptyLineBeforeBlock,
	)
	opts := html.RendererOptions{Flags: html.CommonFlags | html.HrefTargetBlank}
	r := html.NewRenderer(opts)
	return string(markdown.ToHTML(src, p, r))
}
