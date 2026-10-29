package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestReloadCommentedConfiguration(t *testing.T) {
	dir := t.TempDir()
	filename := filepath.Join(dir, "config.yml")
	t.Setenv("HISTER_DATA_DIR", filepath.Join(dir, "data"))
	const reference = "# app:\n#   title: Example title\n# server:\n#   address: 127.0.0.1:5544\n"
	write := func(content string) {
		t.Helper()
		if err := os.WriteFile(filename, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(reference)
	c, err := Load(filename)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ content, address string }{
		{reference, DefaultServerAddress},
		{"# app:\n#   title: Example title\nserver:\n  address: 127.0.0.1:5544\n", "127.0.0.1:5544"},
		{reference, DefaultServerAddress},
	} {
		write(tc.content)
		c, err = c.Reload()
		if err != nil {
			t.Fatal(err)
		}
		if c.App.Title != CreateDefaultConfig().App.Title || c.Server.Address != tc.address {
			t.Fatalf("title = %q, address = %q; reload must only apply uncommented values", c.App.Title, c.Server.Address)
		}
		content, err := os.ReadFile(filename)
		if err != nil || string(content) != tc.content {
			t.Fatalf("reload changed main configuration: %q, err=%v", content, err)
		}
	}
}

func TestReloadPreservesRuntimeStateAndAppliesOverrides(t *testing.T) {
	dir := t.TempDir()
	filename := filepath.Join(dir, "config.yml")
	write := func(title string) {
		t.Helper()
		content := fmt.Sprintf("app:\n  directory: %q\n  title: %s\n", dir, title)
		if err := os.WriteFile(filename, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("Original")
	c, err := Load(filename)
	if err != nil {
		t.Fatal(err)
	}
	write("Reloaded")
	t.Setenv("HISTER__APP__SUBTITLE", "From server environment")
	if err := os.WriteFile(filepath.Join(dir, "tui.yaml"), []byte("dark_theme: changed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, secretKeyFilename), []byte("replacement"), 0o600); err != nil {
		t.Fatal(err)
	}
	next, err := c.Reload(func(next *Config) error {
		next.App.SearchURL = "https://startup.example/?q={query}"
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if next.App.Title != "Reloaded" || next.App.Subtitle != "From server environment" || next.App.SearchURL != "https://startup.example/?q={query}" {
		t.Fatalf("incorrect reload: %+v", next.App)
	}
	if c.App.Title != "Original" || next.TUI != c.TUI || !bytes.Equal(next.SecretKey(), c.SecretKey()) {
		t.Fatal("reload mutated original configuration or replaced startup runtime state")
	}
}

func TestReloadRequiresOriginalFile(t *testing.T) {
	dir := t.TempDir()
	c := CreateDefaultConfig()
	c.fname = filepath.Join(dir, "missing.yml")
	c.App.Directory = dir
	if _, err := c.Reload(); err == nil {
		t.Fatal("reload silently fell back from the original config file")
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("failed reload created files: %v, %v", entries, err)
	}
}
