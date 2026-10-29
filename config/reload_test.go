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

func TestReloadWithoutMainConfiguration(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HISTER_DATA_DIR", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("LOCALAPPDATA", dir)
	c, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	// A file appearing after startup must not silently become authoritative.
	if err := os.WriteFile(filepath.Join(dir, "hister", "config.yml"), []byte("app:\n  title: Unexpected file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HISTER__APP__SUBTITLE", "Updated environment")
	next, err := c.Reload()
	if err != nil {
		t.Fatal(err)
	}
	if next.Filename() != "*Default Config*" || next.App.Title != "Hister" || next.App.Subtitle != "Updated environment" {
		t.Fatalf("reload did not retain defaults and environment without a selected file: %q, %+v", next.Filename(), next.App)
	}
}

func TestReloadRefreshesSourcesAndDetachesCandidates(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HISTER_DATA_DIR", filepath.Join(dir, "data"))
	filename := filepath.Join(dir, "config.yml")
	const original = "app:\n  title: Original\ncrawler:\n  headers:\n    x-test: original\n"
	if err := os.WriteFile(filename, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := Load(filename)
	if err != nil {
		t.Fatal(err)
	}
	if c.Source("app.title").Kind != "file" {
		t.Fatal("startup did not capture the selected file source")
	}
	const updated = "# app:\n#   title: Original\ncrawler:\n  headers:\n    x-test: updated\n"
	if err := os.WriteFile(filename, []byte(updated), 0o600); err != nil {
		t.Fatal(err)
	}
	next, err := c.Reload()
	if err != nil {
		t.Fatal(err)
	}
	if next.App.Title != "Hister" || next.Source("app.title").Locked || next.Source("crawler.headers").Kind != "file" {
		t.Fatal("reload retained a removed file selection")
	}
	next.Crawler.Headers["x-test"] = "candidate"
	snapshot := *c
	snapshot.SetFlagSource("app.title", "test-title")
	if c.Crawler.Headers["x-test"] != "original" || c.App.Title != "Original" || c.Source("app.title").Kind != "file" {
		t.Fatal("changing a candidate or snapshot mutated the active configuration")
	}
	t.Setenv("HISTER__APP__TITLE", "Environment")
	next, err = next.Reload()
	if err != nil || next.App.Title != "Environment" || next.Source("app.title").Kind != "environment" {
		t.Fatalf("reload did not capture updated environment sources: %v", err)
	}
	data, err := os.ReadFile(filename)
	if err != nil || string(data) != updated {
		t.Fatalf("reload rewrote the main file: %v", err)
	}
}

func TestReloadKeepsSelectedRelativeFile(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("HISTER_DATA_DIR", filepath.Join(dir, "data"))
	const content = "app:\n  title: Selected file\n"
	if err := os.WriteFile("config.yml", []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := Load("config.yml")
	if err != nil {
		t.Fatal(err)
	}
	if c.Filename() != filepath.Join(dir, "config.yml") {
		t.Fatalf("main file identity was resolved relative to data storage: %q", c.Filename())
	}
	t.Chdir(t.TempDir())
	next, err := c.Reload()
	if err != nil || next.App.Title != "Selected file" {
		t.Fatalf("reload lost the selected file after changing directory: %v", err)
	}
}
