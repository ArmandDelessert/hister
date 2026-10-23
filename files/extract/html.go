// SPDX-License-Identifier: AGPL-3.0-or-later

package extract

import (
	"context"
	"unicode/utf8"
)

type htmlFileType struct{}

func (htmlFileType) Match(path string) bool {
	return hasExtension(path, ".html", ".htm")
}

func (htmlFileType) Prepare(_ context.Context, d *Result, content []byte) error {
	if !utf8.Valid(content) {
		return ErrBinaryFile
	}
	d.HTML = string(content)
	d.Format = HTML
	return nil
}
