package indexer

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/asciimoo/hister/config"
	"github.com/asciimoo/hister/files"
	"github.com/asciimoo/hister/server/document"
	"github.com/asciimoo/hister/server/model"
	"github.com/asciimoo/hister/server/testutil"

	"github.com/blevesearch/bleve/v2"
)

func TestDirectoryUserResolution(t *testing.T) {
	testutil.InitModel(t)

	u1 := testutil.CreateUser(t, "alice")
	u2 := testutil.CreateUser(t, "bob")

	tests := []struct {
		name     string
		username string
		wantID   uint
		wantErr  bool
	}{
		{
			name:     "empty username is global",
			username: "",
			wantID:   0,
			wantErr:  false,
		},
		{
			name:     "existing user alice",
			username: "alice",
			wantID:   u1.ID,
			wantErr:  false,
		},
		{
			name:     "existing user bob",
			username: "bob",
			wantID:   u2.ID,
			wantErr:  false,
		},
		{
			name:     "non-existent user",
			username: "charlie",
			wantID:   0,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotID uint
			var err error
			if tt.username != "" {
				u, e := model.GetUser(tt.username)
				if e != nil {
					err = e
				} else {
					gotID = u.ID
				}
			}
			if tt.wantErr {
				if err == nil {
					t.Errorf("user resolution(%q) expected error, got nil", tt.username)
				}
				return
			}
			if err != nil {
				t.Errorf("user resolution(%q) unexpected error: %v", tt.username, err)
				return
			}
			if gotID != tt.wantID {
				t.Errorf("user resolution(%q) = %d, want %d", tt.username, gotID, tt.wantID)
			}
		})
	}
}

func TestFileIndexQueueKeepsLatestPendingOperation(t *testing.T) {
	queue := NewFileIndexQueue()
	path := filepath.Join(t.TempDir(), "note.md")

	queue.EnqueueIndex(path, 42)
	queue.EnqueueDelete(path)

	item, ok := queue.pop()
	if !ok {
		t.Fatal("expected queued item")
	}
	if item.op != fileIndexDelete {
		t.Fatalf("queued operation = %v, want delete", item.op)
	}
	if item.path != path {
		t.Fatalf("queued path = %q, want %q", item.path, path)
	}
	if item.userID != 0 {
		t.Fatalf("queued user ID = %d, want 0", item.userID)
	}

	if _, ok := queue.pop(); ok {
		t.Fatal("expected queue to coalesce operations for the same path")
	}
}

func TestIndexFileWithUserID(t *testing.T) {
	testutil.InitModel(t)

	u := testutil.CreateUser(t, "testuser")

	testDir := t.TempDir()

	testFile := testutil.WriteFile(t, testDir, "test.txt", []byte("sample document content about indexing files for testing purposes"))
	testFile2 := testutil.WriteFile(t, testDir, "test2.txt", []byte("sample global document content for indexing test purposes"))

	idxCfg := testutil.Config(t)
	if err := Init(idxCfg); err != nil {
		t.Fatalf("failed to init indexer: %v", err)
	}
	defer i.Close()

	if err := IndexFile(testFile, u.ID); err != nil {
		t.Fatalf("IndexFile with user ID failed: %v", err)
	}

	if err := IndexFile(testFile2, 0); err != nil {
		t.Fatalf("IndexFile without user ID failed: %v", err)
	}
}

func TestAddDocumentIncrementsAddCount(t *testing.T) {
	idxCfg := testutil.Config(t)
	if err := Init(idxCfg); err != nil {
		t.Fatalf("failed to init indexer: %v", err)
	}
	defer i.Close()

	url := "https://example.com/count"
	for range 2 {
		if err := Add(&document.Document{
			URL:   url,
			Title: "Counted",
			Text:  "Counted document text",
		}); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
	}

	got := GetByURLAndUser(url, 0)
	if got == nil {
		t.Fatal("document not found")
	}
	if got.AddCount != 2 {
		t.Fatalf("AddCount = %d, want 2", got.AddCount)
	}

	latest := GetLatestDocuments(10, "", 0)
	if latest == nil {
		t.Fatal("latest documents not found")
	}
	if len(latest.Documents) != 1 {
		t.Fatalf("latest documents count = %d, want 1", len(latest.Documents))
	}
	if latest.Documents[0].AddCount != 2 {
		t.Fatalf("latest AddCount = %d, want 2", latest.Documents[0].AddCount)
	}
}

func TestAddDocumentTimestamps(t *testing.T) {
	idxCfg := testutil.Config(t)
	if err := Init(idxCfg); err != nil {
		t.Fatalf("failed to init indexer: %v", err)
	}
	defer i.Close()

	url := "https://example.com/timestamps"
	if err := Add(&document.Document{
		URL:   url,
		Title: "Initial document",
		Text:  "Initial document text",
	}); err != nil {
		t.Fatalf("initial Add failed: %v", err)
	}

	initial := GetByURLAndUser(url, 0)
	if initial == nil {
		t.Fatal("initial document not found")
	}
	if initial.Added == 0 {
		t.Fatal("Added was not populated")
	}
	if initial.Updated == 0 {
		t.Fatal("Updated was not populated")
	}

	if err := Add(&document.Document{
		URL:       url,
		Title:     "Updated document",
		Text:      "Updated document text",
		Added:     initial.Added + 10,
		Updated:   initial.Updated + 20,
		Processed: true,
	}); err != nil {
		t.Fatalf("second Add failed: %v", err)
	}

	updated := GetByURLAndUser(url, 0)
	if updated == nil {
		t.Fatal("updated document not found")
	}
	if updated.Added != initial.Added {
		t.Fatalf("Added = %d, want %d", updated.Added, initial.Added)
	}
	if updated.Updated != initial.Updated+20 {
		t.Fatalf("Updated = %d, want %d", updated.Updated, initial.Updated+20)
	}
}

func TestInitBackfillsLegacyUpdatedTimestamp(t *testing.T) {
	idxCfg := testutil.Config(t)
	if err := Init(idxCfg); err != nil {
		t.Fatalf("failed to init indexer: %v", err)
	}

	url := "https://example.com/legacy-updated"
	id := document.GetDocID(0, url)
	legacy := map[string]any{
		"url":            url,
		"title":          "Legacy document",
		"text":           "Legacy document text",
		"domain":         "example.com",
		"added":          int64(1234),
		"type":           int64(0),
		"user_id":        int64(0),
		"language":       "",
		"add_count":      int64(1),
		"metadata.topic": "compatibility",
	}
	idx := i.indexers[defaultIndexerName]
	if err := idx.Index(id, legacy); err != nil {
		t.Fatalf("failed to index legacy document: %v", err)
	}
	if err := idx.DeleteInternal([]byte(updatedBackfillKey)); err != nil {
		t.Fatalf("failed to clear backfill marker: %v", err)
	}
	i.Close()

	if err := Init(idxCfg); err != nil {
		t.Fatalf("failed to reopen indexer: %v", err)
	}
	defer i.Close()

	doc := GetByURLAndUser(url, 0)
	if doc == nil {
		t.Fatal("backfilled document not found")
	}
	if doc.Updated != doc.Added || doc.Updated != 1234 {
		t.Fatalf("timestamps are Added=%d Updated=%d, want both 1234", doc.Added, doc.Updated)
	}
	if doc.Metadata["topic"] != "compatibility" {
		t.Fatalf("metadata topic = %#v, want compatibility", doc.Metadata["topic"])
	}
	marker, err := i.indexers[defaultIndexerName].GetInternal([]byte(updatedBackfillKey))
	if err != nil {
		t.Fatalf("failed to read backfill marker: %v", err)
	}
	if len(marker) == 0 {
		t.Fatal("backfill marker was not stored")
	}
}

func TestUpdatedControlsDateSearch(t *testing.T) {
	idxCfg := testutil.Config(t)
	if err := Init(idxCfg); err != nil {
		t.Fatalf("failed to init indexer: %v", err)
	}
	defer i.Close()

	docs := []*document.Document{
		{
			URL:       "https://example.com/recently-added",
			Title:     "Recently added",
			Text:      "Recently added document text",
			Added:     200,
			Updated:   250,
			Processed: true,
		},
		{
			URL:       "https://example.com/recently-updated",
			Title:     "Recently updated",
			Text:      "Recently updated document text",
			Added:     100,
			Updated:   300,
			Processed: true,
		},
	}
	for _, doc := range docs {
		if err := Add(doc); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
	}

	res, err := Search(idxCfg, &Query{MatchAll: true, Sort: "date", DateFrom: 275})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(res.Documents) != 1 {
		t.Fatalf("document count = %d, want 1", len(res.Documents))
	}
	if res.Documents[0].URL != docs[1].URL {
		t.Fatalf("first document URL = %q, want %q", res.Documents[0].URL, docs[1].URL)
	}

	latest := GetLatestDocuments(10, "", 0)
	if latest == nil || len(latest.Documents) != 2 {
		t.Fatalf("latest documents = %#v, want two documents", latest)
	}
	if latest.Documents[0].URL != docs[1].URL {
		t.Fatalf("latest document URL = %q, want %q", latest.Documents[0].URL, docs[1].URL)
	}
}

func TestIndexFileUsesModificationTimeAsUpdated(t *testing.T) {
	testDir := t.TempDir()
	testFile := testutil.WriteFile(t, testDir, "timestamp.txt", []byte("document content with a source modification time"))
	modTime := time.Unix(1_700_000_000, 0)
	if err := os.Chtimes(testFile, modTime, modTime); err != nil {
		t.Fatalf("failed to set file modification time: %v", err)
	}

	idxCfg := testutil.Config(t)
	if err := Init(idxCfg); err != nil {
		t.Fatalf("failed to init indexer: %v", err)
	}
	defer i.Close()

	if err := IndexFile(testFile, 0); err != nil {
		t.Fatalf("IndexFile failed: %v", err)
	}
	doc := GetByURLAndUser(files.PathToFileURL(testFile), 0)
	if doc == nil {
		t.Fatal("indexed file not found")
	}
	if doc.Updated != modTime.Unix() {
		t.Fatalf("Updated = %d, want %d", doc.Updated, modTime.Unix())
	}
	if doc.Added == 0 || doc.Added == doc.Updated {
		t.Fatalf("Added = %d, want an indexing timestamp distinct from the file modification time", doc.Added)
	}
	if err := IndexFile(testFile, 0); err != nil {
		t.Fatalf("second IndexFile failed: %v", err)
	}
	doc = GetByURLAndUser(files.PathToFileURL(testFile), 0)
	if doc.AddCount != 1 {
		t.Fatalf("AddCount = %d, want unchanged file to be skipped", doc.AddCount)
	}
}

func TestAddDocumentDoesNotIncrementAddCountForExtraDocuments(t *testing.T) {
	idxCfg := testutil.Config(t)
	if err := Init(idxCfg); err != nil {
		t.Fatalf("failed to init indexer: %v", err)
	}
	defer i.Close()

	parentURL := "https://example.com/parent"
	extraURL := "https://example.com/extra"
	for range 2 {
		if err := Add(&document.Document{
			URL:   parentURL,
			Title: "Parent",
			Text:  "Parent document text",
			ExtraDocuments: []*document.Document{
				{
					URL:   extraURL,
					Title: "Extra",
					Text:  "Extra document text",
				},
			},
		}); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
	}

	parent := GetByURLAndUser(parentURL, 0)
	if parent == nil {
		t.Fatal("parent document not found")
	}
	if parent.AddCount != 2 {
		t.Fatalf("parent AddCount = %d, want 2", parent.AddCount)
	}

	extra := GetByURLAndUser(extraURL, 0)
	if extra == nil {
		t.Fatal("extra document not found")
	}
	if extra.AddCount != 1 {
		t.Fatalf("extra AddCount = %d, want 1", extra.AddCount)
	}
}

func TestMultiBatchAddsExtraDocuments(t *testing.T) {
	idxCfg := testutil.Config(t)
	if err := Init(idxCfg); err != nil {
		t.Fatalf("failed to init indexer: %v", err)
	}
	defer i.Close()

	parentURL := "https://example.com/batch-parent"
	extraURL := "https://example.com/batch-extra"
	batch := NewMultiBatch()
	if err := batch.Add(&document.Document{
		URL:   parentURL,
		Title: "Parent",
		Text:  "Parent document text",
		ExtraDocuments: []*document.Document{
			{
				URL:   extraURL,
				Title: "Extra",
				Text:  "Extra document text",
			},
		},
	}); err != nil {
		t.Fatalf("batch add failed: %v", err)
	}
	if err := batch.Save(); err != nil {
		t.Fatalf("batch save failed: %v", err)
	}

	if GetByURLAndUser(parentURL, 0) == nil {
		t.Fatal("parent document not found")
	}
	extra := GetByURLAndUser(extraURL, 0)
	if extra == nil {
		t.Fatal("extra document not found")
	}
	if extra.AddCount != 1 {
		t.Fatalf("extra AddCount = %d, want 1", extra.AddCount)
	}
}

func TestAddDocumentTreatsMissingAddCountAsOne(t *testing.T) {
	idxCfg := testutil.Config(t)
	if err := Init(idxCfg); err != nil {
		t.Fatalf("failed to init indexer: %v", err)
	}
	defer i.Close()

	url := "https://example.com/legacy-count"
	err := i.save(&document.Document{
		URL:   url,
		Title: "Legacy counted",
		Text:  "Legacy counted document text",
	})
	if err != nil {
		t.Fatalf("save failed: %v", err)
	}

	got := GetByURLAndUser(url, 0)
	if got == nil {
		t.Fatal("document not found")
	}
	if got.AddCount != 1 {
		t.Fatalf("legacy AddCount = %d, want 1", got.AddCount)
	}

	err = Add(&document.Document{
		URL:   url,
		Title: "Legacy counted",
		Text:  "Legacy counted document text",
	})
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	got = GetByURLAndUser(url, 0)
	if got == nil {
		t.Fatal("document not found after add")
	}
	if got.AddCount != 2 {
		t.Fatalf("AddCount after add = %d, want 2", got.AddCount)
	}
}

func TestAddDocumentReusesExistingDocumentLookup(t *testing.T) {
	idxCfg := testutil.Config(t)
	if err := Init(idxCfg); err != nil {
		t.Fatalf("failed to init indexer: %v", err)
	}
	defer i.Close()

	url := "https://example.com/reused-lookup"
	if err := i.save(&document.Document{
		URL:      url,
		Title:    "Existing document",
		Text:     "Existing document text",
		Label:    "preserved label",
		AddCount: 2,
	}); err != nil {
		t.Fatalf("initial save failed: %v", err)
	}

	searchesBefore := indexSearchCount(t, i.indexers[defaultIndexerName])
	if err := Add(&document.Document{
		URL:   url,
		Title: "Updated document",
		Text:  "Updated document text",
	}); err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	searches := indexSearchCount(t, i.indexers[defaultIndexerName]) - searchesBefore
	if searches != 1 {
		t.Fatalf("existing document searches = %d, want 1", searches)
	}

	got := GetByURLAndUser(url, 0)
	if got == nil {
		t.Fatal("document not found after add")
	}
	if got.AddCount != 3 {
		t.Fatalf("AddCount = %d, want 3", got.AddCount)
	}
	if got.Label != "preserved label" {
		t.Fatalf("Label = %q, want %q", got.Label, "preserved label")
	}
}

func TestSaveRemovesStaleLanguageCopy(t *testing.T) {
	idxCfg := testutil.Config(t)
	if err := Init(idxCfg); err != nil {
		t.Fatalf("failed to init indexer: %v", err)
	}
	defer i.Close()

	url := "https://example.com/language-copy"
	err := i.save(&document.Document{
		URL:      url,
		Title:    "Language copy",
		Text:     "Language copy text",
		Language: "en",
		AddCount: 4,
	})
	if err != nil {
		t.Fatalf("first save failed: %v", err)
	}
	if copies := countDocIDCopies(t, document.GetDocID(0, url)); copies != 1 {
		t.Fatalf("copies after first save = %d, want 1", copies)
	}

	staleIndex := i.indexers[indexNameForLanguage("en")]
	unrelated := i.getOrCreate("fr")
	staleDeletesBefore := indexDeleteCount(t, staleIndex)
	unrelatedDeletesBefore := indexDeleteCount(t, unrelated)

	err = i.save(&document.Document{
		URL:      url,
		Title:    "Language copy",
		Text:     "Language copy text",
		Language: "",
		AddCount: 5,
	})
	if err != nil {
		t.Fatalf("second save failed: %v", err)
	}
	if copies := countDocIDCopies(t, document.GetDocID(0, url)); copies != 1 {
		t.Fatalf("copies after language change = %d, want 1", copies)
	}

	got := GetByURLAndUser(url, 0)
	if got == nil {
		t.Fatal("document not found")
	}
	if got.AddCount != 5 {
		t.Fatalf("AddCount = %d, want 5", got.AddCount)
	}
	staleDeletes := indexDeleteCount(t, staleIndex) - staleDeletesBefore
	if staleDeletes != 1 {
		t.Fatalf("stale index delete count = %d, want 1", staleDeletes)
	}
	unrelatedDeletes := indexDeleteCount(t, unrelated) - unrelatedDeletesBefore
	if unrelatedDeletes != 0 {
		t.Fatalf("unrelated index delete count = %d, want 0", unrelatedDeletes)
	}
}

func indexDeleteCount(t *testing.T, idx bleve.Index) uint64 {
	t.Helper()
	stats := idx.StatsMap()
	indexStats, ok := stats["index"].(map[string]any)
	if !ok {
		t.Fatalf("index stats have type %T, want map[string]any", stats["index"])
	}
	deletes, ok := indexStats["deletes"].(uint64)
	if !ok {
		t.Fatalf("delete count has type %T, want uint64", indexStats["deletes"])
	}
	return deletes
}

func indexSearchCount(t *testing.T, idx bleve.Index) uint64 {
	t.Helper()
	stats := idx.StatsMap()
	searches, ok := stats["searches"].(uint64)
	if !ok {
		t.Fatalf("search count has type %T, want uint64", stats["searches"])
	}
	return searches
}

func countDocIDCopies(t *testing.T, id string) uint64 {
	t.Helper()
	q := bleve.NewDocIDQuery([]string{id})
	req := bleve.NewSearchRequest(q)
	req.Size = 10
	res, err := i.idx.Search(req)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	return res.Total
}

func TestDirectoryUserField(t *testing.T) {
	tests := []struct {
		name     string
		user     string
		expected string
	}{
		{
			name:     "empty user",
			user:     "",
			expected: "",
		},
		{
			name:     "user set",
			user:     "alice",
			expected: "alice",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := &config.Directory{
				Path: "/some/path",
				User: tt.user,
			}
			if dir.User != tt.expected {
				t.Errorf("Directory.User = %q, want %q", dir.User, tt.expected)
			}
		})
	}
}
