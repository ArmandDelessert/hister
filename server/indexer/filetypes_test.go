package indexer

import "testing"

func TestFileTypeHandlerForPath(t *testing.T) {
	tests := []struct {
		path string
		want any
	}{
		{path: "paper.pdf", want: pdfFileType{}},
		{path: "paper.PDF", want: pdfFileType{}},
		{path: "paper.docx", want: docxFileType{}},
		{path: "paper.DOCX", want: docxFileType{}},
		{path: "notes.md", want: markdownFileType{}},
		{path: "notes.markdown", want: markdownFileType{}},
		{path: "notes.org", want: orgFileType{}},
		{path: "notes.txt", want: plainTextFileType{}},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := fileTypeHandlerForPath(tt.path)
			if got != tt.want {
				t.Fatalf("handler = %T, want %T", got, tt.want)
			}
		})
	}
}
