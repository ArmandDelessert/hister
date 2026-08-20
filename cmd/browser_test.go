package cmd

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestPrepareBrowserImportsContinuesAfterUnusableDatabase(t *testing.T) {
	dir := t.TempDir()
	invalidPath := filepath.Join(dir, "Chrome", "Default", "History")
	emptyPath := filepath.Join(dir, "Brave", "Default", "History")
	validPath := filepath.Join(dir, "Vivaldi", "Default", "History")

	writeBrowserHistoryFile(t, invalidPath, false, nil)
	writeBrowserHistoryFile(t, emptyPath, true, nil)
	writeBrowserHistoryFile(t, validPath, true, []string{"https://example.com"})

	choices, issues := prepareBrowserImports([]DBToImport{
		{table: "urls", databaseFile: invalidPath},
		{table: "urls", databaseFile: emptyPath},
		{table: "urls", databaseFile: validPath},
	}, 0, nil, nil)
	if len(choices) != 1 {
		t.Fatalf("prepareBrowserImports() returned %d choices, want 1", len(choices))
	}
	defer func() {
		if err := choices[0].db.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	if choices[0].choice != validPath {
		t.Fatalf("prepareBrowserImports() choice = %q, want %q", choices[0].choice, validPath)
	}
	if choices[0].urls != 1 {
		t.Fatalf("prepareBrowserImports() URL count = %d, want 1", choices[0].urls)
	}

	if len(issues) != 2 {
		t.Fatalf("prepareBrowserImports() returned %d issues, want 2", len(issues))
	}
	if issues[0].databaseFile != invalidPath {
		t.Fatalf("first issue file = %q, want %q", issues[0].databaseFile, invalidPath)
	}
	if issues[1].databaseFile != emptyPath {
		t.Fatalf("second issue file = %q, want %q", issues[1].databaseFile, emptyPath)
	}
	if !errors.Is(issues[1].err, errNoBrowserURLs) {
		t.Fatalf("second issue = %v, want %v", issues[1].err, errNoBrowserURLs)
	}
}

func writeBrowserHistoryFile(t *testing.T, path string, createTable bool, urls []string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	if !createTable {
		return
	}
	if _, err := db.Exec("CREATE TABLE urls (url TEXT, visit_count INTEGER)"); err != nil {
		t.Fatal(err)
	}
	for _, u := range urls {
		if _, err := db.Exec("INSERT INTO urls (url, visit_count) VALUES (?, 1)", u); err != nil {
			t.Fatal(err)
		}
	}
}
