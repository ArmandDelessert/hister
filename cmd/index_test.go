package cmd

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/spf13/cobra"
)

func newIndexTestCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "index [URL...]"}
	cmd.Flags().String("job-id", "", "")
	cmd.Flags().String("url-list", "", "")
	return cmd
}

func TestValidateIndexArgs(t *testing.T) {
	tests := []struct {
		name    string
		jobID   string
		urlList string
		args    []string
		wantErr bool
	}{
		{name: "URL", args: []string{"https://example.com"}},
		{name: "job ID without URL", jobID: "docs-crawl"},
		{name: "URL list without URL", urlList: "urls.txt"},
		{name: "neither job ID, URL list, nor URL", wantErr: true},
		{name: "job ID and URL list", jobID: "docs-crawl", urlList: "urls.txt", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newIndexTestCommand()
			if err := cmd.Flags().Set("job-id", tt.jobID); err != nil {
				t.Fatalf("set job-id flag: %v", err)
			}
			if err := cmd.Flags().Set("url-list", tt.urlList); err != nil {
				t.Fatalf("set url-list flag: %v", err)
			}

			err := validateIndexArgs(cmd, tt.args)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateIndexArgs() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestResolveIndexURLsPrefersURLList(t *testing.T) {
	path := filepath.Join(t.TempDir(), "urls.txt")
	if err := os.WriteFile(path, []byte(" https://example.com \r\n\nhttps://example.org\n"), 0o600); err != nil {
		t.Fatalf("write URL list: %v", err)
	}
	cmd := newIndexTestCommand()
	if err := cmd.Flags().Set("url-list", path); err != nil {
		t.Fatalf("set url-list flag: %v", err)
	}
	want := []string{"https://example.com", "https://example.org"}

	got, err := resolveIndexURLs(cmd, []string{"https://ignored.example"})
	if err != nil {
		t.Fatalf("resolveIndexURLs() error: %v", err)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("resolveIndexURLs() = %q, want %q", got, want)
	}
}

func TestResolveIndexURLsRejectsEmptyList(t *testing.T) {
	path := filepath.Join(t.TempDir(), "urls.txt")
	if err := os.WriteFile(path, []byte("\n \t\n"), 0o600); err != nil {
		t.Fatalf("write URL list: %v", err)
	}
	cmd := newIndexTestCommand()
	if err := cmd.Flags().Set("url-list", path); err != nil {
		t.Fatalf("set url-list flag: %v", err)
	}

	if _, err := resolveIndexURLs(cmd, nil); err == nil {
		t.Fatal("resolveIndexURLs() expected an error for an empty URL list")
	}
}
